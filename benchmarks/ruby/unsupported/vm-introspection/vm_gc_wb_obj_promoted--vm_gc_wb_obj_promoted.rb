class C
  attr_accessor :foo
end
long_lived = C.new

if RUBY_VERSION >= "2.2.0"
  3.times{ GC.start(full_mark: false, immediate_mark: true, immediate_sweep: true) }
elsif
  GC.start
end

short_lived = ''

long_lived.foo = short_lived # write barrier

