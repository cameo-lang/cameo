class Example
  def lazy_set
    @uninitialized ||= 123
  end
end

objs = 10000000.times.map { Example.new }

objs.each(&:lazy_set)

