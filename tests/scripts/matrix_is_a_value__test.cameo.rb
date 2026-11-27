# matrix: testcase to demonstrate that matrix is a value
#
# (Introduced in PR #289. expected output follows the __END__ separator.)
require "matrix"
m = Matrix[[1, 2], [3, 4]]
n = m
x = m.map { |v| v * 10 }
p m, x, n == m, Set[m, Matrix[[1, 2], [3, 4]], x].size, {m => 1}.key?(Matrix[[1, 2], [3, 4]])
v = Vector[1, 2]
w = v
v[0] = 9
p v, w

__END__
Matrix[[1, 2], [3, 4]]
Matrix[[10, 20], [30, 40]]
true
2
true
Vector[9, 2]
Vector[1, 2]
