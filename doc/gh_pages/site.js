// Highlights the samples with highlight.js, vendored in hljs/: the class
// of a <pre> names its language, and an output block stays as printed.
import hljs from "./hljs/core.min.js";
import ruby from "./hljs/ruby.min.js";
import go from "./hljs/go.min.js";
import bash from "./hljs/bash.min.js";
import protobuf from "./hljs/protobuf.min.js";
import diff from "./hljs/diff.min.js";

hljs.registerLanguage("ruby", ruby);
hljs.registerLanguage("go", go);
hljs.registerLanguage("bash", bash);
hljs.registerLanguage("protobuf", protobuf);
hljs.registerLanguage("diff", diff);

// The language of each <pre> class. cameo is highlighted as Ruby: it
// parses as Ruby.
const languages = {
  cameo: "ruby",
  ruby: "ruby",
  go: "go",
  sh: "bash",
  proto: "protobuf",
  diff: "diff",
};

for (const [cls, language] of Object.entries(languages)) {
  for (const code of document.querySelectorAll(`pre.${cls} > code`)) {
    code.classList.add(`language-${language}`);
    hljs.highlightElement(code);
  }
}
