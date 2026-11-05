package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected strings are Ruby's, with singleton 0.3.0.

func TestSingletonGivesOneInstanceAndRefusesNew(t *testing.T) {
	out := runScript(t, `require "singleton"
class Config
  include Singleton
  int :count
  def initialize
    self.count = 7
  end
end
class Sub < Config
end
p Config.instance.count, Config.instance == Config.instance, Config.instance.class, Sub.instance.class, Sub.instance == Config.instance, Config.include?(Singleton), Config.instance.is_a?(Singleton), Config.respond_to?(:instance), Singleton::VERSION
[-> { Config.new }, -> { Config.allocate }, -> { Sub.new }, -> { Config.instance.dup }, -> { Config.instance.clone }].each do |f|
  f.call
rescue NoMethodError, TypeError => e
  puts "#{e.class}: #{e.message}"
end
`)
	assert.Equals(t, out, `7
true
Config
Sub
false
true
true
true
"0.3.0"
NoMethodError: private method 'new' called for class Config
NoMethodError: private method 'allocate' called for class Config
NoMethodError: private method 'new' called for class Sub
TypeError: can't dup instance of singleton Config
TypeError: can't clone instance of singleton Config
`)
}

// The instance survives a reopening of its class, with the class's new
// methods, and initialize ran once; the instance from before the
// reopening is a value of the class as it was, so the two are not ==, as
// no value made before a reopening is. A module cannot include Singleton,
// a class cannot extend it, and a class without a name is named as Ruby
// names it.
func TestSingletonKeepsItsInstanceAcrossAReopening(t *testing.T) {
	out := runScript(t, `require "singleton"
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
`)
	assert.Equals(t, out, `"0.3.0"
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
`)
}
