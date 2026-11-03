package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected values are Ruby 3.4's, with rexml 3.4.0, but where the
// probe says otherwise: a node-set holds each node once in document order
// (xpath15, xpath46), normalize-space works in a predicate (xpath37), a
// bad path is a ParseException (xpath_err), a ParseException's position
// is where cameo's parser stopped, a Text's to_s is not Ruby's live
// String (text_wrap), and the second-root error has no stack in it.
func TestREXMLMatchesRuby(t *testing.T) {
	out := runScript(t, `require "rexml/document"
require "stringio"
def t(label)
  r = yield
  puts "#{label}: #{r.inspect}"
rescue Exception => e
  puts "#{label}: #{e.class}: #{e.message}"
end
SRC = <<~XML
  <?xml version="1.0" encoding="UTF-8"?>
  <!-- a library -->
  <library name="town" xmlns:x="http://x.example">
    <book id="1" lang="en">
      <title>Dune</title>
      <author>Herbert</author>
      <price currency="USD">9.99</price>
    </book>
    <book id="2">
      <title>Solaris &amp; more</title>
      <author>Lem</author>
      <x:note>sf</x:note>
    </book>
    <empty/>
    <![CDATA[raw <stuff>]]>
    text at end
  </library>
XML
doc = REXML::Document.new(SRC)
t("root") { [doc.root.name, doc.root.attributes["name"], doc.root.attributes.size, doc.root.elements.size, doc.root.has_elements?, doc.root.has_text?] }
t("decl") { [doc.xml_decl.version, doc.xml_decl.encoding, doc.version, doc.encoding, doc.xml_decl.to_s, doc.doctype] }
t("children") { doc.children.map(&:class) }
t("rootch") { doc.root.children.map { |c| [c.class, c.to_s.strip[0, 20]] } }
t("el1") { doc.root.elements[1].name }
t("elpath") { doc.root.elements["book/title"].text }
t("elpath2") { doc.elements["library/book[2]/author"].text }
t("elpath3") { doc.elements["//book[@id='2']/title"].text }
t("elpath4") { doc.elements["//title"].text }
t("each") { r = []; doc.root.elements.each("book") { |b| r += [b.attributes["id"]] }; r }
t("each2") { r = []; doc.root.each_element("book/title") { |b| r += [b.text] }; r }
t("each3") { r = []; doc.root.each_element { |b| r += [b.name] }; r }
t("to_a") { doc.root.elements.to_a("book/author").map(&:text) }
t("get_el") { doc.root.get_elements("book").size }
t("xpath1") { REXML::XPath.first(doc, "//title").text }
t("xpath2") { REXML::XPath.match(doc, "//book/title").map(&:text) }
t("xpath3") { REXML::XPath.match(doc, "//book[@lang]/title/text()").map(&:to_s) }
t("xpath4") { REXML::XPath.match(doc, "//book/@id").map(&:value) }
t("xpath5") { REXML::XPath.first(doc, "//price/@currency").to_s }
t("xpath6") { REXML::XPath.match(doc, "/library/book[1]/*").map(&:name) }
t("xpath7") { REXML::XPath.match(doc, "//book[title='Dune']/author/text()").map(&:value) }
t("xpath8") { REXML::XPath.match(doc, "//book[last()]/@id").map(&:value) }
t("xpath9") { REXML::XPath.match(doc, "//*[@currency]").map(&:name) }
t("xpath10") { REXML::XPath.match(doc, "//book[author]/title").map(&:text) }
t("xpath11") { r = []; REXML::XPath.each(doc, "//author") { |a| r += [a.text] }; r }
t("xpath12") { REXML::XPath.match(doc, "//x:note").map(&:text) }
t("xpath13") { REXML::XPath.match(doc, "//note").map(&:text) }
t("xpath14") { REXML::XPath.match(doc, "//book/title | //book/author").map(&:text) }
t("xpath15") { REXML::XPath.match(doc, "//book/..").map(&:name) }
t("xpath16") { REXML::XPath.match(doc, "//book/.").map { |e| e.attributes["id"] } }
t("xpath17") { REXML::XPath.match(doc, "//book[2]/node()").map(&:class) }
t("xpath18") { REXML::XPath.match(doc, "//book[position()=2]/title").map(&:text) }
t("xpath19") { REXML::XPath.match(doc, "//book[@id='1' and @lang='en']/title").map(&:text) }
t("xpath20") { REXML::XPath.match(doc, "//book[@id!='1']/title").map(&:text) }
t("xpath21") { REXML::XPath.match(doc, "//book[@id>1]/title").map(&:text) }
t("xpath22") { REXML::XPath.match(doc, "count(//book)") }
t("xpath23") { REXML::XPath.first(doc, "string(//title)") }
t("xpath24") { REXML::XPath.match(doc, "//book[contains(title, 'Sol')]/author/text()").map(&:to_s) }
t("xpath25") { REXML::XPath.match(doc, "//book[starts-with(@id, '1')]/author/text()").map(&:to_s) }
t("xpath26") { REXML::XPath.match(doc, "//title[not(@x)]").size }
t("xpath27") { REXML::XPath.first(doc, "/library/@name").value }
t("xpath28") { REXML::XPath.match(doc, "library/book").size }
t("xpath29") { REXML::XPath.match(doc.root, "book").size }
t("xpath30") { REXML::XPath.match(doc.root, "./book").size }
t("xpath31") { REXML::XPath.match(doc.root, "/book").size }
t("xpath32") { REXML::XPath.match(doc, "//comment()").map(&:to_s) }
t("xpath33") { REXML::XPath.match(doc, "//book/title/text()").map(&:class) }
t("xpath34") { REXML::XPath.first(doc, "//bogus") }
t("xpath35") { REXML::XPath.match(doc, "//book[@id=2]/title/text()").map(&:to_s) }
t("xpath36") { REXML::XPath.match(doc, "//book[not(@lang)]/@id").map(&:value) }
t("xpath37") { REXML::XPath.match(doc, "//book[normalize-space(author)='Lem']/@id").map(&:value) }
t("xpath38") { REXML::XPath.match(doc, "//text()").map(&:to_s).map(&:strip).reject(&:empty?) }
t("xpath39") { REXML::XPath.match(doc, "//price[. > 5]").map(&:text) }
t("xpath40") { REXML::XPath.match(doc, "//price[number(.) > 5]").map(&:text) }
t("xpath41") { REXML::XPath.match(doc, "//book[@id = '1' or @id = '2']").size }
t("xpath42") { REXML::XPath.match(doc, "//*[local-name()='note']").map(&:name) }
t("xpath43") { REXML::XPath.match(doc, "//*[name()='x:note']").map(&:expanded_name) }
t("xpath44") { REXML::XPath.match(doc, "//book[string-length(title) > 4]/@id").map(&:value) }
t("xpath45") { REXML::XPath.first(doc, "//book[1]/following-sibling::book/@id").value }
t("xpath46") { REXML::XPath.match(doc, "//title/ancestor::*").map(&:name) }
t("xpath47") { REXML::XPath.match(doc, "//price/parent::book/@id").map(&:value) }
t("xpath48") { REXML::XPath.match(doc, "/library/child::book").size }
t("xpath49") { REXML::XPath.match(doc, "//book/descendant::text()").size }
t("xpath50") { REXML::XPath.match(doc, "//book/attribute::id").map(&:value) }
t("xpath51") { REXML::XPath.match(doc, "//book[2]/preceding-sibling::*").map(&:name) }
t("xpath52") { REXML::XPath.match(doc, "//book[1]/self::book").size }
t("text") { [doc.root.elements["book/title"].text, doc.root.elements["book[2]/title"].text, doc.root.elements["empty"].text, doc.root.text.inspect, doc.root.texts.map(&:to_s)] }
t("text2") { [doc.root.elements["book[2]/title"].get_text.to_s, doc.root.elements["book[2]/title"].get_text.value, doc.root.elements["book[2]/title"].texts.size] }
t("attr") { b = doc.root.elements["book"]; [b.attributes["id"], b["id"], b.attribute("lang").value, b.attribute("lang").name, b.attribute("lang").to_s, b.attributes.to_a.map(&:to_s), b.attributes.keys, b.attributes.values.map(&:class), b.attribute("zz"), b.attributes["zz"]] }
t("attr2") { r = []; doc.root.elements["book"].attributes.each { |k, v| r += [[k, v]] }; r }
t("attr3") { r = []; doc.root.elements["book"].attributes.each_attribute { |a| r += [[a.name, a.value, a.class]] }; r }
t("ns") { n = doc.root.elements["book[2]/x:note"]; [n.name, n.expanded_name, n.prefix, n.namespace, doc.root.namespaces, doc.root.elements["book[2]"].namespace("x")] }
t("parent") { [doc.root.elements["book/title"].parent.name, doc.root.parent.class, doc.root.elements["book"].root.name, doc.root.elements["book"].document.class, doc.root.elements["book/title"].root_node.class] }
t("sib") { b = doc.root.elements["book"]; [b.next_element.name, b.next_element.next_element.name, b.previous_element, b.next_sibling.class, b.index_in_parent, doc.root.elements["empty"].next_element] }
t("to_s") { doc.root.elements["book[2]"].to_s }
t("to_s2") { doc.root.elements["book/price"].to_s }
t("to_s3") { doc.root.elements["empty"].to_s }
t("to_s4") { doc.to_s }
t("inspect") { [doc.root.inspect, doc.root.elements["book"].inspect, doc.root.elements["book/title"].inspect, doc.root.elements["empty"].inspect, doc.inspect, doc.root.elements["book/title"].get_text.inspect, doc.root.elements["book"].attribute("id").inspect] }
t("write") { s = StringIO.new; doc.write(s); s.string }
t("write2") { s = StringIO.new; doc.write(s, 2); s.string }
t("write3") { s = StringIO.new; doc.write(output: s, indent: 1); s.string }
t("pretty") { f = REXML::Formatters::Pretty.new(2); f.compact = true; s = StringIO.new; f.write(doc.root.elements["book"], s); s.string }
t("pretty2") { f = REXML::Formatters::Pretty.new; s = StringIO.new; f.write(doc.root.elements["book"], s); s.string }
t("default") { f = REXML::Formatters::Default.new; s = StringIO.new; f.write(doc.root.elements["book"], s); s.string }
t("cdata") { c = doc.root.children.find { |c| c.is_a?(REXML::CData) }; [c.to_s, c.value, c.class, doc.root.cdatas.size] }
t("comment") { c = doc.children.find { |c| c.is_a?(REXML::Comment) }; [c.to_s, c.string, c.class] }
t("build") do
  d = REXML::Document.new
  d << REXML::XMLDecl.new("1.0", "UTF-8")
  r = d.add_element("root", {"a" => "1"})
  c = r.add_element("child")
  c.add_text("hi & <bye>")
  c.add_attribute("k", "v\"q")
  r.add_element(REXML::Element.new("other"))
  r.add_text(REXML::Text.new("x"))
  r.add(REXML::Comment.new("note"))
  r << REXML::CData.new("raw <x>")
  [d.to_s, r.elements.size, c.text, c.attributes["k"], d.root.name]
end
t("build2") { e = REXML::Element.new("e"); e.text = "a < b"; e.add_attribute("x", "1"); e.add_attribute("y", "2"); e.add_attributes({"z" => "3"}); [e.to_s, e.text, e.attributes.size] }
t("build3") { e = REXML::Element.new("e"); e.add_element("a"); e.add_element("b"); e.add_element("a"); e.delete_element("a"); e.elements.delete("b"); [e.to_s, e.elements.size] }
t("build4") { e = REXML::Element.new("e"); e.add_attribute("x", "1"); e.delete_attribute("x"); [e.to_s, e.attributes.size] }
t("build5") { e = REXML::Element.new("e"); e.text = "a"; e.text = "b"; [e.to_s, e.texts.size] }
t("build6") { e = REXML::Element.new("e"); e.text = nil; e.add_text("a"); e.add_text("b"); [e.to_s, e.texts.size, e.text] }
t("textval") { [REXML::Text.new("a &amp; b").to_s, REXML::Text.new("a &amp; b").value, REXML::Text.new("a & b").to_s, REXML::Text.new("a & b", false, nil, true).to_s, REXML::Text.new("<").value, REXML::Text.normalize("a<b&c\"d"), REXML::Text.unnormalize("a&lt;b&amp;c&#65;&#x42;")] }
t("err1") { REXML::Document.new("<a><b></a>") }
t("err2") { REXML::Document.new("<a>") }
t("err3") { REXML::Document.new("<a b='1' b='2'/>") }
t("err4") { REXML::Document.new("<a><b/><c/>") }
t("err5") { REXML::Document.new("text") }
t("err6") { REXML::Document.new("<a>&bogus;</a>").root.text }
t("err7") { REXML::Document.new("<a x=1/>") }
t("err8") { REXML::Document.new("<a></b>") }
t("err9") { REXML::Document.new("<1a/>") }
t("err10") { REXML::Document.new("<a/><b/>") }
t("err11") { begin; REXML::Document.new("<a><b></a>"); rescue REXML::ParseException => e; [e.class.superclass, e.line, e.position, e.context]; end }
t("err12") { REXML::Document.new("").root }
t("err13") { REXML::Document.new("   ").root }
t("ent") { d = REXML::Document.new("<a>&lt;&gt;&amp;&quot;&apos;&#65;&#x42; &#169;</a>"); [d.root.text, d.root.get_text.to_s, d.root.to_s] }
t("ws") { d = REXML::Document.new("<a>\n  <b>  x  </b>\n</a>"); [d.root.elements["b"].text, d.root.text, d.root.to_s, d.root.children.size] }
t("ws2") { d = REXML::Document.new("<a>\n  <b>  x  </b>\n</a>", ignore_whitespace_nodes: :all); [d.root.children.size, d.root.to_s] }
t("ws3") { d = REXML::Document.new("<a>\n  <b>  x  </b>\n</a>", compress_whitespace: :all); [d.root.elements["b"].text, d.root.to_s] }
t("attrq") { d = REXML::Document.new(%(<a x="it's" y='say "hi"' z="&lt;&amp;"/>)); [d.root.attributes["x"], d.root.attributes["y"], d.root.attributes["z"], d.root.to_s, d.root.attribute("z").to_s, d.root.attribute("y").to_string] }
t("deep") { d = REXML::Document.new("<a><b><c><d>x</d></c></b></a>"); [d.elements["a/b/c/d"].text, d.elements["//d"].text, d.root.elements["b"].elements["c"].elements["d"].text, d.elements["a"].elements.to_a.map(&:name)] }
t("mixed") { d = REXML::Document.new("<p>one <b>two</b> three</p>"); [d.root.text, d.root.texts.map(&:to_s), d.root.children.map(&:class), d.root.elements.size, d.root.to_s, d.root.children.map(&:to_s)] }
t("docel") { [doc.root.document == doc, doc.root.elements[1].parent == doc.root, doc.name, doc.root.elements[1].class, doc.elements.size, doc.elements[1].name, doc.root.size, doc.root.elements["book"].size, doc.root.node_type, doc.node_type, doc.root.texts.first.node_type, doc.root.attributes.first.class] }
t("deep_clone") { e = doc.root.elements["book"].deep_clone; [e.parent, e.to_s == doc.root.elements["book"].to_s] }
t("clone") { e = doc.root.elements["book"].clone; [e.to_s, e.attributes.size] }
t("docstr") { REXML::Document.new("<a>x</a>").to_s }
t("docstr2") { REXML::Document.new("<?xml version='1.0'?><a>x</a>").to_s }
t("docstr3") { d = REXML::Document.new("<a>x</a>"); d << REXML::XMLDecl.new; d.to_s }
t("xmldecl") { [REXML::XMLDecl.new.to_s, REXML::XMLDecl.new("1.1", "UTF-8", "yes").to_s, REXML::XMLDecl.default.to_s, REXML::XMLDecl.new.version, REXML::XMLDecl.new.encoding, REXML::XMLDecl.new.standalone] }
t("empty_el") { [REXML::Element.new("a").to_s, REXML::Element.new("a").text, REXML::Element.new("a").elements.size, REXML::Element.new("a").has_elements?, REXML::Element.new("a").has_text?, REXML::Element.new("a").parent, REXML::Element.new("a").document] }
t("dup_attr") { REXML::Document.new("<a x='1' y='2' x='3'/>") }
t("inst") { d = REXML::Document.new("<?xml version='1.0'?><?pi data?><a/>"); [d.children.map(&:class), d.children[1].target, d.children[1].content, d.children[1].to_s] }
t("doctype") { d = REXML::Document.new("<!DOCTYPE a><a/>"); [d.doctype.class, d.doctype.name, d.doctype.to_s, d.to_s] }
t("doctype2") { d = REXML::Document.new('<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1.dtd"><html/>'); [d.doctype.name, d.doctype.external_id, d.doctype.public, d.doctype.system, d.to_s] }
t("stringsrc") { REXML::Document.new(REXML::Source.new("<a/>")).root.name }
t("add_el_hash") { e = REXML::Element.new("a"); e.add_element("b", "x" => "1", "y" => "2"); e.to_s }
t("root_set") { d = REXML::Document.new; d.add_element("r"); [d.root.name, d.to_s] }
t("el_eq") { d = REXML::Document.new("<a><b/><b/></a>"); [d.root.elements[1] == d.root.elements[2], d.root.elements[1] == d.root.elements[1], d.root.elements[1].equal?(d.root.elements[1])] }
t("elements_ix") { d = REXML::Document.new("<a><b/><c/><b/></a>"); [d.root.elements[0], d.root.elements[1].name, d.root.elements[3].name, d.root.elements[4], d.root.elements["b", 2].class, d.root.elements["c"].name, d.root.elements[1, "b"].name, d.root.elements[2, "b"].name] }
t("elements_set") { d = REXML::Document.new("<a><b/><c/></a>"); d.root.replace_child(d.root.elements[1], REXML::Element.new("z")); d.root.replace_child(d.root.elements["c"], REXML::Element.new("y")); d.root.to_s }
t("elements_ins") { d = REXML::Document.new("<a><b/></a>"); d.root.elements << REXML::Element.new("c"); d.root.elements.add("d"); d.root.insert_before("c", REXML::Element.new("q")); d.root.insert_after(d.root.elements["d"], REXML::Element.new("r")); d.root.to_s }
t("elements_each_idx") { d = REXML::Document.new("<a><b/><c/></a>"); r = []; d.root.elements.each_with_index { |e, i| r += [[e.name, i]] }; [r, d.root.elements.map(&:name), d.root.elements.collect { |e| e.name }, d.root.elements.to_a.class, d.root.elements.empty?, d.root.elements.index(d.root.elements["c"])] }
t("whitespace_text") { d = REXML::Document.new("<a> </a>"); [d.root.text, d.root.has_text?, d.root.children.size] }
t("each_recursive") { r = []; doc.root.each_recursive { |e| r += [e.name] }; r }
t("find_first") { doc.root.find_first_recursive { |e| e.name == "author" }.text }
t("text_wrap") { d = REXML::Document.new("<a>x</a>"); tx = d.root.get_text; [tx.parent.name, tx.to_s, tx.value, (tx << " y").to_s, d.root.text, (tx.value = "z"), d.root.text, tx.empty?, REXML::Text.new("").empty?] }
t("raw") { d = REXML::Document.new("<a>&amp;</a>", raw: :all); [d.root.text, d.root.get_text.to_s, d.root.get_text.raw] }
t("text_attrs") { doc.root.elements["book/title"].get_text.raw }
t("element_new_parent") { p = REXML::Element.new("p"); c = REXML::Element.new("c", p); [p.to_s, c.parent.name] }
t("element_new_ctx") { e = REXML::Element.new("e", nil, {raw: :all}); e.add_text("&amp;"); e.to_s }
t("cdata_text") { d = REXML::Document.new("<a><![CDATA[x < y]]></a>"); [d.root.text, d.root.cdatas.map(&:to_s), d.root.get_text.class, d.root.to_s] }
t("entities") { d = REXML::Document.new("<!DOCTYPE a [<!ENTITY foo 'bar'>]><a>&foo;</a>"); [d.root.text, d.root.get_text.to_s, d.doctype.entities.keys.size, d.doctype.entity("foo")] }
t("unicode") { d = REXML::Document.new("<a>héllo ✓</a>"); [d.root.text, d.root.to_s, d.root.text.encoding.to_s] }
t("attr_escape") { e = REXML::Element.new("e"); e.add_attribute("a", "x < y & \"z\" 'w'"); [e.to_s, e.attributes["a"]] }
t("text_escape") { e = REXML::Element.new("e"); e.text = "x < y > z & \"q\" 'r' ]]>"; [e.to_s, e.text] }
t("write_decl") { d = REXML::Document.new("<?xml version='1.0' encoding='utf-8'?>\n<a>\n<b/>\n</a>"); s = StringIO.new; d.write(s); [s.string, d.xml_decl.encoding] }
t("pretty_doc") { d = REXML::Document.new("<a><b>text</b><c><d/><d>e</d></c></a>"); s = StringIO.new; d.write(s, 2); s.string }
t("pretty_doc_c") { d = REXML::Document.new("<a><b>text</b><c><d/><d>e</d></c></a>"); f = REXML::Formatters::Pretty.new(2); f.compact = true; s = StringIO.new; f.write(d, s); s.string }
t("pretty_mixed") { d = REXML::Document.new("<p>one <b>two</b> three</p>"); s = StringIO.new; d.write(s, 2); s.string }
t("pretty_long") { d = REXML::Document.new("<a><b>" + ("word " * 30).strip + "</b></a>"); f = REXML::Formatters::Pretty.new(2); f.compact = true; s = StringIO.new; f.write(d, s); s.string }
t("pretty_long2") { d = REXML::Document.new("<a><b>" + ("word " * 30).strip + "</b></a>"); f = REXML::Formatters::Pretty.new(2); f.compact = true; f.width = 40; s = StringIO.new; f.write(d, s); s.string }
t("write_trans") { d = REXML::Document.new("<a>x</a>"); s = StringIO.new; d.write(s, -1, true); s.string }
t("write_ie") { d = REXML::Document.new("<a><b/></a>"); s = StringIO.new; d.write(s, -1, false, true); s.string }
t("stringify") { [doc.root.elements["book"].to_s.class, doc.root.elements["book/title"].get_text.to_s, "#{doc.root.elements['book/title']}", doc.root.elements["book/title"].text.class] }
t("xpath_vars") { REXML::XPath.match(doc, "//book[@id=$id]/title", {}, {"id" => "2"}).map(&:text) }
t("xpath_ns") { REXML::XPath.match(doc, "//y:note", {"y" => "http://x.example"}).map(&:text) }
t("xpath_err") { REXML::XPath.match(doc, "//book[") }
t("xpath_err2") { REXML::XPath.match(doc, "//book[@id=") }
t("xpath_err3") { REXML::XPath.match(doc, "bogus()") }
t("xpath_num") { [REXML::XPath.first(doc, "count(//book) + 1"), REXML::XPath.first(doc, "sum(//price)"), REXML::XPath.first(doc, "string-length('abc')"), REXML::XPath.first(doc, "concat('a', 'b', 'c')"), REXML::XPath.first(doc, "substring('hello', 2, 3)"), REXML::XPath.first(doc, "translate('abc', 'ab', 'AB')"), REXML::XPath.first(doc, "true()"), REXML::XPath.first(doc, "false()"), REXML::XPath.first(doc, "not(true())"), REXML::XPath.first(doc, "boolean(//bogus)"), REXML::XPath.first(doc, "floor(2.7)"), REXML::XPath.first(doc, "ceiling(2.1)"), REXML::XPath.first(doc, "round(2.5)"), REXML::XPath.first(doc, "number('3')"), REXML::XPath.first(doc, "string(123)"), REXML::XPath.first(doc, "1 div 2"), REXML::XPath.first(doc, "7 mod 3"), REXML::XPath.first(doc, "2 * 3 - 1"), REXML::XPath.first(doc, "-1"), REXML::XPath.first(doc, "name(/library)"), REXML::XPath.first(doc, "normalize-space('  a   b ')"), REXML::XPath.first(doc, "substring-before('a-b', '-')"), REXML::XPath.first(doc, "substring-after('a-b', '-')"), REXML::XPath.first(doc, "string(//book[1]/@id)"), REXML::XPath.first(doc, "local-name(//x:note)"), REXML::XPath.first(doc, "namespace-uri(//x:note)"), REXML::XPath.first(doc, "'lit'"), REXML::XPath.first(doc, "3"), REXML::XPath.first(doc, "1 = 1"), REXML::XPath.first(doc, "1 < 2"), REXML::XPath.first(doc, "//book[1]/@id = 1"), REXML::XPath.first(doc, "//title = 'Dune'"), REXML::XPath.first(doc, "1 div 0"), REXML::XPath.first(doc, "number('x')")] }
`)
	assert.Equals(t, out, `root: ["library", "town", 2, 3, true, true]
decl: ["1.0", "UTF-8", "1.0", "UTF-8", "<?xml version='1.0' encoding='UTF-8'?>", nil]
children: [REXML::XMLDecl, REXML::Text, REXML::Comment, REXML::Text, REXML::Element]
rootch: [[REXML::Text, ""], [REXML::Element, "<book id='1' lang='e"], [REXML::Text, ""], [REXML::Element, "<book id='2'>\n    <t"], [REXML::Text, ""], [REXML::Element, "<empty/>"], [REXML::Text, ""], [REXML::CData, "raw <stuff>"], [REXML::Text, "text at end"]]
el1: "book"
elpath: "Dune"
elpath2: "Lem"
elpath3: "Solaris & more"
elpath4: "Dune"
each: ["1", "2"]
each2: ["Dune", "Solaris & more"]
each3: ["book", "book", "empty"]
to_a: ["Herbert", "Lem"]
get_el: 2
xpath1: "Dune"
xpath2: ["Dune", "Solaris & more"]
xpath3: ["Dune"]
xpath4: ["1", "2"]
xpath5: "USD"
xpath6: ["title", "author", "price"]
xpath7: ["Herbert"]
xpath8: ["2"]
xpath9: ["price"]
xpath10: ["Dune", "Solaris & more"]
xpath11: ["Herbert", "Lem"]
xpath12: ["sf"]
xpath13: []
xpath14: ["Dune", "Solaris & more", "Herbert", "Lem"]
xpath15: ["library"]
xpath16: ["1", "2"]
xpath17: [REXML::Text, REXML::Element, REXML::Text, REXML::Element, REXML::Text, REXML::Element, REXML::Text]
xpath18: ["Solaris & more"]
xpath19: ["Dune"]
xpath20: ["Solaris & more"]
xpath21: ["Solaris & more"]
xpath22: [2]
xpath23: "Dune"
xpath24: ["Lem"]
xpath25: ["Herbert"]
xpath26: 2
xpath27: "town"
xpath28: 2
xpath29: 2
xpath30: 2
xpath31: 0
xpath32: [" a library "]
xpath33: [REXML::Text, REXML::Text]
xpath34: nil
xpath35: ["Solaris &amp; more"]
xpath36: ["2"]
xpath37: ["2"]
xpath38: ["Dune", "Herbert", "9.99", "Solaris &amp; more", "Lem", "sf", "raw <stuff>", "text at end"]
xpath39: ["9.99"]
xpath40: ["9.99"]
xpath41: 2
xpath42: ["note"]
xpath43: ["x:note"]
xpath44: ["2"]
xpath45: "2"
xpath46: ["library", "book", "book"]
xpath47: ["1"]
xpath48: 2
xpath49: 14
xpath50: ["1", "2"]
xpath51: ["book"]
xpath52: 1
text: ["Dune", "Solaris & more", nil, "\"\\n  \"", ["\n  ", "\n  ", "\n  ", "\n  ", "raw <stuff>", "\n  text at end\n"]]
text2: ["Solaris &amp; more", "Solaris & more", 1]
attr: ["1", "1", "en", "lang", "en", ["1", "en"], ["id", "lang"], [REXML::Attribute, REXML::Attribute], nil, nil]
attr2: [["id", "1"], ["lang", "en"]]
attr3: [["id", "1", REXML::Attribute], ["lang", "en", REXML::Attribute]]
ns: ["note", "x:note", "x", "http://x.example", {"x" => "http://x.example"}, "http://x.example"]
parent: ["book", REXML::Document, "library", REXML::Document, REXML::Document]
sib: ["book", "empty", nil, REXML::Text, 2, nil]
to_s: "<book id='2'>\n    <title>Solaris &amp; more</title>\n    <author>Lem</author>\n    <x:note>sf</x:note>\n  </book>"
to_s2: "<price currency='USD'>9.99</price>"
to_s3: "<empty/>"
to_s4: "<?xml version='1.0' encoding='UTF-8'?>\n<!-- a library -->\n<library name='town' xmlns:x='http://x.example'>\n  <book id='1' lang='en'>\n    <title>Dune</title>\n    <author>Herbert</author>\n    <price currency='USD'>9.99</price>\n  </book>\n  <book id='2'>\n    <title>Solaris &amp; more</title>\n    <author>Lem</author>\n    <x:note>sf</x:note>\n  </book>\n  <empty/>\n  <![CDATA[raw <stuff>]]>\n  text at end\n</library>"
inspect: ["<library name='town' xmlns:x='http://x.example'> ... </>", "<book id='1' lang='en'> ... </>", "<title> ... </>", "<empty/>", "<UNDEFINED> ... </>", "\"Dune\"", "id='1'"]
write: "<?xml version='1.0' encoding='UTF-8'?>\n<!-- a library -->\n<library name='town' xmlns:x='http://x.example'>\n  <book id='1' lang='en'>\n    <title>Dune</title>\n    <author>Herbert</author>\n    <price currency='USD'>9.99</price>\n  </book>\n  <book id='2'>\n    <title>Solaris &amp; more</title>\n    <author>Lem</author>\n    <x:note>sf</x:note>\n  </book>\n  <empty/>\n  <![CDATA[raw <stuff>]]>\n  text at end\n</library>"
write2: "<?xml version='1.0' encoding='UTF-8'?>\n<!-- a library -->\n<library name='town' xmlns:x='http://x.example'>\n  <book id='1' lang='en'>\n    <title>\n      Dune\n    </title>\n    <author>\n      Herbert\n    </author>\n    <price currency='USD'>\n      9.99\n    </price>\n  </book>\n  <book id='2'>\n    <title>\n      Solaris &amp; more\n    </title>\n    <author>\n      Lem\n    </author>\n    <x:note>\n      sf\n    </x:note>\n  </book>\n  <empty/>\n  <![CDATA[raw <stuff>]]>\n   text at end \n</library>"
write3: "<?xml version='1.0' encoding='UTF-8'?>\n<!-- a library -->\n<library name='town' xmlns:x='http://x.example'>\n <book id='1' lang='en'>\n  <title>\n   Dune\n  </title>\n  <author>\n   Herbert\n  </author>\n  <price currency='USD'>\n   9.99\n  </price>\n </book>\n <book id='2'>\n  <title>\n   Solaris &amp; more\n  </title>\n  <author>\n   Lem\n  </author>\n  <x:note>\n   sf\n  </x:note>\n </book>\n <empty/>\n <![CDATA[raw <stuff>]]>\n  text at end \n</library>"
pretty: "<book id='1' lang='en'>\n  <title>Dune</title>\n  <author>Herbert</author>\n  <price currency='USD'>9.99</price>\n</book>"
pretty2: "<book id='1' lang='en'>\n  <title>\n    Dune\n  </title>\n  <author>\n    Herbert\n  </author>\n  <price currency='USD'>\n    9.99\n  </price>\n</book>"
default: "<book id='1' lang='en'>\n    <title>Dune</title>\n    <author>Herbert</author>\n    <price currency='USD'>9.99</price>\n  </book>"
cdata: ["raw <stuff>", "raw <stuff>", REXML::CData, 1]
comment: [" a library ", " a library ", REXML::Comment]
build: ["<?xml version='1.0' encoding='UTF-8'?><root a='1'><child k='v&quot;q'>hi &amp; &lt;bye&gt;</child><other/>x<!--note--><![CDATA[raw <x>]]></root>", 2, "hi & <bye>", "v\"q", "root"]
build2: ["<e x='1' y='2' z='3'>a &lt; b</e>", "a < b", 3]
build3: ["<e><a/></e>", 1]
build4: ["<e/>", 0]
build5: ["<e>b</e>", 1]
build6: ["<e>ab</e>", 1, "ab"]
textval: RuntimeError: Illegal character "&" in raw string "a & b"
err1: REXML::ParseException: Missing end tag for 'b' (got 'a')
Line: 1
Position: 10
Last 80 unconsumed characters:
err2: REXML::ParseException: Missing end tag for '/a'
Line: 1
Position: 3
Last 80 unconsumed characters:
err3: REXML::ParseException: Duplicate attribute "b"
Line: 1
Position: 14
Last 80 unconsumed characters:
/>
err4: REXML::ParseException: Missing end tag for '/a'
Line: 1
Position: 11
Last 80 unconsumed characters:
err5: REXML::ParseException: Malformed XML: Content at the start of the document (got 'text')
Line: 1
Position: 4
Last 80 unconsumed characters:
err6: "&bogus;"
err7: REXML::ParseException: Missing attribute value start quote: <x>
Line: 1
Position: 5
Last 80 unconsumed characters:
1/>
err8: REXML::ParseException: Missing end tag for 'a' (got 'b')
Line: 1
Position: 7
Last 80 unconsumed characters:
err9: REXML::ParseException: malformed XML: missing tag start
Line: 1
Position: 0
Last 80 unconsumed characters:
<1a/>
err10: REXML::ParseException: attempted adding second root element to document
Line: 1
Position: 8
Last 80 unconsumed characters:
err11: [RuntimeError, 1, 10, [10, 1, 1]]
err12: nil
err13: nil
ent: ["<>&\"'AB ©", "&lt;&gt;&amp;&quot;&apos;&#65;&#x42; &#169;", "<a>&lt;&gt;&amp;&quot;&apos;&#65;&#x42; &#169;</a>"]
ws: ["  x  ", "\n  ", "<a>\n  <b>  x  </b>\n</a>", 3]
ws2: [1, "<a><b>  x  </b></a>"]
ws3: [" x ", "<a>\n <b> x </b>\n</a>"]
attrq: ["it's", "say \"hi\"", "<&", "<a x='it&apos;s' y='say \"hi\"' z='&lt;&amp;'/>", "&lt;&amp;", "y='say \"hi\"'"]
deep: ["x", "x", "x", ["b"]]
mixed: ["one ", ["one ", " three"], [REXML::Text, REXML::Element, REXML::Text], 1, "<p>one <b>two</b> three</p>", ["one ", "<b>two</b>", " three"]]
docel: [true, true, "", REXML::Element, 1, "library", 9, 7, :element, :document, :text, Array]
deep_clone: [nil, true]
clone: ["<book id='1' lang='en'/>", 2]
docstr: "<a>x</a>"
docstr2: "<?xml version='1.0'?><a>x</a>"
docstr3: "<?xml version='1.0'?><a>x</a>"
xmldecl: ["<?xml version='1.0'?>", "<?xml version='1.1' encoding='UTF-8' standalone='yes'?>", "", "1.0", "UTF-8", nil]
empty_el: ["<a/>", nil, 0, false, false, nil, nil]
dup_attr: REXML::ParseException: Duplicate attribute "x"
Line: 1
Position: 20
Last 80 unconsumed characters:
/>
inst: [[REXML::XMLDecl, REXML::Instruction, REXML::Element], "pi", "data", "<?pi data?>"]
doctype: [REXML::DocType, "a", "<!DOCTYPE a>", "<!DOCTYPE a><a/>"]
doctype2: ["html", "PUBLIC", "-//W3C//DTD XHTML 1.0//EN", "http://www.w3.org/TR/xhtml1/DTD/xhtml1.dtd", "<!DOCTYPE html PUBLIC \"-//W3C//DTD XHTML 1.0//EN\" \"http://www.w3.org/TR/xhtml1/DTD/xhtml1.dtd\"><html/>"]
stringsrc: "a"
add_el_hash: "<a><b x='1' y='2'/></a>"
root_set: ["r", "<r/>"]
el_eq: [false, true, true]
elements_ix: RuntimeError: index (0) must be >= 1
elements_set: "<a><z/><y/></a>"
elements_ins: "<a><b/><q/><c/><d/><r/></a>"
elements_each_idx: [[["b", 0], ["c", 1]], ["b", "c"], ["b", "c"], Array, false, 2]
whitespace_text: [" ", true, 1]
each_recursive: ["book", "title", "author", "price", "book", "title", "author", "note", "empty"]
find_first: "Herbert"
text_wrap: ["a", "x", "x", "x y", "x y", "z", "z", false, true]
raw: ["&", "&amp;", true]
text_attrs: true
element_new_parent: ["<p><c/></p>", "p"]
element_new_ctx: "<e>&amp;</e>"
cdata_text: ["x < y", ["x < y"], REXML::CData, "<a><![CDATA[x < y]]></a>"]
entities: ["bar", "&foo;", 5, "bar"]
unicode: ["héllo ✓", "<a>héllo ✓</a>", "UTF-8"]
attr_escape: ["<e a='x &lt; y &amp; &quot;z&quot; &apos;w&apos;'/>", "x < y & \"z\" 'w'"]
text_escape: ["<e>x &lt; y &gt; z &amp; &quot;q&quot; &apos;r&apos; ]]&gt;</e>", "x < y > z & \"q\" 'r' ]]>"]
write_decl: ["<?xml version='1.0' encoding='UTF-8'?>\n<a>\n<b/>\n</a>", "UTF-8"]
pretty_doc: "<a>\n  <b>\n    text\n  </b>\n  <c>\n    <d/>\n    <d>\n      e\n    </d>\n  </c>\n</a>"
pretty_doc_c: "<a>\n  <b>text</b>\n  <c>\n    <d/>\n    <d>e</d>\n  </c>\n</a>"
pretty_mixed: "<p>\n  one \n  <b>\n    two\n  </b>\n   three\n</p>"
pretty_long: "<a>\n  <b>\n    word word word word word word word word word word word word word word word\n    word word word word word word word word word word word word word word word\n  </b>\n</a>"
pretty_long2: "<a>\n  <b>\n    word word word word word word word\n    word word word word word word word\n    word word word word word word word\n    word word word word word word word\n    word word\n  </b>\n</a>"
write_trans: "<a>x</a>"
write_ie: "<a><b /></a>"
stringify: [String, "Dune", "<title>Dune</title>", String]
xpath_vars: ["Solaris & more"]
xpath_ns: ["sf"]
xpath_err: REXML::ParseException: Invalid XPath "//book[": expected a node test but got the end
xpath_err2: REXML::ParseException: Invalid XPath "//book[@id=": expected a node test but got the end
xpath_err3: REXML::ParseException: Invalid XPath "bogus()": unknown function bogus()
xpath_num: [3.0, 9.99, 3, "abc", "ell", "ABc", true, false, false, false, 2, 3, 3, 3.0, "123", 0.5, 1.0, 5.0, -1.0, "library", "a b", "a", "b", "1", "note", "http://x.example", "lit", 3, true, true, true, true, Infinity, NaN]
`)
}

func TestREXMLWritesToStandardOutput(t *testing.T) {
	out := runScript(t, `require "rexml/document"
doc = REXML::Document.new("<a><b>x</b><c/></a>")
doc.write
puts
doc.write($stdout, 2)
puts
doc.root.elements["b"].write(indent: 1)
puts
f = REXML::Formatters::Pretty.new(1)
f.compact = true
f.write(doc.root, $stdout)
puts
p doc.root.elements["b"] == doc.root.elements["b"], doc.root.elements["b"] == doc.root.elements["c"], doc.root.elements["b"].equal?(doc.root.elements[1])
p doc.root.elements["b"].get_text == REXML::Text.new("x"), doc.root.elements["b"].get_text.equal?(REXML::Text.new("x"))
`)
	assert.Equals(t, out, "<a><b>x</b><c/></a>\n<a>\n  <b>\n    x\n  </b>\n  <c/>\n</a>\n<b>\n x\n</b>\n<a>\n <b>x</b>\n <c/>\n</a>\ntrue\nfalse\ntrue\ntrue\nfalse\n")
}

func TestREXMLParseExceptionCarriesThePlace(t *testing.T) {
	out := runScript(t, `require "rexml/document"
begin
  REXML::Document.new("<a>\n  <b>\n</a>")
rescue REXML::ParseException => e
  p e.class.superclass, e.line, e.position, e.message.lines.first
end
p REXML::VERSION, REXML::Element.superclass, REXML::Document.superclass, REXML::CData.superclass, REXML::Text.superclass
`)
	assert.Equals(t, out, "RuntimeError\n3\n14\n\"Missing end tag for 'b' (got 'a')\\n\"\n\"3.4.0\"\nREXML::Parent\nREXML::Element\nREXML::Text\nREXML::Child\n")
}
