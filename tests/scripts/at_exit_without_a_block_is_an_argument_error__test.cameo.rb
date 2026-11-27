# at exit: testcase to demonstrate that at exit without a block is an argument
# error
#
# The scripts are plain Ruby; what they print, and the status they end with,
# are Ruby 3.4's.
#
# (Introduced in PR #252. expected output follows the __END__ separator.)
begin
  at_exit
rescue ArgumentError => e
  puts e.message
end

__END__
called without a block
