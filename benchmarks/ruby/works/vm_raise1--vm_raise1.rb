def rec n
  if n > 0
    rec n-1
  else
    raise
  end
end

begin
  rec 1
rescue
  # ignore
end

