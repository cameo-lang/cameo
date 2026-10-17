def bp_yield
  yield
end

def bp_pass &b
  bp_yield &b
end

#--- benchmark: the prelude above runs once, the script below in the loop ---
bp_pass{}

