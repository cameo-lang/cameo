# warmruby

A command that runs a Ruby script on CRuby 3.4 compiled to WebAssembly
(WASI), on [wazero](https://wazero.io/), the Ruby a Go program can embed
without cgo. [../compare](../compare) measures amber against it.

```sh
make build
bin/warmruby [-time FILE] [-cache DIR] script.rb
```

- **The module** is `ruby+stdlib.wasm` from the npm package
  [@ruby/3.4-wasm-wasi](https://www.npmjs.com/package/@ruby/3.4-wasm-wasi)
  2.10.1, which packs Ruby's standard library into the module. The first
  run downloads it into the cache directory (`$XDG_CACHE_HOME/amber-warmruby`
  by default) and checks its SHA-256.
- **Warm** means compiled ahead: wazero compiles the module to machine code
  once, about 25 s, and keeps the result in the cache directory, so a later
  run starts from it in about two seconds, most of it Ruby's own boot.
- **The sandbox** sees the script's directory, read-only, as `/work`, and
  nothing else of the host; the script runs there as Ruby's main script.
- **`-time FILE`** writes the seconds the script itself ran, measured by
  Ruby around it, so the start is not in them.

The module is built for browsers too and imports the host functions of
Ruby's `js` gem; the command stubs them, so a script that requires `js`
fails.
