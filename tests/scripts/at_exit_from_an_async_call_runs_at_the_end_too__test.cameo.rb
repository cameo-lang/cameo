# at exit: testcase to demonstrate that at exit from an async call runs at the
# end too
#
# The scripts are plain Ruby; what they print, and the status they end with,
# are Ruby 3.4's.
#
# (Introduced in PR #252. expected output follows the __END__ separator.)
include Cameo
class Job
  def run
    at_exit { puts "from the worker" }
    nil
  end
end
await async Job.new, :run
at_exit { puts "from the top level" }
puts "body"

__END__
body
from the top level
from the worker
