require 'irb/color'
code = <<~'CODE'
  def self.foo # bar
    :"erb #{ERB.new("<%= self %>", trim_mode: ?-).result}"
  end
CODE

IRB::Color.colorize_code(code, complete: false)

