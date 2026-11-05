package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected strings are Ruby's, with singleton 0.2.0.

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
"0.2.0"
NoMethodError: private method 'new' called for class Config
NoMethodError: private method 'allocate' called for class Config
NoMethodError: private method 'new' called for class Sub
TypeError: can't dup instance of singleton Config
TypeError: can't clone instance of singleton Config
`)
}
