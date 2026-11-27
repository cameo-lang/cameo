# binary paths: testcase to demonstrate that an exception message with bytes
# is readable
#
# An exception's message is text for a reader, whatever bytes it was given: a
# byte that is part of no character is written as \xNN. Reading it in a rescue
# raises nothing.
#
# Each test here is a path that handles a binary String, or bytes that are not
# UTF-8, and raised when Strings began to be checked where they are made: a
# review found them.
#
# (Introduced in PR #438. expected output follows the __END__ separator.)
begin
  raise ArgumentError, b"\xFFa"
rescue => e
  p e.message, e.message.encoding, e.to_s, e.inspect, e.full_message
end
begin
  nil.send(b"\xFFa")
rescue NoMethodError => e
  p e.message
end
begin
  raise "héllo"
rescue => e
  p e.message
end

__END__
"\\xFFa"
#<Encoding:UTF-8>
"\\xFFa"
"#<ArgumentError: \\xFFa>"
"\\xFFa (ArgumentError)"
"undefined method '\\xFFa' for nil"
"héllo"
