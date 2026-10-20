# Cameo::Runner belongs in the class that runs the work. The top level
# refuses it, on cameo and on Ruby alike; a script that wants async at the
# top level includes Cameo, which only cameo has.
require "cameo/runner"

begin
  include Cameo::Runner
rescue StandardError => e
  puts "refused: #{e.message}"
end
