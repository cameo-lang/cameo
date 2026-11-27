# unicode normalize: testcase to demonstrate that unicode normalize keeps
# Ruby's encoding rules
#
# The encoding rules are Ruby's: bytes that are not UTF-8, which are a binary
# string, are refused, a US-ASCII one is as it is and normalized, a binary one
# is refused after the arguments are counted.
#
# The expected strings are Ruby 3.4's.
#
# (Introduced in PR #297. expected output follows the __END__ separator.)
begin; b"\xff".unicode_normalize; rescue Encoding::CompatibilityError => e; puts e.message; end
begin; b"\xff".unicode_normalized?; rescue Encoding::CompatibilityError => e; puts e.message; end
p "e".force_encoding("US-ASCII").unicode_normalize(:nfd).bytes, "e".force_encoding("US-ASCII").unicode_normalized?(:nfd)
begin; "x".b.unicode_normalize(:nfc, :nfd); rescue ArgumentError => e; puts e.message; end
begin; "x".b.unicode_normalize; rescue Encoding::CompatibilityError => e; puts e.message; end

__END__
Unicode Normalization not appropriate for ASCII-8BIT
Unicode Normalization not appropriate for ASCII-8BIT
[101]
true
wrong number of arguments (given 2, expected 0..1)
Unicode Normalization not appropriate for ASCII-8BIT
