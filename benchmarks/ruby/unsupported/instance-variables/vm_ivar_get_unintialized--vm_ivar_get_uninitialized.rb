class Example
  def read
    @uninitialized
  end
end

obj = Example.new

obj.read

