def rec n
  if n > 0
    rec n-1
  else
    raise
  end
end

begin
  rec 10
rescue
  # ignore
end

