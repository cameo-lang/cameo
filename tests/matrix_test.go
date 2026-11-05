package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected values are Ruby 3.4's, with matrix 0.4.2, but where cameo
// has no Rational: the inverse of an Integer matrix, and what is computed
// from it (arith2, props, inv, div, float), has Floats where Ruby has
// Rationals; Matrix ** Float, which Ruby does through the eigensystem,
// is ErrOperationNotImplemented (pow).
func TestMatrixMatchesRuby(t *testing.T) {
	out := runScript(t, `require "matrix"
def t(label)
  r = yield
  puts "#{label}: #{r.inspect}"
rescue Exception => e
  puts "#{label}: #{e.class}: #{e.message}"
end
m = Matrix[[1, 2], [3, 4]]
v = Vector[1, 2, 3]
t("basic") { [m, m.row_count, m.column_count, m[0, 1], m[1, 0], m[2, 0], m.row(0), m.column(1), m.to_a, m.square?, m.empty?, m.row_size, m.column_size] }
t("build") { [Matrix.rows([[1, 2], [3, 4]]), Matrix.columns([[1, 2], [3, 4]]), Matrix.identity(2), Matrix.unit(2), Matrix.zero(2), Matrix.zero(2, 3), Matrix.diagonal(1, 2), Matrix.scalar(2, 5), Matrix.build(2, 3) { |r, c| r * 10 + c }, Matrix.build(2) { 0 }, Matrix.row_vector([1, 2]), Matrix.column_vector([1, 2]), Matrix.empty(0, 2), Matrix.empty(2, 0), Matrix[], Matrix.vstack(m, m), Matrix.hstack(m, m), Matrix.combine(m, m) { |a, b| a + b }] }
t("arith2") { [m + m, m - m, m * m, m * 2, m / 2, m ** 2, m ** 0, m ** -1, m ** 3, -m, m * Vector[1, 1], m.map { |x| x * 2 }, m.collect(&:to_s), m / 2.0, m * 1.5, m + Matrix.identity(2)] }
t("arith3") { m + Matrix[[1, 2, 3]] }
t("arith4") { m * Matrix[[1, 2, 3]] }
t("arith5") { m + 1 }
t("arith6") { [m.hadamard_product(m), m.entrywise_product(m), m.each_with_index.to_a, m.each.to_a, m.each(:diagonal).to_a, m.each(:off_diagonal).to_a, m.each(:lower).to_a, m.each(:upper).to_a, m.each(:strict_lower).to_a, m.each(:strict_upper).to_a] }
t("props") { [m.determinant, m.det, m.inverse, m.inv, m.transpose, m.t, m.rank, m.trace, m.tr, m.minor(0, 1, 0, 2), m.minor(0..0, 0..1), m.first_minor(0, 0), m.cofactor(0, 1), m.adjugate, m.laplace_expansion(row: 0), m.cofactor_expansion(column: 1), m.symmetric?, m.antisymmetric?, m.diagonal?, m.lower_triangular?, m.upper_triangular?, m.regular?, m.singular?, m.zero?, m.real?, m.normal?, m.orthogonal?, m.permutation?, m.unitary?, m.hermitian?] }
t("det") { [Matrix[[2, 0], [0, 3]].det, Matrix[[1, 2, 3], [4, 5, 6], [7, 8, 10]].det, Matrix[[1.5, 2], [3, 4]].det, Matrix[[1]].det, Matrix.empty(0, 0).det, Matrix[[1, 2, 3], [4, 5, 6], [7, 8, 9]].det] }
t("det2") { Matrix[[1, 2, 3], [4, 5, 6]].det }
t("inv") { [Matrix[[2, 0], [0, 4]].inverse, Matrix[[1, 2], [3, 4]].inverse, Matrix[[1.0, 2], [3, 4]].inverse, Matrix[[4, 7], [2, 6]].inv, Matrix[[1, 2, 3], [0, 1, 4], [5, 6, 0]].inverse] }
t("inv2") { Matrix[[1, 2], [2, 4]].inverse }
t("inv3") { Matrix[[1, 2, 3], [4, 5, 6]].inverse }
t("rank") { [Matrix[[1, 2], [2, 4]].rank, Matrix[[1, 2, 3], [4, 5, 6], [7, 8, 9]].rank, Matrix.zero(3).rank, Matrix[[1, 2, 3], [4, 5, 6]].rank, Matrix.identity(4).rank] }
t("eq") { [m == Matrix[[1, 2], [3, 4]], m == Matrix[[1, 2], [3, 5]], m.eql?(Matrix[[1, 2], [3, 4]]), m == Matrix[[1.0, 2], [3, 4]], m.eql?(Matrix[[1.0, 2], [3, 4]]), m.hash == Matrix[[1, 2], [3, 4]].hash, m == [[1, 2], [3, 4]], m.clone == m, Matrix[[1, 2], [3, 4]] <=> m] }
t("str") { [m.to_s, m.inspect, Matrix.empty(0, 2).to_s, Matrix.empty(2, 0).inspect, Matrix[[1.5, 2]].to_s, m.to_a, m.to_matrix, Matrix[[1, 2]].row_vectors, Matrix[[1, 2]].column_vectors] }
t("vec") { [v, v.size, v[0], v[-1], v[3], v.to_a, v + v, v - v, v * 2, v / 2, v / 2.0, -v, v.inner_product(v), v.dot(v), v.cross_product(Vector[0, 0, 1]), v.cross(Vector[1, 0, 0]), v.norm, v.magnitude, v.r, v.normalize, v.map { |x| x * x }, v.collect2(v) { |a, b| a * b }, v.each2(v).to_a, v.zero?, Vector.zero(3), Vector.basis(size: 3, index: 1), Vector.elements([1, 2]), Vector.elements([1, 2], false), v.to_matrix, v.covector, v.angle_with(Vector[1, 2, 3]), v.angle_with(Vector[-1, -2, -3]).round(6), v.independent?(Vector[2, 4, 6]), Vector.independent?(v, Vector[1, 0, 0]), v.each.to_a, v.each_with_index.to_a, v.to_s, v.inspect, v == Vector[1, 2, 3], v.eql?(Vector[1, 2, 3]), v.hash == Vector[1, 2, 3].hash, v.sum, v.max, v.round(1), Vector[1.5, 2.5].round, v.zero?, Vector.zero(2).zero?, v.element(1), v.component(2), v.cross_product(Vector[1, 2, 3]), Vector[1, 2].cross_product, v.to_a.class] }
t("vec2") { v + Vector[1, 2] }
t("vec3") { v.inner_product(Vector[1, 2]) }
t("vec4") { Vector[1, 2].cross_product(Vector[3, 4]) }
t("vec5") { [v * v.covector, v.covector * v, m * Vector[1, 2], Vector[1, 2] * m] }
t("vec6") { Vector[1, 2] * Vector[1, 2] }
t("vec7") { [v * Matrix[[1, 2]], Vector[1, 2].covector * m] }
t("vec8") { [Vector[3, 4].normalize, Vector[0, 0].normalize] }
t("vec9") { Vector[1, 2].angle_with(Vector[0, 0]) }
t("errs") { [Matrix::ErrDimensionMismatch.superclass, Matrix::ErrNotRegular.superclass, Matrix::ErrOperationNotDefined.superclass, Matrix::ErrOperationNotImplemented.superclass, Vector::ZeroVectorError.superclass, ExceptionForMatrix::ErrDimensionMismatch] }
t("errs2") { Matrix[[1, 2], [3]] }
t("errs3") { Matrix.rows([[1, 2], [3]]) }
t("errs4") { Matrix[[1, 2], [3, 4], [5, 6]].det }
t("errs5") { Matrix[[1, 2], [3, 4], [5, 6]].inverse }
t("errs6") { Matrix[[1, 2], [3, 4], [5, 6]] ** 2 }
t("errs7") { Matrix[[1, 2], [3, 4], [5, 6]].trace }
t("errs8") { m.row(5) }
t("errs9") { m.column(5) }
t("errs10") { Matrix[[1, 2], [3, 4]] / Matrix[[1, 2], [2, 4]] }
t("div") { [m / Matrix[[2, 0], [0, 2]], Matrix[[2, 4], [6, 8]] / 2, Matrix[[2, 4], [6, 8]] / Matrix[[1, 0], [0, 1]]] }
t("misc") { [m.each_slice(2).to_a, m.find_index(3), m.find_index { |x| x > 2 }, m.find_index(:diagonal) { |x| x > 1 }, m.index(4), m.element(1, 1), m.component(0, 0), m[-1, -1], m.minor(1, 1, 1, 1), m.minor(0, 5, 0, 5), m.minor(3, 1, 0, 1), m.conj, m.conjugate, m.imag, m.imaginary, m.real, m.rect, m.round(1), Matrix[[1.26, 2.5]].round(1), Matrix[[1.26, 2.5]].round, m.freeze.frozen?, m.sum, m.max, m.include?(2), m.to_a.flatten] } 
t("misc3") { [Matrix[[1, 2], [3, 4]].vstack(Matrix[[5, 6]]), Matrix[[1, 2], [3, 4]].hstack(Matrix[[5], [6]]), Matrix[[1, 2], [3, 4]].rotate_entries, Matrix[[1, 2], [3, 4]].rotate_entries(:counter_clockwise), Matrix[[1, 2], [3, 4]].rotate_entries(:half_turn), Matrix[[1, 2], [3, 4]].rotate_entries(2), Matrix[[1, 2], [3, 4]].rotate_entries(-1), Matrix[[1, 2], [3, 4]].antisymmetric?, Matrix[[0, 1], [-1, 0]].antisymmetric?, Matrix[[0, 1], [-1, 0]].skew_symmetric?] }
t("mut") { x = Matrix[[1, 2], [3, 4]]; x.map { |v| v == 1 ? 9 : v } }
t("mut2") { x = Vector[1, 2]; x[0] = 9; x }
t("mut3") { m.frozen? }
t("float") { [Matrix[[1.0, 2.0], [3.0, 4.0]].inverse, Matrix[[1, 2], [3, 4]].inverse.to_a.flatten.map(&:class), Matrix[[1, 2], [3, 4]] / 2, (Matrix[[1, 2], [3, 4]] / 2).to_a.flatten.map(&:class), Matrix[[1, 2], [3, 4]] ** -2] }
t("pow") { Matrix[[1, 2], [3, 4]] ** 1.5 }
t("rows_arr") { [Matrix.rows([[1, 2]], false), Matrix.rows([]), Matrix.rows([]).row_count, Matrix.rows([]).column_count, Matrix.columns([]), Matrix[[]], Matrix[[]].row_count, Matrix[[]].column_count, Matrix[[]].inspect] }
t("vector_ops") { [Vector[1, 2, 3].each_slice(2).to_a, Vector[1, 2, 3].to_a.sum, Vector[1, 2, 3][0..1], Vector[1, 2, 3].size, Vector[].size, Vector[].to_s, Vector[].inspect, Vector[1, 2] == Matrix.column_vector([1, 2]), Vector[1, 2].to_matrix == Matrix.column_vector([1, 2]), Vector[0.1, 0.2].round(1), Vector[1, 2].map2(Vector[3, 4]) { |a, b| a + b }, Vector[3, 4].magnitude, Vector[3, 4].norm.class, Vector[1, 2].inner_product(Vector[3, 4])] }
t("matrix_index") { [m[0], m[0, 0], m[0, 1], m[0, 5], m[5, 0], m[-1, 0], m[-3, 0], m.row(-1), m.column(-1), m.row(0).class, m.row(0) { |x| x }, m.column(0) { |x| x }, m.each_with_index { |x, i, j| }, m.each { |x| }] }
t("laplace") { [Matrix[[1, 2, 3], [4, 5, 6], [7, 8, 10]].laplace_expansion(row: 1), Matrix[[1, 2], [3, 4]].laplace_expansion(column: 0), Matrix[[1, 2, 3], [4, 5, 6], [7, 8, 10]].cofactor(1, 1), Matrix[[1, 2, 3], [4, 5, 6], [7, 8, 10]].adjugate, Matrix[[1, 2, 3], [4, 5, 6], [7, 8, 10]].first_minor(1, 1)] }
t("laplace2") { m.laplace_expansion }
t("laplace3") { m.laplace_expansion(row: 0, column: 0) }
t("laplace4") { Matrix[[1, 2, 3]].laplace_expansion(row: 0) }
t("vector_cross") { [Vector[1, 0, 0].cross(Vector[0, 1, 0]), Vector[1, 0, 0, 0].cross_product(Vector[0, 1, 0, 0], Vector[0, 0, 1, 0]), Vector[1, 2].cross_product] }
t("minor_edge") { [m.minor(0, 0, 0, 0), m.minor(2, 1, 0, 1), m.minor(1..5, 0..0), m.minor(0, 2, 2, 1), m.minor(-1, 1, 0, 1)] }
t("triangular") { [Matrix[[1, 0], [2, 3]].lower_triangular?, Matrix[[1, 2], [0, 3]].upper_triangular?, Matrix[[1, 2], [0, 3]].lower_triangular?, Matrix[[1, 0], [0, 3]].diagonal?, Matrix.identity(3).orthogonal?, Matrix[[0, 1], [1, 0]].permutation?, Matrix[[1, 2], [2, 1]].symmetric?, Matrix[[1, 2], [2, 1]].normal?, Matrix.identity(2).unitary?, Matrix[[1, 2], [3, 4]].lower_triangular?] }
t("tri2") { Matrix[[1, 2, 3]].symmetric? }
t("misc4") { [m.row_vectors.map(&:class), m.column_vectors, Matrix.identity(2).to_s, Matrix[[1, 2], [3, 4]].each(:bogus).to_a] }
t("hash") { [{m => 1}[Matrix[[1, 2], [3, 4]]], [m, Matrix[[1, 2], [3, 4]]].uniq.size, {v => 1}[Vector[1, 2, 3]]] }
`)
	assert.Equals(t, out, `basic: [Matrix[[1, 2], [3, 4]], 2, 2, 2, 3, nil, Vector[1, 2], Vector[2, 4], [[1, 2], [3, 4]], true, false, 2, 2]
build: [Matrix[[1, 2], [3, 4]], Matrix[[1, 3], [2, 4]], Matrix[[1, 0], [0, 1]], Matrix[[1, 0], [0, 1]], Matrix[[0, 0], [0, 0]], Matrix[[0, 0, 0], [0, 0, 0]], Matrix[[1, 0], [0, 2]], Matrix[[5, 0], [0, 5]], Matrix[[0, 1, 2], [10, 11, 12]], Matrix[[0, 0], [0, 0]], Matrix[[1, 2]], Matrix[[1], [2]], Matrix.empty(0, 2), Matrix.empty(2, 0), Matrix.empty(0, 0), Matrix[[1, 2], [3, 4], [1, 2], [3, 4]], Matrix[[1, 2, 1, 2], [3, 4, 3, 4]], Matrix[[2, 4], [6, 8]]]
arith2: [Matrix[[2, 4], [6, 8]], Matrix[[0, 0], [0, 0]], Matrix[[7, 10], [15, 22]], Matrix[[2, 4], [6, 8]], Matrix[[0, 1], [1, 2]], Matrix[[7, 10], [15, 22]], Matrix[[1, 0], [0, 1]], Matrix[[-2.0, 1.0], [1.5, -0.5]], Matrix[[37, 54], [81, 118]], Matrix[[-1, -2], [-3, -4]], Vector[3, 7], Matrix[[2, 4], [6, 8]], Matrix[["1", "2"], ["3", "4"]], Matrix[[0.5, 1.0], [1.5, 2.0]], Matrix[[1.5, 3.0], [4.5, 6.0]], Matrix[[2, 2], [3, 5]]]
arith3: ExceptionForMatrix::ErrDimensionMismatch: Dimension mismatch
arith4: ExceptionForMatrix::ErrDimensionMismatch: Dimension mismatch
arith5: ExceptionForMatrix::ErrOperationNotDefined: Operation(+) can't be defined: Matrix op Integer
arith6: [Matrix[[1, 4], [9, 16]], Matrix[[1, 4], [9, 16]], [[1, 0, 0], [2, 0, 1], [3, 1, 0], [4, 1, 1]], [1, 2, 3, 4], [1, 4], [2, 3], [1, 3, 4], [1, 2, 4], [3], [2]]
props: [-2, -2, Matrix[[-2.0, 1.0], [1.5, -0.5]], Matrix[[-2.0, 1.0], [1.5, -0.5]], Matrix[[1, 3], [2, 4]], Matrix[[1, 3], [2, 4]], 2, 5, 5, Matrix[[1, 2]], Matrix[[1, 2]], Matrix[[4]], -3, Matrix[[4, -2], [-3, 1]], -2, -2, false, false, false, false, false, true, false, false, true, false, false, false, false, false]
det: [6, -3, 0.0, 1, 1, 0]
det2: ExceptionForMatrix::ErrDimensionMismatch: Dimension mismatch
inv: [Matrix[[0.5, 0.0], [0.0, 0.25]], Matrix[[-2.0, 1.0], [1.5, -0.5]], Matrix[[-1.9999999999999998, 0.9999999999999998], [1.4999999999999998, -0.49999999999999994]], Matrix[[0.6, -0.7], [-0.2, 0.4]], Matrix[[-24.0, 18.0, 5.0], [20.0, -15.0, -4.0], [-5.0, 4.0, 1.0]]]
inv2: ExceptionForMatrix::ErrNotRegular: Not Regular Matrix
inv3: ExceptionForMatrix::ErrDimensionMismatch: Dimension mismatch
rank: [1, 2, 0, 2, 4]
eq: [true, false, true, true, false, true, false, true, 0]
str: ["Matrix[[1, 2], [3, 4]]", "Matrix[[1, 2], [3, 4]]", "Matrix.empty(0, 2)", "Matrix.empty(2, 0)", "Matrix[[1.5, 2]]", [[1, 2], [3, 4]], Matrix[[1, 2], [3, 4]], [Vector[1, 2]], [Vector[1], Vector[2]]]
vec: [Vector[1, 2, 3], 3, 1, 3, nil, [1, 2, 3], Vector[2, 4, 6], Vector[0, 0, 0], Vector[2, 4, 6], Vector[0, 1, 1], Vector[0.5, 1.0, 1.5], Vector[-1, -2, -3], 14, 14, Vector[2, -1, 0], Vector[0, 3, -2], 3.7416573867739413, 3.7416573867739413, 3.7416573867739413, Vector[0.2672612419124244, 0.5345224838248488, 0.8017837257372732], Vector[1, 4, 9], [1, 4, 9], [[1, 1], [2, 2], [3, 3]], false, Vector[0, 0, 0], Vector[0, 1, 0], Vector[1, 2], Vector[1, 2], Matrix[[1], [2], [3]], Matrix[[1, 2, 3]], 0, 3.141593, false, true, [1, 2, 3], [[1, 0], [2, 1], [3, 2]], "Vector[1, 2, 3]", "Vector[1, 2, 3]", true, true, true, 6, 3, Vector[1, 2, 3], Vector[2, 3], false, true, 2, 3, Vector[0, 0, 0], Vector[-2, 1], Array]
vec2: ExceptionForMatrix::ErrDimensionMismatch: Dimension mismatch
vec3: ExceptionForMatrix::ErrDimensionMismatch: Dimension mismatch
vec4: ArgumentError: wrong number of arguments (1 for 0)
vec5: ExceptionForMatrix::ErrDimensionMismatch: Dimension mismatch
vec6: ExceptionForMatrix::ErrOperationNotDefined: Operation(*) can't be defined: Vector op Vector
vec7: [Matrix[[1, 2], [2, 4], [3, 6]], Matrix[[7, 10]]]
vec8: Vector::ZeroVectorError: Zero vectors can not be normalized
vec9: Vector::ZeroVectorError: Can't get angle of zero vector
errs: [StandardError, StandardError, StandardError, StandardError, StandardError, ExceptionForMatrix::ErrDimensionMismatch]
errs2: ExceptionForMatrix::ErrDimensionMismatch: row size differs (1 should be 2)
errs3: ExceptionForMatrix::ErrDimensionMismatch: row size differs (1 should be 2)
errs4: ExceptionForMatrix::ErrDimensionMismatch: Dimension mismatch
errs5: ExceptionForMatrix::ErrDimensionMismatch: Dimension mismatch
errs6: ExceptionForMatrix::ErrDimensionMismatch: Dimension mismatch
errs7: ExceptionForMatrix::ErrDimensionMismatch: Dimension mismatch
errs8: nil
errs9: nil
errs10: ExceptionForMatrix::ErrNotRegular: Not Regular Matrix
div: [Matrix[[0.5, 1.0], [1.5, 2.0]], Matrix[[1, 2], [3, 4]], Matrix[[2.0, 4.0], [6.0, 8.0]]]
misc: [[[1, 2], [3, 4]], [1, 0], [1, 0], [1, 1], [1, 1], 4, 1, 4, Matrix[[4]], Matrix[[1, 2], [3, 4]], nil, Matrix[[1, 2], [3, 4]], Matrix[[1, 2], [3, 4]], Matrix[[0, 0], [0, 0]], Matrix[[0, 0], [0, 0]], Matrix[[1, 2], [3, 4]], [Matrix[[1, 2], [3, 4]], Matrix[[0, 0], [0, 0]]], Matrix[[1, 2], [3, 4]], Matrix[[1.3, 2.5]], Matrix[[1, 3]], true, 10, 4, true, [1, 2, 3, 4]]
misc3: [Matrix[[1, 2], [3, 4], [5, 6]], Matrix[[1, 2, 5], [3, 4, 6]], Matrix[[3, 1], [4, 2]], Matrix[[2, 4], [1, 3]], Matrix[[4, 3], [2, 1]], Matrix[[4, 3], [2, 1]], Matrix[[2, 4], [1, 3]], false, true, true]
mut: Matrix[[9, 2], [3, 4]]
mut2: Vector[9, 2]
mut3: true
float: [Matrix[[-1.9999999999999998, 0.9999999999999998], [1.4999999999999998, -0.49999999999999994]], [Float, Float, Float, Float], Matrix[[0, 1], [1, 2]], [Integer, Integer, Integer, Integer], Matrix[[5.5, -2.5], [-3.75, 1.75]]]
pow: ExceptionForMatrix::ErrOperationNotImplemented: Sorry, Operation(**) not implemented: Matrix op Float
rows_arr: [Matrix[[1, 2]], Matrix.empty(0, 0), 0, 0, Matrix.empty(0, 0), Matrix.empty(1, 0), 1, 0, "Matrix.empty(1, 0)"]
vector_ops: [[[1, 2], [3]], 6, [1, 2], 3, 0, "Vector[]", "Vector[]", false, true, Vector[0.1, 0.2], Vector[4, 6], 5.0, Float, 11]
matrix_index: ArgumentError: wrong number of arguments (given 1, expected 2)
laplace: [-3, -2, -11, Matrix[[2, 4, -3], [2, -11, 6], [-3, 6, -3]], Matrix[[1, 3], [7, 10]]]
laplace2: ArgumentError: exactly one the row or column arguments must be specified
laplace3: ArgumentError: exactly one the row or column arguments must be specified
laplace4: ExceptionForMatrix::ErrDimensionMismatch: Dimension mismatch
vector_cross: [Vector[0, 0, 1], Vector[0, 0, 0, 1], Vector[-2, 1]]
minor_edge: [Matrix.empty(0, 0), Matrix.empty(0, 1), Matrix[[3]], Matrix.empty(2, 0), Matrix[[3]]]
triangular: [true, true, false, true, true, true, true, true, true, false]
tri2: ExceptionForMatrix::ErrDimensionMismatch: Dimension mismatch
misc4: ArgumentError: expected :bogus to be one of :all, :diagonal, :off_diagonal, :lower, :strict_lower, :strict_upper or :upper
hash: [1, 1, 1]
`)
}

func TestMatrixIsAValue(t *testing.T) {
	out := runScript(t, `require "matrix"
m = Matrix[[1, 2], [3, 4]]
n = m
x = m.map { |v| v * 10 }
p m, x, n == m, Set[m, Matrix[[1, 2], [3, 4]], x].size, {m => 1}.key?(Matrix[[1, 2], [3, 4]])
v = Vector[1, 2]
w = v
v[0] = 9
p v, w
`)
	assert.Equals(t, out, "Matrix[[1, 2], [3, 4]]\nMatrix[[10, 20], [30, 40]]\ntrue\n2\ntrue\nVector[9, 2]\nVector[1, 2]\n")
}

// A row or column number, a size and an index are checked with Ruby's
// messages, a selector is checked before any element is looked at, a
// huge minor size is clamped, and hash, eql? and dup are Object's. The
// answers are Ruby 3.4.1's.
func TestMatrixChecksItsArgumentsAsRubyDoes(t *testing.T) {
	out := runScript(t, `require "matrix"
m = Matrix[[1, 2], [3, 4]]
[-> { m.laplace_expansion(row: 5) }, -> { m.laplace_expansion(column: -1) }, -> { Matrix.identity(-1) }, -> { Matrix.zero(-1) }, -> { Matrix.scalar(-2, 1) }, -> { Vector.zero(-1) }, -> { Vector.basis(size: 3, index: 5) }, -> { Vector.basis(size: 0, index: 0) }, -> { m.each("diagonal").to_a }, -> { Matrix.empty(0, 0).each(:bogus).to_a }, -> { m.rotate_entries(:bogus) }, -> { m.minor(1, 2**63 - 1, 0, 1) }, -> { m.map(:strict_lower) { |x| x * 10 } }, -> { m.hash == Matrix[[1, 2], [3, 4]].hash }, -> { m.eql?(Matrix[[1, 2], [3, 4]]) }, -> { m.dup == m }, -> { Vector[1, 2].eql?(Vector[1, 2]) }, -> { m.laplace_expansion(row: 1) }].each do |l|
  p l.call
rescue ArgumentError => e
  puts e.message
end
`)
	assert.Equals(t, out, `invalid num (5 for 0..1)
invalid num (-1 for 0..1)
negative array size
negative array size
negative array size
invalid size (-1 for 0..)
invalid index (5 for 0...3)
invalid size (0 for 1..)
expected "diagonal" to be one of :all, :diagonal, :off_diagonal, :lower, :strict_lower, :strict_upper or :upper
expected :bogus to be one of :all, :diagonal, :off_diagonal, :lower, :strict_lower, :strict_upper or :upper
expected :bogus to be one of :clockwise, :counter_clockwise, :half_turn or an integer
Matrix[[3]]
Matrix[[1, 2], [30, 4]]
true
true
true
true
-2
`)
}

// Matrix#coerce and Vector#coerce give a number a Matrix::Scalar, whose
// operators are Ruby's: a number keeps it a Scalar, a matrix or a vector
// multiplies, divides by the inverse, or refuses as Ruby refuses.
func TestMatrixCoercesANumberToAScalar(t *testing.T) {
	out := runScript(t, `require "matrix"
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
`)
	assert.Equals(t, out, `s1: [[#<Matrix::Scalar value=2>, Matrix[[1, 2], [3, 4]]], [#<Matrix::Scalar value=1.5>, Vector[1, 2]], 2, #<Matrix::Scalar value=6>, #<Matrix::Scalar value=3>, #<Matrix::Scalar value=1>, #<Matrix::Scalar value=0.5>, #<Matrix::Scalar value=8>]
s2: [Matrix[[2, 4], [6, 8]], Vector[2, 4], Matrix[[2.0, 4.0], [6.0, 8.0]], Matrix[[1.0, 0.0], [0.0, 0.5]], Vector[3, 6]]
s3: ExceptionForMatrix::ErrOperationNotDefined: Operation(+) can't be defined: Integer op Matrix
s4: ExceptionForMatrix::ErrOperationNotDefined: Operation(-) can't be defined: Integer op Vector
s5: ExceptionForMatrix::ErrOperationNotDefined: Operation(/) can't be defined: Integer op Vector
s6: ExceptionForMatrix::ErrOperationNotImplemented: Sorry, Operation(**) not implemented: Integer op Matrix
s7: TypeError: Matrix can't be coerced into String
s8: TypeError: "x" can't be coerced into Matrix::Scalar
`)
}

// A number on the left of a matrix or a vector goes through coerce, as
// Ruby's does, and comes out a Matrix or a Vector.
func TestANumberOnTheLeftOfAMatrixCoerces(t *testing.T) {
	out := runScript(t, `require "matrix"
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
`)
	assert.Equals(t, out, `c1: [Matrix[[2, 4], [6, 8]], Matrix[[2.0, 4.0], [6.0, 8.0]], Vector[3, 6], Vector[2.5, 5.0], Matrix[[1.0, 0.0], [0.0, 0.5]], 8]
c2: ExceptionForMatrix::ErrOperationNotDefined: Operation(+) can't be defined: Integer op Matrix
c3: ExceptionForMatrix::ErrOperationNotDefined: Operation(-) can't be defined: Integer op Vector
c4: ExceptionForMatrix::ErrOperationNotDefined: Operation(/) can't be defined: Integer op Vector
c5: ExceptionForMatrix::ErrOperationNotImplemented: Sorry, Operation(**) not implemented: Integer op Matrix
c10: Matrix[[1.0, 0.0], [0.0, 0.5]]
c11: Matrix[[2, 4]]
c12: Vector[3, 6]
`)
}
