def bp_yield
  yield
end

def bp_pass &b
  bp_yield &b
end

bp_pass{}

