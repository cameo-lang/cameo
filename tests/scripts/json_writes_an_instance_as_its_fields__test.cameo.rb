# json: testcase to demonstrate that JSON writes an instance as its fields
#
# (Introduced in PR #105. expected output follows the __END__ separator.)
class Point
  int :x, :y
end
puts JSON.generate([Point.new(x: 1)])

__END__
[{"x":1,"y":null}]
