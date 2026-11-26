-- gfm.lua turns one chapter into GitHub-flavoured markdown that a plain
-- markdown preview shows: an anchor before each element with an id, links
-- to another chapter as links to its file, the book's boxes as quotes, and
-- images from build/md/img. Metadata: chapter (its path under chapters/,
-- without .md) and ids (a file of "id chapter" lines).

local ids, chapter = {}, ""

local function anchor(id)
  return '<a id="' .. id .. '"></a>'
end

local function image(src)
  return (src:gsub("^%.%./gh_pages/img/", "../img/"))
end

local function meta(m)
  chapter = pandoc.utils.stringify(m.chapter or "")
  local f = io.open(pandoc.utils.stringify(m.ids or ""), "r")
  if f then
    for line in f:lines() do
      local id, ch = line:match("^(%S+)%s+(%S+)$")
      if id then ids[id] = ch end
    end
    f:close()
  end
end

local function header(h)
  if h.identifier == "" then return nil end
  return { pandoc.RawBlock("html", anchor(h.identifier)), h }
end

local function span(s)
  if s.identifier == "" then return nil end
  local out = { pandoc.RawInline("html", anchor(s.identifier)) }
  for _, x in ipairs(s.content) do table.insert(out, x) end
  return out
end

local function link(l)
  local id = l.target:match("^#(.+)$")
  local ch = id and ids[id]
  if ch and ch ~= chapter then
    l.target = "../" .. ch .. ".md#" .. id
  end
  return l
end

local function img(i)
  i.src = image(i.src)
  return i
end

local function raw(r)
  if r.format == "html" then
    r.text = r.text:gsub('src="%.%./gh_pages/img/', 'src="../img/')
  end
  return r
end

local function div(d)
  if d.classes:includes("source") or d.classes:includes("why")
      or d.classes:includes("rejected") then
    return pandoc.BlockQuote(d.content)
  end
  return d
end

-- Meta first, so that the links know every chapter's ids.
return {
  { Meta = meta },
  { Header = header, Span = span, Link = link, Image = img,
    RawBlock = raw, RawInline = raw, Div = div },
}
