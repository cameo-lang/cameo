IVARS = 60
class Record
  def initialize(offset = false)
    @offset = 1 if offset
    @first = 0
    IVARS.times do |i|
      instance_variable_set("@ivar_#{i}", i)
    end
  end

  def first
    @first
  end

  def lazy_set
    @lazy_set ||= 123
  end

  def undef
    @undef
  end
end

Record.new # Need one alloc to right size

BASE = Record.new
LAZY = Record.new
LAZY.lazy_set

class Miss < Record
  @first = 0
  IVARS.times do |i|
    instance_variable_set("@i_#{i}", i)
  end
end

Miss.new # Need one alloc to right size
MISS = Miss.new

DIVERGENT = Record.new(true)

BASE.first
DIVERGENT.first
BASE.first
DIVERGENT.first
BASE.first
DIVERGENT.first

