long_lived = []

if RUBY_VERSION > "2.2.0"
  3.times{ GC.start(full_mark: false, immediate_mark: true, immediate_sweep: true) }
elsif
  GC.start
end

short_lived = ''

long_lived[0] = short_lived # write barrier

