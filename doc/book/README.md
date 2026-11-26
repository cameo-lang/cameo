# The cameo book

cameo's documentation as one book, in a reading order: what cameo is for,
embedding it in a Go program, how it differs from Ruby, then the guide for
agents, the design, performance and compatibility, and the reference. One
markdown file per chapter, built into one PDF.

```sh
make            # build/cameo.pdf
make html       # build/cameo.html
make md         # build/md/: one GitHub markdown file per chapter
make install-prerequisites   # pandoc and Chrome, with brew (macOS) or apt
make help
```

`make md` is for a markdown preview, such as the one of VS Code or GitHub.
The text, code, tables, images and links between chapters are those of the
book. The layout is not: the boxes (`source`, `why`, `rejected`) are quotes,
the two sides of a `pair` follow each other, and there are no pages. The PDF
is the book as it prints.

The build needs [pandoc](https://pandoc.org/installing.html) and Chrome or
Chromium. `make` finds Chrome on `PATH` or in `/Applications`; `CHROME=…`
names another.

## Layout

| Path | Holds |
|---|---|
| `chapters/<n>-<part>/00-part.md` | the title of part n |
| `chapters/<n>-<part>/<nn>-<name>.md` | a chapter: its title is the `##` heading, its sections `###` and below |
| `book.yaml` | pandoc's settings: title, contents, the stylesheet |
| `book.css` | the page: paper, page numbers, page breaks, code, boxes |
| `Makefile` | pandoc to HTML, Chrome to PDF, and pandoc to one GitHub markdown file per chapter |
| `gfm.lua`, `ids.lua` | the pandoc filters of `make md`: links between chapters become links between files |

The book takes the chapters in file-name order, so a new chapter is a new
file with the number of its place. A chapter's links to another chapter
point at its id, `#ch-<name>`, which its heading sets.

Three kinds of block have a style of their own:

- `::: source` names the file a chapter was made from.
- `::: pair` with two `::: side` blocks sets Ruby and cameo side by side.
- `::: why` is a box after each difference from Ruby, in part 3, for the
  reason cameo differs there. It is numbered, Why #1 to Why #31, and empty.

## Where the text comes from

The chapters were made on 2026-11-24 from the site (`doc/gh_pages`), the
README, `agents.md`, the agent-docs topics and the documents under `doc/`,
`examples/`, `benchmarks/` and `specs/`, at main `d51d4c6`. They are a
copy: the site and those files are still what cameo publishes, and what
`go test ./tests` checks. The book's own code blocks are not run.

Left out: the history (`doc/history/`), the guidance for agents that work
on cameo (`agents/`, `CLAUDE.md`), the work logs and the generated pages
of `doc/gh_pages/compatibility/`.
