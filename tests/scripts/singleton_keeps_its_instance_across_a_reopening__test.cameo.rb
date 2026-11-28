# singleton: testcase to demonstrate that singleton keeps its instance across
# a reopening
#
# The instance survives a reopening of its class, with the class's new
# methods, and initialize ran once; the instance from before the reopening is
# a value of the class as it was, so the two are not ==, as no value made
# before a reopening is. A module cannot include Singleton, a class cannot
# extend it, and a class without a name is named as Ruby names it.
#
# The expected strings are Ruby's, with singleton 0.3.0.
#
# (Introduced in PR #282. expected output follows the __END__ separator.)
require "singleton"
p Singleton::VERSION
class Config
  include Singleton
  def initialize
    puts "init"
  end
end
a = Config.instance
class Config
  def extra
    2
  end
end
b = Config.instance
p a == b, b.extra, Config.instance.extra
class Sub < Config
end
p Sub.instance == Config.instance, Sub.instance.class
begin
  module M
    include Singleton
  end
rescue TypeError => e
  puts e.message
end
begin
  class E
    extend Singleton
  end
rescue NoMethodError => e
  puts e.message
end
k = Class.new { include Singleton }
begin; k.new; rescue NoMethodError => e; puts e.message; end

__END__
"0.3.0"
init
false
2
2
init
false
Sub
Inclusion of the OO-Singleton module in module M
undefined method 'extend_object' for module Singleton
private method 'new' called for class #<Class>
