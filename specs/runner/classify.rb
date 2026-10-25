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
    [/This is not supported, use \S+ = \S+ instead|returns a copy in cameo and changes nothing|changes its receiver|cannot store into|the left side of an assignment must start|index assignment takes one index|cameo has no setters|a field is set in initialize only|setters are not supported|multiple assignment binds local variables only/, "expected", "mutation"],
    [/\A(public|private|protected|private_constant|module_function|private_class_method|public_class_method)\b.* is not supported|every method is public/, "expected", "visibility"],
    [/\Aattr_(reader|writer|accessor)/, "expected", "attr-accessor"],
    [/\b(eval|instance_eval|class_eval|module_eval|instance_exec|class_exec|define_method|define_singleton_method|remove_method|undef_method|alias_method|instance_method|method_added|singleton_class|const_set|remove_const)\b.*(not supported|undefined method)|class << self|on an object: methods belong to classes|allocate is not supported|undef is not supported|\bundefined method '(instance_method|method|singleton_methods|define_method|public_method)'/, "expected", "metaprogramming"],
    [/undefined method '(equal\?|object_id|__id__|frozen\?)'|FrozenError|can't modify frozen/, "expected", "identity-and-freezing"],
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
  ].freeze

  # Examples whose name says they test something cameo leaves out, where
  # the message does not.
  NAME_RULES = [
    [/'[es]' (modifier|option)|EUC-JP|Windows-31J|Shift_JIS|UTF-16|UTF-32|ISO-8859/, "expected", "encodings"],
  ].freeze

  # status returns [status, reason] for a message that stopped a spec, the
  # example's name, if there is one, deciding first.
  def self.status(message, name = "")
    NAME_RULES.each { |pattern, status, reason| return [status, reason] if name.match?(pattern) }
    if (name = message[/undefined (?:local variable or )?method '([^']+)' for/, 1]) && HARNESS.include?(name)
      return ["harness", name]
    end
    if (name = message[/uninitialized constant (\w+)/, 1]) && HARNESS.include?(name)
      return ["harness", name]
    end
    RULES.each { |pattern, status, reason| return [status, reason] if message.match?(pattern) }
    LEFT_OUT.each do |reason, names|
      names.each do |n|
        return ["expected", reason] if message.match?(/uninitialized constant (\w+::)*#{Regexp.escape(n)}\b|for (class|module) #{Regexp.escape(n)}\b|'#{Regexp.escape(n)}' for|undefined method '#{Regexp.escape(n)}'/)
      end
    end
    ["bug", ""]
  end
end
