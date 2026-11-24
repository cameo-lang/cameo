# The cameo book: one markdown file per chapter under chapters/, built into
# one PDF by pandoc (markdown to HTML, book.yaml) and Chrome (HTML to PDF,
# book.css).
#
#   make            # build/cameo.pdf
#   make html       # build/cameo.html, to read in a browser
#   make check      # every chapter parses, and the PDF builds
#   make clean

CHAPTERS := $(shell find chapters -name '*.md' | LC_ALL=C sort)
OUT      := build

PANDOC ?= pandoc
# Chrome or Chromium: the first one on PATH, or the macOS application.
CHROME ?= $(firstword $(foreach c,chromium chromium-browser google-chrome google-chrome-stable chrome,$(shell command -v $(c) 2>/dev/null)) \
            $(wildcard /Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome))

.PHONY: pdf html check clean prerequisites help
.DELETE_ON_ERROR:

pdf: $(OUT)/cameo.pdf ## Build the book as a PDF

html: $(OUT)/cameo.html ## Build the book as one HTML file

help: ## Show this help
	@grep -Eh '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "  %-8s %s\n", $$1, $$2}'

prerequisites: ## Check that pandoc and Chrome are installed
	@command -v $(PANDOC) >/dev/null || { echo "pandoc is not installed: https://pandoc.org/installing.html" >&2; exit 1; }
	@test -n "$(CHROME)" || { echo "Chrome or Chromium is not installed; or set CHROME=/path/to/chrome" >&2; exit 1; }

$(OUT)/cameo.html: $(CHAPTERS) book.yaml book.css | prerequisites
	@mkdir -p $(OUT)
	$(PANDOC) --defaults book.yaml -o $@ $(CHAPTERS)

$(OUT)/cameo.pdf: $(OUT)/cameo.html
	"$(CHROME)" --headless --disable-gpu --no-sandbox --no-pdf-header-footer \
	  --print-to-pdf="$(abspath $@)" "file://$(abspath $<)" 2>/dev/null
	@test -s $@

check: pdf ## Build the PDF, failing on a chapter pandoc cannot read

clean: ## Remove build/
	rm -rf $(OUT)
