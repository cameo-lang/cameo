# matrix: testcase to demonstrate that a number on the left of a matrix
# coerces
#
# A number on the left of a matrix or a vector goes through coerce, as Ruby's
# does, and comes out a Matrix or a Vector.
#
# (Introduced in PR #290. expected output follows the __END__ separator.)
require "matrix"
def t(label)
  r = yield
  puts "#{label}: #{r.inspect}"
rescue Exception => e
  puts "#{label}: #{e.class}: #{e.message}"
end
m = Matrix[[1, 2], [3, 4]]
v = Vector[1, 2]
t("c1") { [2 * m, 2.0 * m, 3 * v, 2.5 * v, 2 / Matrix[[2, 0], [0, 4.0]], 2 ** 3] }
t("c2") { 2 + m }
t("c3") { 2 - v }
t("c4") { 2 / v }
t("c5") { 2 ** m }
t("c10") { 2.0 / Matrix[[2, 0], [0, 4]] }
t("c11") { 2 * Matrix[[1, 2]] }
t("c12") { 3 * Vector[1, 2] }

__END__
c1: [Matrix[[2, 4], [6, 8]], Matrix[[2.0, 4.0], [6.0, 8.0]], Vector[3, 6], Vector[2.5, 5.0], Matrix[[1.0, 0.0], [0.0, 0.5]], 8]
c2: ExceptionForMatrix::ErrOperationNotDefined: Operation(+) can't be defined: Integer op Matrix
c3: ExceptionForMatrix::ErrOperationNotDefined: Operation(-) can't be defined: Integer op Vector
c4: ExceptionForMatrix::ErrOperationNotDefined: Operation(/) can't be defined: Integer op Vector
c5: ExceptionForMatrix::ErrOperationNotImplemented: Sorry, Operation(**) not implemented: Integer op Matrix
c10: Matrix[[1.0, 0.0], [0.0, 0.5]]
c11: Matrix[[2, 4]]
c12: Vector[3, 6]
