# securerandom: testcase to demonstrate that random formatter does not load
# secure random
#
# The expected strings are Ruby 3.4's, with securerandom 0.4.1: a seeded
# Random draws the values CRuby's does.
#
# (Introduced in PR #269. expected output follows the __END__ separator.)
require "random/formatter"
p(require "securerandom")
p(require "random/formatter")

__END__
true
false
