def bp_yield &b
  yield
end

#--- benchmark: the prelude above runs once, the script below in the loop ---
bp_yield{}

