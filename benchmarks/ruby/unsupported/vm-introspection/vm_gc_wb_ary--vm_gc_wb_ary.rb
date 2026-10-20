short_lived_ary = []

if RUBY_VERSION >= "2.2.0"
  GC.start(full_mark: false, immediate_mark: true, immediate_sweep: true)
end

short_lived = ''

short_lived_ary[0] = short_lived # write barrier

