# new returns instance: testcase to demonstrate that new returns the instance
# when initialize returns early
#
# new returns the instance when initialize is left by a return, with a value
# or without one, from a loop or from under an ensure.
#
# (Introduced in PR #350. expected output follows the __END__ separator.)
class A
  int :n
  def initialize(n)
    self.n = n
    return if n > 0
    self.n = -1
  end
end
class B
  int :n
  def initialize(n)
    self.n = n
    return 42
  end
end
class C
  int :n
  def initialize(n)
    self.n = n
    while true
      begin
        return :left
      ensure
        self.n = n + 1
      end
    end
  end
end
p A.new(5).n
p A.new(0).n
p B.new(7).n
p C.new(1).n
p A.send(:new, 5).n

__END__
5
-1
7
2
5
