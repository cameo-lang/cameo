# frozen_string_literal: true

# Cameo::Runner gives a Ruby program cameo's async, await and promises, so
# that a script written for cameo runs on CRuby as well. A class includes
# it to run its work concurrently:
#
#   require "cameo/runner"
#
#   class Report
#     include Cameo::Runner
#
#     def line(name) = "#{name}: ..."
#     def build(names) = await(names.map { |name| async self, :line, name })
#   end
#
# cameo has the library built in, so the same file runs on both; on both,
# the top level refuses the include. Here async copies the receiver and the
# arguments with Marshal, as cameo copies them, and runs the call on a
# thread of its own. CRuby's global VM lock runs one thread's Ruby code at
# a time, so calls that compute take turns on one core, where cameo spreads
# them over the cores; a call that waits for I/O lets the others run on
# both. Results, failures, promise ids and error messages are cameo's. See
# https://cameo-lang.github.io/async.html.
module Cameo
  module Runner
    # Promise is one result to come, which the script holds by its id.
    class Promise
      attr_reader :id, :method_name

      def initialize(id, method_name)
        @id = id
        @method_name = method_name
        @mutex = Mutex.new
        @settled_cond = ConditionVariable.new
        @settled = false
        @awaited = false
      end

      # resolve sets the result once; false when the promise was settled
      # already. fail sets the error the same way.
      def resolve(value)
        settle(value, nil)
      end

      def fail(error)
        settle(nil, error)
      end

      # wait blocks until the promise is settled and returns its value and
      # its error, one of them nil.
      def wait
        @mutex.synchronize do
          @settled_cond.wait(@mutex) until @settled
          @awaited = true
          [@value, @error]
        end
      end

      def awaited?
        @mutex.synchronize { @awaited }
      end

      private

      def settle(value, error)
        @mutex.synchronize do
          return false if @settled

          @settled = true
          @value = value
          @error = error
          @settled_cond.broadcast
        end
        true
      end
    end

    # The promises of the program, shared by every thread, and the ones each
    # method started, which await :all collects.
    @mutex = Mutex.new
    @next = 0
    @promises = {}
    @started = []
    @scopes = Hash.new { |h, k| h[k] = [] }

    class << self
      def add(method_name, scope: nil)
        @mutex.synchronize do
          @next += 1
          promise = Promise.new(@next, method_name)
          @promises[promise.id] = promise
          if scope
            @started << promise
            @scopes[scope] << promise
          end
          promise
        end
      end

      def find(id)
        @mutex.synchronize { @promises[id] }
      end

      def started_in(scope)
        @mutex.synchronize { @scopes[scope].dup }
      end

      def started
        @mutex.synchronize { @started.dup }
      end

      # copy is a copy of value that shares nothing with it, as a value that
      # crosses to a cameo worker is; a Proc, a Method, a class or a module
      # stays where it was made.
      def copy(value, what)
        case value
        when Proc, Method, Module
          raise TypeError, format(what, kind(value))
        end
        Marshal.load(Marshal.dump(value))
      rescue TypeError => e
        raise if e.message.start_with?("a ")

        raise TypeError, format(what, kind(value))
      end

      def kind(value)
        case value
        when Proc, Method then "Proc"
        when Class then "Class"
        when Module then "Module"
        else value.class.name
        end
      end

      # scope names the method a call was written in, blocks included, as
      # cameo scopes await :all to the method that started the calls.
      def scope(location)
        method = location.label.sub(/\Ablock (\(\d+ levels\) )?in /, "")
        [Thread.current, location.path, method]
      end

      # report waits for every call async started and writes the failures
      # nobody awaited to standard error, as cameo does when a run ends.
      def report
        started.each do |promise|
          seen = promise.awaited?
          _, error = promise.wait
          next unless error && !seen

          warn "cameo: promise #{promise.id} (#{promise.method_name}) failed and was never awaited: " \
               "#{error.message} (#{error.class})"
        end
      end
    end

    at_exit { Runner.report }

    # A class includes the module; the top level, which would bring it to
    # every object, refuses it as cameo does.
    def self.included(base)
      return unless base == Object

      raise ArgumentError, "include Cameo::Runner belongs in the class that runs the work; at the top level include Cameo"
    end

    # async starts receiver.method(*args, **kwargs) on a thread of its own,
    # over copies of the receiver and the arguments, and returns its
    # promise, an Integer.
    def async(receiver, method, *args, **kwargs)
      unless method.is_a?(Symbol) || method.is_a?(String)
        raise TypeError, "async names the method by a symbol, not an instance of #{method.class}"
      end
      unless receiver.respond_to?(method)
        raise NoMethodError, "undefined method '#{method}' for an instance of #{receiver.class}"
      end

      what = "a %s cannot be passed to async"
      receiver = Runner.copy(receiver, what)
      args = args.map { |a| Runner.copy(a, what) }
      kwargs = kwargs.transform_values { |v| Runner.copy(v, what) }
      promise = Runner.add(method.to_s, scope: Runner.scope(caller_locations(1, 1).first))
      thread = Thread.new do
        value = receiver.public_send(method, *args, **kwargs)
        promise.resolve(Runner.copy(value, "a worker returned a %s, which cannot be passed back"))
      rescue Exception => e # rubocop:disable Lint/RescueException
        promise.fail(e)
      end
      thread.report_on_exception = false
      promise.id
    end

    # await waits for a promise and returns its result, or raises what the
    # call raised. It takes a promise, a list of them (their results in
    # order, raising the first failure once all are settled), :all (every
    # call this method started and has not awaited) or a call, which it
    # starts and waits for.
    def await(*args, **kwargs)
      if args.size > 1
        return await(async(*args, **kwargs))
      end
      raise ArgumentError, "await takes a promise, a list of promises, :all, or a receiver with a method name and arguments" if args.empty?

      target = args.first
      case target
      when :all
        pending = Runner.started_in(Runner.scope(caller_locations(1, 1).first)).reject(&:awaited?)
        await_each(pending)
      when Array
        await_each(target.map { |id| promise_of(id) })
      else
        value, error = promise_of(target).wait
        raise error if error

        value
      end
    end

    # promise makes a pending promise, which resolve or fail settles.
    def promise
      Runner.add("").id
    end

    # resolve settles a promise with a copy of value.
    def resolve(id, value)
      promise = promise_of(id)
      value = Runner.copy(value, "resolve got a %s, which cannot be passed back")
      raise ArgumentError, "promise #{id} is settled already" unless promise.resolve(value)

      id
    end

    # fail settles a promise with the exception raise would raise with the
    # rest of the arguments. With anything but a promise first it raises, as
    # Kernel#fail does.
    def fail(*args)
      return super unless args.size >= 2 && args.first.is_a?(Integer)

      id, *exception = args
      promise = promise_of(id)
      error = begin
        Kernel.raise(*exception)
      rescue Exception => e # rubocop:disable Lint/RescueException
        e
      end
      raise ArgumentError, "promise #{id} is settled already" unless promise.fail(error)

      id
    end

    private

    def promise_of(id)
      unless id.is_a?(Integer)
        raise TypeError, "a promise is the Integer async or promise returned, not an instance of #{id.class}"
      end

      Runner.find(id) || raise(ArgumentError, "no promise #{id}")
    end

    def await_each(promises)
      results = []
      first = nil
      promises.each do |promise|
        value, error = promise.wait
        if error
          first ||= error
        else
          results << value
        end
      end
      raise first if first

      results
    end
  end

  extend Runner
end
