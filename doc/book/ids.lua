-- ids.lua replaces a chapter with one line "id chapter" for each id in
-- it, also the ids pandoc gives headings, so that gfm.lua can turn a link
-- to another chapter into a link to its file. Metadata: chapter.

return {
  {
    Pandoc = function(doc)
      local chapter = pandoc.utils.stringify(doc.meta.chapter or "")
      local lines = {}
      local function add(el)
        if el.identifier ~= "" then
          table.insert(lines, pandoc.Plain(el.identifier .. " " .. chapter))
        end
      end
      doc:walk({ Header = add, Div = add, Span = add, Code = add, CodeBlock = add })
      return pandoc.Pandoc(lines)
    end,
  },
}
