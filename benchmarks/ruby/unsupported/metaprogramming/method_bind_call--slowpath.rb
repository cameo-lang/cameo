named_module = Kernel

module FakeName
  def self.name
    "NotMyame".freeze
  end
end

MOD_NAME = Module.instance_method(:name)

MOD_NAME.bind_call(FakeName)
