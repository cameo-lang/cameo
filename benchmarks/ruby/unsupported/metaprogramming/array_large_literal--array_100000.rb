def def_array(size)
  Object.class_eval(<<-END)
    def array_#{size}
      x = 1
      [#{(['x'] * size).join(',')}]
    end
  END
end
def_array(100)
def_array(1000)
def_array(10000)
def_array(100000)

array_100000
