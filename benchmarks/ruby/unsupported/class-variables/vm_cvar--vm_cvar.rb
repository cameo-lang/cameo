class A
  @@foo = 1

  def self.foo
    @@foo
  end

  ("A".."Z").each do |module_name|
    eval <<-EOM
      module #{module_name}
      end

      include #{module_name}
    EOM
  end
end

A.foo
