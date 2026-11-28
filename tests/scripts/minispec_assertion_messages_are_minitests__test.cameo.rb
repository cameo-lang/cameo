# minispec: testcase to demonstrate that minispec assertion messages are
# minitest's
#
# minispec is cameo's own, so these outputs are its specification; the
# messages of the assertions are minitest 5.25's.
#
# (Introduced in PR #262. expected output follows the __END__ separator.)
require "minispec"
def failing
  yield
  puts "held"
rescue Minispec::Assertion => e
  puts e.message
end
failing { assert false }
failing { assert nil, "custom" }
failing { assert_equal "a", "b" }
failing { refute_equal 1, 1 }
failing { assert_nil 1 }
failing { refute_nil nil }
failing { assert_empty [1] }
failing { refute_empty [] }
failing { assert_includes [1, 2], 3 }
failing { refute_includes [1, 2], 2 }
failing { assert_instance_of String, 1 }
failing { refute_instance_of Integer, 1 }
failing { assert_kind_of String, 1 }
failing { refute_kind_of Numeric, 1 }
failing { assert_match(/x/, "abc") }
failing { assert_match "a.c", "abc" }
failing { refute_match(/b/, "abc") }
failing { assert_respond_to 1, :upcase }
failing { refute_respond_to 1, :succ }
failing { assert_predicate 1, :zero? }
failing { refute_predicate 0, :zero? }
failing { assert_operator 1, :>, 2 }
failing { refute_operator 1, :<, 2 }
failing { assert_in_delta 1.0, 1.1 }
failing { assert_in_delta 1.0, 1.5, 0.1, "close" }
failing { refute_in_delta 1.0, 1.0001 }
failing { flunk }
failing { flunk "no" }
failing { assert_equal 1, 1 }

__END__
Expected false to be truthy.
custom.
Expected nil to be truthy.
Expected: "a"
  Actual: "b"
Expected 1 to not be equal to 1.
Expected 1 to be nil.
Expected nil to not be nil.
Expected [1] to be empty.
Expected [] to not be empty.
Expected [1, 2] to include 3.
Expected [1, 2] to not include 2.
Expected 1 to be an instance of String, not Integer.
Expected 1 to not be an instance of Integer.
Expected 1 to be a kind of String, not Integer.
Expected 1 to not be a kind of Numeric.
Expected /x/ to match "abc".
Expected /a\.c/ to match "abc".
Expected /b/ to not match "abc".
Expected 1 (Integer) to respond to #upcase.
Expected 1 to not respond to succ.
Expected 1 to be zero?.
Expected 0 to not be zero?.
Expected 1 to be > 2.
Expected 1 to not be < 2.
Expected |1.0 - 1.1| (0.10000000000000009) to be <= 0.001.
close.
Expected |1.0 - 1.5| (0.5) to be <= 0.1.
Expected |1.0 - 1.0001| (9.999999999998899e-05) to not be <= 0.001.
Epic Fail!
no
held
