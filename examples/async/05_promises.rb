# A promise made by hand: promise makes one, and resolve or fail settles it,
# once. A promise is an Integer, so it travels to a worker like any other
# value, and the worker settles it while the caller awaits it.
require "cameo/runner"

class Courier
  include Cameo::Runner

  def deliver(promise, parcel)
    resolve(promise, "#{parcel}, delivered")
  end

  def lose(promise, parcel)
    fail(promise, KeyError, "#{parcel} is lost")
  end

  def demo
    parcel = promise
    async self, :deliver, parcel, "books"
    puts await(parcel)

    missing = promise
    async self, :lose, missing, "a lamp"
    begin
      await missing
    rescue KeyError => e
      puts "failed: #{e.message}"
    end

    begin
      resolve parcel, "again"
    rescue ArgumentError => e
      puts e.message
    end

    # fail with anything but a promise first raises, as Ruby's fail does.
    begin
      fail "not a promise"
    rescue RuntimeError => e
      puts "raised: #{e.message}"
    end
  end
end

Courier.new.demo
