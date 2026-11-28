# uri: testcase to demonstrate that URI mail to and LDAP components
#
# The scripts are plain Ruby, and the expected outputs are Ruby 3.4's, with
# uri 1.0.2.
#
# (Introduced in PR #263. expected output follows the __END__ separator.)
require "uri"
m = URI("mailto:a@b.c,d@e.f?subject=hi&body=x")
p m.to, m.headers, m.select(:to, :headers), m.to_mailtext
p URI::MailTo.build(to: "joe@example.com", headers: [["subject", "hi"]]).to_s
l = URI("ldap://x/dc=ex?a,b?sub?(cn=x)")
p l.class, l.dn, l.attributes, l.scope, l.filter, l.extensions, l.port
p URI::Generic.new("http", nil, "x.com", 80, nil, "/p", nil, "q", "f").to_s
p URI.for("Http", nil, "x.com", nil, nil, "/p", nil, nil, nil).class, URI::Generic.ancestors.take(3)

__END__
"a@b.c,d@e.f"
[["subject", "hi"], ["body", "x"]]
["a@b.c,d@e.f", [["subject", "hi"], ["body", "x"]]]
"To: a@b.c,d@e.f\nSubject: hi\n\nx\n"
"mailto:joe@example.com?subject=hi"
URI::LDAP
"dc=ex"
"a,b"
"sub"
"(cn=x)"
nil
389
"http://x.com:80/p?q#f"
URI::HTTP
[URI::Generic, URI, Object]
