# uri: testcase to demonstrate that URI refuses what Ruby refuses
#
# The scripts are plain Ruby, and the expected outputs are Ruby 3.4's, with
# uri 1.0.2.
#
# (Introduced in PR #263. expected output follows the __END__ separator.)
require "uri"
["http://x.com:-1", "ht tp://x", "http://x.com/€"].each do |s|
  URI.parse(s)
rescue URI::InvalidURIError => e
  puts e.message
end
begin; URI.parse(nil); rescue URI::InvalidURIError => e; puts e.message; end
begin; URI(nil); rescue ArgumentError => e; puts e.message; end
begin; URI.join("x", "y"); rescue URI::BadURIError => e; puts e.message; end
begin; URI::HTTP.build(host: "x com", path: "/p"); rescue URI::InvalidComponentError => e; puts e.message; end
begin; URI::HTTP.build(host: "x.com", path: "p"); rescue URI::InvalidComponentError => e; puts e.message; end
begin; URI::HTTP.build(host: "a b"); rescue URI::InvalidComponentError => e; puts e.message; end
begin; URI("http://x.com/a").select(:nope); rescue ArgumentError => e; puts e.message; end

__END__
bad URI (is not URI?): "http://x.com:-1"
bad URI (is not URI?): "ht tp://x"
URI must be ascii only "http://x.com/\u20AC"
bad URI (is not URI?): nil
bad argument (expected URI object or URI string)
both URI are relative
bad component(expected host component): x com
bad component(expected absolute path component): p
bad component(expected host component): a b
expected of components of URI::HTTP (scheme, userinfo, host, port, path, query, fragment)
