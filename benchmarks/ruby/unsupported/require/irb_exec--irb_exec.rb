# frozen_string_literal: true
require 'rbconfig'
irb_f = [File.join(File.dirname(RbConfig.ruby), 'irb'), '-f']

IO.popen(irb_f, 'w') do |io|
  io.write('exit')
end

