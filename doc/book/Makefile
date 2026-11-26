# The cameo book: one markdown file per chapter under chapters/, built into
# one PDF by pandoc (markdown to HTML, book.yaml) and Chrome (HTML to PDF,
# book.css).
#
#   make            # build/cameo.pdf
#   make html       # build/cameo.html, to read in a browser
#   make md         # build/md/: one GitHub markdown file per chapter
#   make check      # every chapter parses, and the PDF builds
#   make install-prerequisites   # pandoc and Chrome, with brew or apt
#   make clean

CHAPTERS := $(shell find chapters -name '*.md' | LC_ALL=C sort)
OUT      := build

PANDOC ?= pandoc
# Chrome or Chromium: the first one on PATH, or the macOS application.
CHROME ?= $(firstword $(foreach c,chromium chromium-browser google-chrome google-chrome-stable chrome,$(shell command -v $(c) 2>/dev/null)) \
            $(wildcard /Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome))

# Code stays plain: no colours, no line anchors. pandoc 3.8 renamed the flag.
NOHIGHLIGHT := $(shell $(PANDOC) --help 2>/dev/null | grep -q -- --syntax-highlighting && echo --syntax-highlighting=none || echo --no-highlight)

MD := $(patsubst chapters/%,$(OUT)/md/%,$(CHAPTERS))

.PHONY: pdf html md check clean prerequisites pandoc install-prerequisites help
.DELETE_ON_ERROR:

pdf: $(OUT)/cameo.pdf ## Build the book as a PDF

html: $(OUT)/cameo.html ## Build the book as one HTML file

md: $(MD) $(OUT)/md/img ## Build one GitHub markdown file per chapter, for a preview

help: ## Show this help
	@grep -Eh '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "  %-22s %s\n", $$1, $$2}'

prerequisites: ## Check that pandoc and Chrome are installed
	@command -v $(PANDOC) >/dev/null || { echo "pandoc is not installed: make install-prerequisites, or https://pandoc.org/installing.html" >&2; exit 1; }
	@test -n "$(CHROME)" || { echo "Chrome or Chromium is not installed: make install-prerequisites, or set CHROME=/path/to/chrome" >&2; exit 1; }

pandoc: ## Check that pandoc is installed: all that make md needs
	@command -v $(PANDOC) >/dev/null || { echo "pandoc is not installed: make install-prerequisites, or https://pandoc.org/installing.html" >&2; exit 1; }

# install-prerequisites installs what is missing: with Homebrew on macOS,
# with apt on Debian and Ubuntu. It leaves alone what is there.
install-prerequisites: ## Install pandoc and Chrome, where they are missing
	@if command -v $(PANDOC) >/dev/null; then echo "pandoc: $$(command -v $(PANDOC))"; \
	elif command -v brew >/dev/null; then brew install pandoc; \
	elif command -v apt-get >/dev/null; then sudo apt-get install -y pandoc; \
	else echo "Install pandoc by hand: https://pandoc.org/installing.html" >&2; exit 1; fi
	@if test -n "$(CHROME)"; then echo "Chrome: $(CHROME)"; \
	elif command -v brew >/dev/null; then brew install --cask google-chrome; \
	elif command -v apt-get >/dev/null; then sudo apt-get install -y chromium; \
	else echo "Install Chrome or Chromium by hand, or set CHROME=/path/to/chrome" >&2; exit 1; fi

$(OUT)/cameo.html: $(CHAPTERS) book.yaml book.css | prerequisites
	@mkdir -p $(OUT)
	$(PANDOC) --defaults book.yaml $(NOHIGHLIGHT) -o $@ $(CHAPTERS)

$(OUT)/cameo.pdf: $(OUT)/cameo.html
	"$(CHROME)" --headless --disable-gpu --no-sandbox --no-pdf-header-footer \
	  --print-to-pdf="$(abspath $@)" "file://$(abspath $<)" 2>/dev/null
	@test -s $@

$(OUT)/ids.tsv: $(CHAPTERS) ids.lua | pandoc
	@mkdir -p $(OUT)
	@for f in $(CHAPTERS); do \
	  c=$${f#chapters/}; \
	  $(PANDOC) -f markdown -t plain --wrap=none --lua-filter ids.lua -M chapter=$${c%.md} $$f; \
	done > $@

$(OUT)/md/%.md: chapters/%.md gfm.lua $(OUT)/ids.tsv
	@mkdir -p $(dir $@)
	$(PANDOC) -f markdown -t gfm --wrap=none --lua-filter gfm.lua \
	  -M chapter=$* -M ids=$(OUT)/ids.tsv -o $@ $<

$(OUT)/md/img: $(wildcard ../gh_pages/img/*)
	@mkdir -p $@
	cp ../gh_pages/img/* $@/
	@touch $@

check: pdf ## Build the PDF, failing on a chapter pandoc cannot read

clean: ## Remove build/
	rm -rf $(OUT)
