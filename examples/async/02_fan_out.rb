# Fan out: start every call before awaiting any, then await the list of
# promises, which returns the results in the order of the list.
require "cameo/runner"

class Words
  include Cameo::Runner

  def count(text)
    text.split.size
  end

  def count_all(pages)
    promises = pages.map { |page| async self, :count, page }
    await promises
  end
end

pages = [
  "the quick brown fox",
  "jumps over",
  "the lazy dog and keeps on running"
]
counts = Words.new.count_all(pages)
puts counts.inspect
puts "#{counts.sum} words on #{pages.size} pages"
