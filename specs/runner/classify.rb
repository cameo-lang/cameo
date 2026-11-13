# frozen_string_literal: true

# Classify sorts what stopped a spec, a checker message or an example's
# failure, into one of three statuses:
#
#   expected  cameo leaves the construct or library out on purpose, as
#             doc/cameo-vs-ruby.md says: mutation, instance variables,
#             object identity, threads, I/O and so on
#   harness   the example needs a part of mspec the prelude does not have
#   bug       anything else: a wrong result, or a method or class cameo
#             should have
#
# The first rule whose pattern matches the message decides; a message no
# rule matches is a bug, so that it is looked at.
module Classify
  # The classes and modules cameo leaves out, by the reason it does.
  LEFT_OUT = {
    "threads" => %w[Thread ThreadGroup Mutex Queue SizedQueue ConditionVariable Fiber Ractor Monitor],
    "io" => %w[IO File Dir ARGF STDIN STDOUT STDERR FileTest Errno Socket],
    "process" => %w[Process Signal ENV GC ObjectSpace RubyVM TracePoint Binding Warning],
    "rational-complex" => %w[Rational Complex],
    "encodings" => %w[Encoding::Converter Encoding::Compatibility],
    "metaprogramming" => %w[Module Method UnboundMethod],
    "struct" => %w[Struct],
    "data" => %w[Data],
    "marshal" => %w[Marshal],
  }.freeze

  HARNESS = %w[mock mock_numeric mock_int mock_to_path fixture ruby_exe with_timezone should_receive
               should_not_receive stub! ScratchPad MockObject MockIntObject block_caller complain output
               output_to_fd tmp rm_r mkdir_p touch new_fd new_io cp].freeze

  # Messages for constructs cameo refuses; its checker words them.
  RULES = [
    [/\Aparse error/, "bug", "parse-error"],
    [/cameo has no instance variables|instance_variable_[gs]et|instance_variables/, "expected", "instance-variables"],
    [/\Aglobal variables are not supported|\$[\w~!]/, "expected", "globals"],
    [/class variables are not supported|class_variable/, "expected", "class-variables"],
    [/This is not supported, use \S+ = \S+ instead|returns a copy in cameo and changes nothing|changes its receiver|cannot store into|the left side of an assignment must start|index assignment takes one index|cameo has no setters|a field is set in initialize|setters are not supported|multiple assignment binds local variables only/, "expected", "mutation"],
    [/\A(public|private|protected|private_constant|module_function|private_class_method|public_class_method)\b.* is not supported|every method is public/, "expected", "visibility"],
    [/\Aattr(_reader|_writer|_accessor)? /, "expected", "attr-accessor"],
    [/\b(eval|instance_eval|class_eval|module_eval|instance_exec|class_exec|define_method|define_singleton_method|remove_method|undef_method|alias_method|instance_method|method_added|singleton_class|const_set|remove_const)\b.*(not supported|undefined method)|class << self|on an object: methods belong to classes|allocate is not supported|undef is not supported|\bundefined method '(instance_method|method|singleton_methods|define_method|public_method)'/, "expected", "metaprogramming"],
    [/undefined method '(equal\?|object_id|__id__|frozen\?)'|FrozenError|can't modify frozen/, "expected", "identity-and-freezing"],
    [/a subclass of Numeric is not supported/, "expected", "numeric-subclass"],
    [/a subclass of BasicObject is not supported/, "expected", "basic-object-subclass"],
    [/cannot load such file|\A(load|require_relative) is not supported/, "expected", "require"],
    [/backtick commands|undefined method '(system|spawn|fork|exec|trap|at_exit|exit!|sleep|`)'/, "expected", "process"],
    [/scoped constant/, "expected", "scoped-constants"],
    [/undefined method '(const_set|initialize_copy|initialize_clone|initialize_dup)'/, "expected", "metaprogramming"],
    [/undefined method '(to_r|rationalize)'/, "expected", "rational-complex"],
    [/undefined method '(list|aliases|name_list|find|default_external|default_internal|compatible\?)' for class Encoding|uninitialized constant Encoding::(?!UTF_8\b|BINARY\b|ASCII_8BIT\b|US_ASCII\b)|fixed_encoding\?|FIXEDENCODING|force_encoding|unknown encoding name/, "expected", "encodings"],
    [/undefined method 'last_match' for class Regexp/, "expected", "globals"],
    # cameo's own refusal, which names the alternative
    [/: use a literal \(NoMethodError\)/, "expected", "literal-only"],
    [/This regexp is not supported in Cameo/, "expected", "regexp"],
    [/ruby2_keywords/, "expected", "ruby2_keywords"],
    # a mutating call's refusal names the rebinding to use instead
    [/mutating calls without rebinding/, "expected", "mutation"],
    # a block captures the values of its variables (doc/cameo-vs-ruby.md)
    [/a block cannot rebind|captures the values its variables have/, "expected", "block-captures-values"],
    # Marshal reads cameo's wire form, not Ruby's
    [/incompatible marshal data/, "expected", "marshal"],
    # every method is public, so the lists by visibility do not exist
    [/undefined method '(private|public|protected)_(instance_)?methods'/, "expected", "visibility"],
    [/(imaginary|rational) literals are not supported|undefined method '(to_c|i)' for/, "expected", "rational-complex"],
    [/fork is not supported/, "expected", "process"],
    # bytes that are not UTF-8 live in a binary string, written b"..."
    [/is a binary string and is written b"|keep the bytes in a binary string/, "expected", "binary-strings"],
    # initialize runs in new alone; send cannot reach it
    [/send\(:initialize\): initialize runs only in new/, "expected", "metaprogramming"],
  ].freeze

  # Examples whose name says they test something cameo leaves out, where
  # the message does not.
  NAME_RULES = [
    [/'[es]' (modifier|option)|EUC-JP|Windows-31J|Shift_JIS|UTF-16|UTF-32|ISO-8859/, "expected", "encodings"],
  ].freeze

  # Examples of something cameo leaves out whose failure message says
  # nothing: a comparison of Marshal.dump's bytes with Ruby's, an
  # exception ObjectSpace raised. Checked last, after the harness and the
  # message rules, so that they hide no other cause.
  LEFT_OUT_NAMES = [
    [/\AMarshal\.(dump|load) /, "expected", "marshal"],
    [/\A(ObjectSpace|GC)\./, "expected", "process"],
  ].freeze

  # status returns [status, reason] for a message that stopped a spec, the
  # example's name, if there is one, deciding first.
  def self.status(message, name = "")
    NAME_RULES.each { |pattern, status, reason| return [status, reason] if name.match?(pattern) }
    if (called = message[/undefined (?:local variable or )?method '([^']+)' for/, 1]) && HARNESS.include?(called)
      return ["harness", called]
    end
    if (called = message[/uninitialized constant (\w+)/, 1]) && HARNESS.include?(called)
      return ["harness", called]
    end
    RULES.each { |pattern, status, reason| return [status, reason] if message.match?(pattern) }
    LEFT_OUT.each do |reason, names|
      names.each do |n|
        return ["expected", reason] if message.match?(/uninitialized constant (\w+::)*#{Regexp.escape(n)}\b|for (class|module) #{Regexp.escape(n)}\b|'#{Regexp.escape(n)}' for|undefined method '#{Regexp.escape(n)}'/)
      end
    end
    LEFT_OUT_NAMES.each { |pattern, status, reason| return [status, reason] if name.match?(pattern) }
    ["bug", ""]
  end
end
