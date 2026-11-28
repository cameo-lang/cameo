# optparse: testcase to demonstrate that ARGV is empty when a script is given
# no arguments
#
# The expected values are Ruby 3.4's, with optparse 0.6.0. ARGV with arguments
# is checked in tests/optparse_test.go.
#
# (Introduced in PR #288. expected output follows the __END__ separator.)
p ARGV

__END__
[]
