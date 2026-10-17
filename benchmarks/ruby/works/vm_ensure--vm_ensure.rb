
#--- benchmark: the prelude above runs once, the script below in the loop ---
i = 0
while i<30_000_000
  i += 1
  begin
    begin
    ensure
    end
  ensure
  end
end

