class C
  attr_accessor :x
  def initialize
    @x = nil
  end
  class_eval <<-END
    def ar
      #{'x;'*256}
    end
    def aw
      #{'self.x = nil;'*256}
    end
    def arm
      m = method(:x)
      #{'m.call;'*256}
    end
    def awm
      m = method(:x=)
      #{'m.call(nil);'*256}
    end
  END
end
obj = C.new

obj.awm
