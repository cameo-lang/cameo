symbol = RubyVM::InstructionSequence.compile(":foo; :bar; :baz; :egg; :spam").to_binary

define_method = RubyVM::InstructionSequence.compile(%{
  def foo; end
  def bar; end
  def baz; end
  def egg; end
  def spam; end
}).to_binary

all = RubyVM::InstructionSequence.compile(%{
  module Foo; def foo; :foo; end; end
  module Bar; def bar; :bar; end; end
  module Baz; def baz; :baz; end; end
  class Egg; def egg; :egg; end; end
  class Spaml; def spam; :spam; end; end
}).to_binary

RubyVM::InstructionSequence.load_from_binary(define_method)
