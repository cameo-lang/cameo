# io: give a script a file system

```sh
go run ./examples/io
```

```
3 lines
THREE
the script wrote 17 bytes into the file system
```

A script does no I/O unless the program that runs it allows a kind of
it. [main.go](main.go) allows the kind `file` with `prog.AllowIO("file")`
and says which files with `prog.FileSystem(fsys, dir)`: here a file
system in memory that holds one file, so [notes.cameo](notes.cameo) reads
and writes with `File.read`, `File.write` and `File.open` and never
touches the disk.

The site's [I/O](https://cameo-lang.github.io/cameo/io.html) page has the
kinds, and the same from the command line with `cameo --io=file`.
