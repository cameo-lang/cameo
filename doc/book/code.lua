-- code.lua prepares code blocks for highlighting: a cameo block is
-- highlighted as Ruby, whose syntax it has. Blocks of output stay plain.

function CodeBlock(cb)
  if cb.classes:includes("cameo") and not cb.classes:includes("ruby") then
    cb.classes:insert(1, "ruby")
  end
  return cb
end
