# matrix: testcase to demonstrate that matrix checks its arguments as Ruby
# does
#
# A row or column number, a size and an index are checked with Ruby's
# messages, a selector is checked before any element is looked at, a huge
# minor size is clamped, and hash, eql? and dup are Object's. The answers are
# Ruby 3.4.1's.
#
# (Introduced in PR #289. expected output follows the __END__ separator.)
require "matrix"
m = Matrix[[1, 2], [3, 4]]
[-> { m.laplace_expansion(row: 5) }, -> { m.laplace_expansion(column: -1) }, -> { Matrix.identity(-1) }, -> { Matrix.zero(-1) }, -> { Matrix.scalar(-2, 1) }, -> { Vector.zero(-1) }, -> { Vector.basis(size: 3, index: 5) }, -> { Vector.basis(size: 0, index: 0) }, -> { m.each("diagonal").to_a }, -> { Matrix.empty(0, 0).each(:bogus).to_a }, -> { m.rotate_entries(:bogus) }, -> { m.minor(1, 2**63 - 1, 0, 1) }, -> { m.map(:strict_lower) { |x| x * 10 } }, -> { m.hash == Matrix[[1, 2], [3, 4]].hash }, -> { m.eql?(Matrix[[1, 2], [3, 4]]) }, -> { m.dup == m }, -> { Vector[1, 2].eql?(Vector[1, 2]) }, -> { m.laplace_expansion(row: 1) }].each do |l|
  p l.call
rescue ArgumentError => e
  puts e.message
end

__END__
invalid num (5 for 0..1)
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
