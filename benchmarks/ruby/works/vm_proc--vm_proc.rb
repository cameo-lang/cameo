def m &b
  b
end

pr = m{
  a = 1
}

#--- benchmark: the prelude above runs once, the script below in the loop ---
pr.call

