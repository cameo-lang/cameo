i = 0
total = 0
while i < 1_000_000
  h = { "foo" => "bar", "bar" => "baz", "baz" => "lol", "lol" => "lgtm", "lgtm" => "nope", "nope" => "ok", "ok" => "again", "again" => "wait" }
  total += h.size
  i += 1
end
p total
