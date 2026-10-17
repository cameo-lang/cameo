def m
  yield
end

#--- benchmark: the prelude above runs once, the script below in the loop ---
m{
}

