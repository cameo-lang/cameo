# A call that async starts gets copies of its receiver and its arguments,
# and its result comes back as a copy: nothing a worker does reaches the
# caller's values. A Proc crosses too, with the values it captured. A class
# cannot be copied, so it cannot be passed.
require "cameo/runner"

class Editor
  include Cameo::Runner

  def append(list, item)
    list + [item]
  end

  def each_changed(list, change)
    list.map { |item| change.call(item) }
  end

  def demo
    list = [1, 2]
    longer = await self, :append, list, 3
    puts "the worker's: #{longer.inspect}, the caller's: #{list.inspect}"

    factor = 10
    scaled = await self, :each_changed, list, ->(item) { item * factor }
    puts "changed by a proc: #{scaled.inspect}"

    begin
      async self, :append, list, String
    rescue TypeError => e
      puts e.message
    end
  end
end

Editor.new.demo
