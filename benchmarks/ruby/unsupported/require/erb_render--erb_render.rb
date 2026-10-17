require 'erb'

data = <<erb
<html>
  <head> <%= title %> </head>
  <body>
    <h1> <%= title %> </h1>
    <p>
      <%= content %>
    </p>
  </body>
</html>
erb

title = "hello world!"
content = "hello world!\n" * 10

src = "def self.render(title, content); #{ERB.new(data).src}; end"
mod = Module.new
mod.instance_eval(src, "(ERB)")

mod.render(title, content)
