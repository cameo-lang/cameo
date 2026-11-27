# uri: testcase to demonstrate that URI parses every kind of reference
#
# The scripts are plain Ruby, and the expected outputs are Ruby 3.4's, with
# uri 1.0.2.
#
# (Introduced in PR #263. expected output follows the __END__ separator.)
require "uri"
p URI.split("mailto:joe@example.com?subject=hi")
p URI.split("http://"), URI.split("http:"), URI.split(""), URI.split("//h/p"), URI.split("?q")
p URI("http://h?q").path, URI("http://").host, URI("urn:isbn:123").opaque, URI("x.com/p").path
p URI("http://u@x.com").password, URI("http://u:@x.com").password, URI("http://:p@x.com").user
p URI("http://x.com:/").port, URI("http://x.com:0").to_s, URI("http://x.com:65536").port

__END__
["mailto", nil, nil, nil, nil, nil, "joe@example.com?subject=hi", nil, nil]
["http", nil, "", nil, nil, "", nil, nil, nil]
["http", nil, nil, nil, nil, "", nil, nil, nil]
[nil, nil, nil, nil, nil, "", nil, nil, nil]
[nil, nil, "h", nil, nil, "/p", nil, nil, nil]
[nil, nil, nil, nil, nil, "", nil, "q", nil]
""
""
"isbn:123"
"x.com/p"
nil
""
""
80
"http://x.com:0"
65536
