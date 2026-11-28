# at exit: testcase to demonstrate that at exit runs the handlers last first
# when the script ends
#
# The scripts are plain Ruby; what they print, and the status they end with,
# are Ruby 3.4's.
#
# (Introduced in PR #252. expected output follows the __END__ separator.)
at_exit { puts "first registered" }
at_exit { puts "second"; at_exit { puts "nested" } }
h = at_exit { puts "third" }
p h.class
puts "body"

__END__
Proc
body
third
second
nested
first registered
