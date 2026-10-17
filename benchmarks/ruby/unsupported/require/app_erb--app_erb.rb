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

ERB.new(data).result(binding)
