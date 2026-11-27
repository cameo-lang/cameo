# singleton: testcase to demonstrate that singleton gives one instance and
# refuses new
#
# The expected strings are Ruby's, with singleton 0.3.0.
#
# (Introduced in PR #282. expected output follows the __END__ separator.)
require "singleton"
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

__END__
7
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
