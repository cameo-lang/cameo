def rec n
  if n > 0
    rec n-1
  else
    raise
  end
end

#--- benchmark: the prelude above runs once, the script below in the loop ---
begin
  rec 1
rescue
  # ignore
end

