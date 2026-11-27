# matrix: testcase to demonstrate that matrix coerces a number to a scalar
#
# Matrix#coerce and Vector#coerce give a number a Matrix::Scalar, whose
# operators are Ruby's: a number keeps it a Scalar, a matrix or a vector
# multiplies, divides by the inverse, or refuses as Ruby refuses.
#
# (Introduced in PR #289. expected output follows the __END__ separator.)
require "matrix"
def t(label)
  r = yield
  puts "#{label}: #{r.inspect}"
rescue Exception => e
  puts "#{label}: #{e.class}: #{e.message}"
end
m = Matrix[[1, 2], [3, 4]]
v = Vector[1, 2]
s = Matrix::Scalar.new(2)
t("s1") { [m.coerce(2), v.coerce(1.5), s.value, s * 3, s + 1, s - 1, s / 4.0, s ** 3] }
t("s2") { [s * m, s * v, Matrix::Scalar.new(2.0) * m, s / Matrix[[2, 0], [0, 4.0]], Matrix::Scalar.new(3) * Vector[1, 2]] }
t("s3") { s + m }
t("s4") { s - v }
t("s5") { s / v }
t("s6") { s ** m }
t("s7") { m.coerce("x") }
t("s8") { s * "x" }

__END__
s1: [[#<Matrix::Scalar value=2>, Matrix[[1, 2], [3, 4]]], [#<Matrix::Scalar value=1.5>, Vector[1, 2]], 2, #<Matrix::Scalar value=6>, #<Matrix::Scalar value=3>, #<Matrix::Scalar value=1>, #<Matrix::Scalar value=0.5>, #<Matrix::Scalar value=8>]
s2: [Matrix[[2, 4], [6, 8]], Vector[2, 4], Matrix[[2.0, 4.0], [6.0, 8.0]], Matrix[[1.0, 0.0], [0.0, 0.5]], Vector[3, 6]]
s3: ExceptionForMatrix::ErrOperationNotDefined: Operation(+) can't be defined: Integer op Matrix
s4: ExceptionForMatrix::ErrOperationNotDefined: Operation(-) can't be defined: Integer op Vector
s5: ExceptionForMatrix::ErrOperationNotDefined: Operation(/) can't be defined: Integer op Vector
s6: ExceptionForMatrix::ErrOperationNotImplemented: Sorry, Operation(**) not implemented: Integer op Matrix
s7: TypeError: Matrix can't be coerced into String
s8: TypeError: "x" can't be coerced into Matrix::Scalar
