# frozen_string_literal: true
hash = 10.times.to_h do |i|
  [i, i]
end
dyn_sym = "dynamic_symbol".to_sym
binary = RubyVM::InstructionSequence.compile("# frozen_string_literal: true\n'iseq_load'").to_binary
iseq_literal_string = RubyVM::InstructionSequence.load_from_binary(binary).eval

hash[:some_symbol] = 1
hash[dyn_sym] = 2
hash["small"] = 3
hash["frozen_string_literal"] = 4
hash[iseq_literal_string] = 5

hash[dyn_sym]
