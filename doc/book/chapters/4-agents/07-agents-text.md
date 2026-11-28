## Text and bytes {#ch-agents-text}

::: source
from `src/agentdocs/topics/text.md`
:::

A String is text (UTF-8) or bytes (binary), and cameo keeps the two apart
where Ruby lets them mix. Text holds whole characters alone: no String is
"invalid UTF-8".

``` ruby
text = "héllo"                         # UTF-8
bytes = b"\xFF\x00"                    # binary; "…".b makes one of text
p text.length, bytes.bytesize          # 5 and 2

packet = b"ID:" + text                 # binary: bytes joined with anything are bytes
p packet.encoding                      # #<Encoding:BINARY (ASCII-8BIT)>

again = packet[3..].force_encoding("UTF-8")   # say that bytes are text
puts "got #{again}"                    # interpolation takes text alone
```

-   Joining with `+`, `<<` or `join` gives bytes as soon as one part is
    binary.
-   An interpolation and `format` raise `Encoding::CompatibilityError` for a
    binary String, and so do `upcase`, `strip`, `split` and the other methods
    that read characters. `force_encoding("UTF-8")` says the bytes are text,
    and raises when they are not.
-   What reads text raises for other bytes, and never returns a broken String:
    `File.read` (use `mode: "rb"` for bytes), `JSON.parse`, `CGI.unescape`,
    `URI.decode_www_form_component`.
