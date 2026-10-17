
def m
  i = 0
  while i<30_000_000
    i += 1
    yield
  end
end

m{}

