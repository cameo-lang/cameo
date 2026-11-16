// Package codes links an error about something cameo leaves out on purpose
// to the page that explains it. Each reason has a code, as
// COMPATIBILITY.md's table gives them (ivar, meta, mut, ...), and a page
// of cameo's site, compatibility/<code>.html, that says what cameo leaves
// out, why, and what to write instead. A message gives the page by a short
// link, which leads there.
package codes

// SiteURL is the address of cameo's site.
const SiteURL = "https://cameo-lang.github.io/cameo/"

// URL is the short link a message gives for code.
func URL(code string) string { return "https://15b.it/" + code }

// Page is the page of the site that explains code, where URL leads.
func Page(code string) string { return SiteURL + "compatibility/" + code + ".html" }

// Link ends msg with the link to the page of code:
// "global variables are not supported (see https://15b.it/glob)". It
// returns msg as it is when code is "".
func Link(msg, code string) string {
	if code == "" {
		return msg
	}
	return msg + " (see " + URL(code) + ")"
}
