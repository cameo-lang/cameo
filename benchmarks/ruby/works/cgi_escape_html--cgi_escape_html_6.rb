# frozen_string_literal: true
require 'cgi/escape'

example_html = <<~HTML
  <body>
  <div>
      <h1>Example Domain</h1>
      <p>This domain is established to be used for illustrative examples in documents. You may use this
      domain in examples without prior coordination or asking for permission.</p>
      <p><a href="http://www.iana.org/domains/example">More information...</a></p>
  </div>
  </body>
HTML

#--- benchmark: the prelude above runs once, the script below in the loop ---
CGI.escapeHTML(example_html)
