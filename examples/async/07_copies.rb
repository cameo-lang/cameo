# A call that async starts gets copies of its receiver and its arguments,
# and its result comes back as a copy: nothing a worker does reaches the
# caller's values. A Proc or a class cannot be copied, so it cannot be
# passed.
require "cameo/runner"

class Editor
  include Cameo::Runner

  def append(list, item)
    list + [item]
  end

  def demo
    list = [1, 2]
    longer = await self, :append, list, 3
    puts "the worker's: #{longer.inspect}, the caller's: #{list.inspect}"

    begin
      async self, :append, list, -> { 4 }
    rescue TypeError => e
      puts e.message
    end
  end
end

Editor.new.demo
