class C
  attr_accessor :foo
end
short_lived_obj = C.new

if RUBY_VERSION >= "2.2.0"
  GC.start(full_mark: false, immediate_mark: true, immediate_sweep: true)
end

short_lived = ''

short_lived_obj.foo = short_lived # write barrier

