p = proc{_1}
o = Object.new
def o.each
  i = 0
  while i < 3_000_000 do
    yield i
    i += 1
  end
end


o.each(&p)

