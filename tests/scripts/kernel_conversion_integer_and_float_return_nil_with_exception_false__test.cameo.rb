# kernel conversion: testcase to demonstrate that integer and float return nil
# with exception false
#
# Integer() and Float() with exception: false return nil for a value that does
# not convert, as Ruby does: a string that is no number, nil, a value of a
# type with no conversion, a float that is no integer, and a value whose own
# to_int raises. A value that converts converts as without the keyword.
#
# (Introduced in PR #447. expected output follows the __END__ separator.)
class Broken
  def to_int = raise(ArgumentError, "boom")
end
p Integer("12", exception: false), Integer("x", exception: false), Float("1.5", exception: false), Float("x", exception: false)
p Integer(" 0x1A ", exception: false), Integer(2.7, exception: false), Integer(nil, exception: false), Integer([1], exception: false)
p Integer(Float::NAN, exception: false), Integer(Broken.new, exception: false)
p Float(3, exception: false), Float(nil, exception: false), Float(:a, exception: false), Float([1], exception: false)

__END__
12
nil
1.5
nil
26
2
nil
nil
nil
nil
3.0
nil
nil
nil
