# frozen_string_literal: true

require "cgi"

# SitePage renders the Markdown compatibility.rb writes as a page of the
# site in doc/gh_pages: the header, with its navigation, and the footer
# of the site's index.html around the page's own HTML.
#
# It knows the Markdown that compatibility.rb writes and no more:
# headings, paragraphs, lists of one level, tables, links, code spans,
# bold and italics, and HTML comments, which it drops.
module SitePage
  REPOSITORY = "https://github.com/cameo-lang/cameo/blob/main/"

  module_function

  # page returns the page for markdown, made from the site's index.html
  # (template): its header with name in the navigation marked current,
  # title and description in the head, and its footer. The paragraph
  # after the first heading is the page's lead, as on the site's other
  # pages.
  def page(markdown, template:, name:, title:, description:)
    head = template[/\A.*?<main>\n/m] or raise "the template has no <main>"
    foot = template[%r{</main>\n.*\z}m] or raise "the template has no </main>"
    head = head.sub(%r{<title>.*?</title>}, "<title>cameo — #{h(title)}</title>")
               .sub(/<meta name="description" content="[^"]*">/, %(<meta name="description" content="#{h(description)}">))
               .gsub(' aria-current="page"', "")
               .sub(%(<a href="#{name}">), %(<a href="#{name}" aria-current="page">))
    head + html(markdown).sub(%r{(</h1>\n\n)<p>}, '\1<p class="lead">') + foot
  end

  # html renders markdown, block by block.
  def html(markdown)
    lines = markdown.gsub(/<!--.*?-->\n*/m, "").lines.map(&:chomp)
    out = +""
    until lines.empty?
      line = lines.shift
      case line
      when ""
        next
      when /\A(#+) (.*)\z/
        level = Regexp.last_match(1).size
        text = Regexp.last_match(2)
        out << %(<h#{level} id="#{anchor(text)}">#{inline(text)}</h#{level}>\n\n)
      when /\A\|/
        rows = [line]
        rows << lines.shift while lines.first&.start_with?("|")
        out << table(rows)
      when /\A- /
        items = [line.delete_prefix("- ")]
        while lines.first&.match?(/\A(- |  \S)/)
          next_line = lines.shift
          if next_line.start_with?("- ")
            items << next_line.delete_prefix("- ")
          else
            items[-1] += " #{next_line.strip}"
          end
        end
        out << "<ul>\n#{items.map { "  <li>#{inline(_1)}</li>\n" }.join}</ul>\n\n"
      else
        text = [line]
        text << lines.shift while lines.first && !lines.first.empty? && !lines.first.match?(/\A(#|\||- )/)
        out << "<p>#{inline(text.join("\n"))}</p>\n\n"
      end
    end
    out
  end

  # table renders the rows of a Markdown table, the second of which gives
  # the alignment of each column.
  def table(rows)
    header, align, *body = rows.map { cells(_1) }
    right = align.map { _1.end_with?(":") }
    cell = ->(tag, text, i) { right[i] ? %(<#{tag} class="num">#{inline(text)}</#{tag}>) : "<#{tag}>#{inline(text)}</#{tag}>" }
    out = +"<table>\n"
    out << "<tr>#{header.each_with_index.map { |t, i| cell.("th", t, i) }.join}</tr>\n"
    body.each { |row| out << "<tr>#{row.each_with_index.map { |t, i| cell.("td", t, i) }.join}</tr>\n" }
    out << "</table>\n\n"
  end

  # cells splits a table row at each | that is not escaped as \|.
  def cells(row)
    row.strip.delete_prefix("|").delete_suffix("|").split(/(?<!\\)\|/).map { _1.strip.gsub("\\|", "|") }
  end

  # inline renders code spans, links, bold and italics in text.
  # The code spans are set aside first, as \u0000 and their number, so
  # that nothing inside one reads as a link or emphasis.
  def inline(text)
    spans = []
    text = text.gsub(/``\s(.*?)\s``|`([^`]*)`/) do
      spans << "<code>#{h(Regexp.last_match(1) || Regexp.last_match(2))}</code>"
      "\u0000#{spans.size - 1}\u0000"
    end
    h(text).gsub(/\[([^\]]+)\]\(([^)\s]+)\)/) { %(<a href="#{h(href(CGI.unescapeHTML(Regexp.last_match(2))))}">#{Regexp.last_match(1)}</a>) }
           .gsub(/\*\*(.+?)\*\*/, '<strong>\1</strong>')
           .gsub(/(?<![\w*])\*([^*\s][^*]*?)\*(?![\w*])/, '<em>\1</em>')
           .gsub(/\u0000(\d+)\u0000/) { spans[Regexp.last_match(1).to_i] }
  end

  # href returns where a link of COMPATIBILITY.md points from the site: a
  # file of the repository on GitHub, an anchor or a URL as it is.
  def href(target)
    return target if target.start_with?("#") || target.match?(%r{\A[a-z]+://})

    REPOSITORY + target
  end

  # anchor is the id GitHub gives a heading: lowercase, without
  # punctuation, with a hyphen for each space.
  def anchor(text)
    text.downcase.gsub(/[^\w\- ]/, "").tr(" ", "-")
  end

  def h(text) = CGI.escapeHTML(text)
end
