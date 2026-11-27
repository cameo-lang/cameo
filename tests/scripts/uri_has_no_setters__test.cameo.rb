# uri: testcase to demonstrate that URI has no setters
#
# A URI has no setters. Ruby changes the URI in place; cameo refuses the
# assignment, with itself, and the FTP and MailTo writers, and a different URI
# is parsed, joined, merged or built. The last line prints what Ruby prints.
#
# The scripts are plain Ruby, and the expected outputs are Ruby 3.4's, with
# uri 1.0.2.
#
# (Introduced in PR #263. expected output follows the __END__ separator.)
require "uri"
a = URI("http://x.com/a")
b = a
begin; a.path = "/b"; rescue NoMethodError => e; puts e.message; end
begin; a.with(query: "q"); rescue NoMethodError => e; puts e.message; end
begin; a.with { |x| { port: 80 } }; rescue NoMethodError => e; puts e.message; end
f = URI("ftp://h/pub/f;type=a")
begin; f.typecode = "i"; rescue NoMethodError => e; puts e.message; end
m = URI("mailto:a@b.c")
begin; m.to = "x@y.z"; rescue NoMethodError => e; puts e.message; end
begin; m.headers = [["subject", "hi"]]; rescue NoMethodError => e; puts e.message; end
p a == b, a + "/b", a.merge("?q=1"), URI.join(a, "c#f"), URI::HTTP.build(host: "x.com", port: 444, path: "/b", query: "q=1")

__END__
undefined method 'path=' for an instance of URI::HTTP: a URI is parsed, joined, merged or built, not changed
undefined method 'query=' for an instance of URI::HTTP: a URI is parsed, joined, merged or built, not changed
undefined method 'with' for an instance of URI::HTTP: a URI is parsed, joined, merged or built, not changed
undefined method 'typecode=' for an instance of URI::FTP: a URI is parsed, joined, merged or built, not changed
undefined method 'to=' for an instance of URI::MailTo: a URI is parsed, joined, merged or built, not changed
undefined method 'headers=' for an instance of URI::MailTo: a URI is parsed, joined, merged or built, not changed
true
#<URI::HTTP http://x.com/b>
#<URI::HTTP http://x.com/a?q=1>
#<URI::HTTP http://x.com/c#f>
#<URI::HTTP http://x.com:444/b?q=1>
