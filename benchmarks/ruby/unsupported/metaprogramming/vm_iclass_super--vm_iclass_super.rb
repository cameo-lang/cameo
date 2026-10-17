class C
  def m
    1
  end

  ("A".."M").each do |module_name|
    eval <<-EOM
        module #{module_name}
          def m; super; end
        end
        prepend #{module_name}
    EOM
  end
end

obj = C.new

obj.m
