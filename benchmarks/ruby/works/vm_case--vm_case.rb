
#--- benchmark: the prelude above runs once, the script below in the loop ---
case :foo
when :bar
  raise
when :baz
  raise
when :boo
  raise
when :foo
  # noop
end

