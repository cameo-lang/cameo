# 27. `require` of built-in libraries, CGI escaping and Time parsing

74 benchmark cases begin with `require "set"`, `require "time"` or
`require "cgi"`, and step 1 rejected `require` along with all file access:
an amber script is one file.

The libraries those cases load are part of what amber builds in, or small
enough to be. So `require` of a library amber builds in returns `true` the
first time and `false` after, as Ruby's does
([src/eval/install.go:130](https://github.com/radiospiel/amber/blob/@@COMMIT@@/src/eval/install.go#L130-L137)); the library is there
whether or not the script requires it. Any other name raises a `LoadError`,
which sits outside `StandardError` as in Ruby, so a bare `rescue` does not
swallow it. `require_relative` and `load` stay refused, since they read a
file.

The libraries:

- `set`: `Set` is built in since step 4.
- `time`: `Time.parse`, `Time.iso8601`, `Time.strptime` and
  `Time#iso8601`, and `Time.new` reads a string as Ruby 3.2's does.
- `cgi`, `cgi/escape`, `cgi/util`: `CGI.escapeHTML`, `unescapeHTML`,
  `escape`, `unescape`, and the URI-component pair.
- `pp`: `pp` is Kernel's.

`works/` holds 742: 23 `require` cases, and 9 `rpartition` and `scan`
cases that already worked once their `require` passed. The rest of
`unsupported/require` is filed by the library it loads.
