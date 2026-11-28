## The libraries a script may require {#ch-agents-stdlib}

::: source
from `src/agentdocs/topics/stdlib.md`
:::

These are built in; `require` returns true and they follow Ruby 3.4.1:

`abbrev`, `base64`, `benchmark`, `bigdecimal`, `cgi`, `csv`, `date`,
`delegate`, `digest`, `forwardable`, `getoptlong`, `ipaddr`, `json`, `logger`,
`matrix`, `openssl` (digests and HMAC), `optparse`, `pathname`, `pp`, `prime`,
`securerandom`, `set`, `shellwords`, `singleton`, `stringio`, `strscan`,
`time`, `tsort`, `unicode_normalize`, `uri` and `zlib`.

Any other `require` raises `LoadError`, as do `require_relative` and `load`: a
script is one file.

-   `net/http` reaches the network only through a client the Go host gives the
    program; without one, `require "net/http"` raises.
-   `StringIO`, `StringScanner` and `Logger` are handles: a value whose calls
    advance the thing it names, which every holder sees, and which cannot be
    copied, passed to `async` or stored in a field.
-   Libraries whose work is the file system, processes, threads or
    metaprogramming (`fileutils`, `open3`, `socket`, `timeout`, `ostruct`,
    `yaml`, `erb`, …) are left out.

The full table, with what is planned and why each library is in or out, is
<https://github.com/cameo-lang/cameo/blob/main/doc/stdlib.md>.
