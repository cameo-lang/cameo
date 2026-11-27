# uri: testcase to demonstrate that URI merge and route follow Ruby
#
# The scripts are plain Ruby, and the expected outputs are Ruby 3.4's, with
# uri 1.0.2.
#
# (Introduced in PR #263. expected output follows the __END__ separator.)
require "uri"
b = URI("http://x.com/a/b")
puts b.merge("//y.com/b"), b.merge("?q"), b.merge("#f"), b.merge(""), b.merge("c"), b.merge("/c"), b.merge("other:x")
puts URI("http://x.com/a/b/").merge(".."), URI("http://x.com/a/b?q").merge("c"), URI("http://x.com").merge("c")
[["http://x.com/a/b", "http://x.com/a/b"], ["http://x.com/a/b", "http://x.com/a/"], ["http://x.com/a/b", "http://x.com/c"],
 ["http://x.com/a/b", "http://y.com/c"], ["http://x.com/a/b", "https://x.com/c"], ["http://x.com/a/b", "http://x.com/a/b?q"],
 ["http://x.com/a/b/", "http://x.com/"], ["http://x.com/a", "http://x.com/a/"], ["http://x.com", "http://x.com/a"],
 ["http://x.com/a", "http://x.com"]].each do |from, to|
  p URI(from).route_to(to).to_s
end
p URI("http://x.com/a/b") - "http://x.com/a/"
p URI.join("http://a/b/", "//other.com/x", "y"), URI.join("http://a/b/", "mailto:z@x", "c")

__END__
http://y.com/b
http://x.com/a/b?q
http://x.com/a/b#f
http://x.com/a/b
http://x.com/a/c
http://x.com/c
other:x
http://x.com/a/
http://x.com/a/c
http://x.com/c
""
"./"
"../c"
"//y.com/c"
"https://x.com/c"
"?q"
"../../"
"a/"
"/a"
"../"
#<URI::Generic b>
#<URI::HTTP http://other.com/y>
#<URI::MailTo mailto:z@x>
