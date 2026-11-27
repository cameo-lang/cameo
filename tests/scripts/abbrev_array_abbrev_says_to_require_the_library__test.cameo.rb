# abbrev: testcase to demonstrate that array abbrev says to require the
# library
#
# The expected values are Ruby's, with abbrev 0.1.2.
#
# (Introduced in PR #281. expected output follows the __END__ separator.)
begin; [].abbrev; rescue NoMethodError => e; puts e.message; end

__END__
undefined method 'abbrev' for an instance of Array: require "abbrev" first
