# kernel conversion: testcase to demonstrate that integer and float check the
# exception keyword
#
# Integer() and Float() with exception: true raise as without the keyword, and
# refuse an exception: that is neither true nor false and any other keyword,
# with Ruby's messages.
#
# (Introduced in PR #447. expected output follows the __END__ separator.)
[
  -> { Integer("x", exception: true) },
  -> { Float("x", exception: true) },
  -> { Integer(nil) },
  -> { Float(nil) },
  -> { Integer("x", exception: nil) },
  -> { Float("x", exception: 1) },
  -> { Integer("1", foo: 1) },
  -> { Float("1", foo: 1) },
].each do |f|
  begin
    f.call
  rescue ArgumentError, TypeError => e
    puts "#{e.class}: #{e.message}"
  end
end

__END__
ArgumentError: invalid value for Integer(): "x"
ArgumentError: invalid value for Float(): "x"
TypeError: can't convert nil into Integer
TypeError: can't convert nil into Float
ArgumentError: expected true or false as exception: nil
ArgumentError: expected true or false as exception: 1
ArgumentError: unknown keyword: :foo
ArgumentError: unknown keyword: :foo
