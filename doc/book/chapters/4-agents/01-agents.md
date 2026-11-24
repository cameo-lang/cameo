## Writing cameo programs: a guide for agents {#ch-agents}

::: source
from `agents.md`
:::

This file is for a model that writes cameo scripts or embeds cameo. The
guidance for agents that work on cameo itself is in [CLAUDE.md](https://github.com/cameo-lang/cameo/blob/main/CLAUDE.md).

**Write Ruby, and leave out everything that changes a value in place.** cameo
has Ruby's syntax and most of its core classes. Every value is immutable, so a
program is written as values computed from values.

The guide is built into cameo, as short markdown topics:

``` sh
cameo agent-docs             # the overview and the list of topics
cameo agent-docs <topic>     # one topic, such as rules, async or embedding
```

A Go program that embeds cameo reaches the same text through
`github.com/cameo-lang/cameo/src/agentdocs` (`agentdocs.Text(topic)`), to hand
it to the models it runs, and the site publishes it:
[llms.txt](https://cameo-lang.github.io/cameo/llms.txt) links each topic as
markdown, such as
[agent-docs/async.md](https://cameo-lang.github.io/cameo/agent-docs/async.md).
The topics are the files of [src/agentdocs/topics](https://github.com/cameo-lang/cameo/blob/main/src/agentdocs/topics):

| Topic                                              | Says                                                                    |
|----------------------------------------------------|-------------------------------------------------------------------------|
| [overview](#ch-agents-overview)       | what cameo is, the one rule, and the commands to check and run a script |
| [rules](#ch-agents-rules)             | what to leave out of Ruby, and what to write instead                    |
| [classes](#ch-agents-classes)         | classes with typed fields, and `with` for a changed copy                |
| [collections](#ch-agents-collections) | computing with collections without accumulators                         |
| [async](#ch-agents-async)             | `async` and `await`, for work that does not depend on other work        |
| [text](#ch-agents-text)               | text and bytes: Strings that are UTF-8 or binary                        |
| [recursion](#ch-agents-recursion)     | tail calls: a recursion that ends in the call runs at any depth         |
| [io](#ch-agents-io)                   | what I/O a script may do, and who allows it                             |
| [stdlib](#ch-agents-stdlib)           | the libraries a script may `require`                                    |
| [embedding](#ch-agents-embedding)     | running cameo from Go: `Load`, `Run`, `Call` and host modules           |
| [pause](#ch-agents-pause)             | pausing a running script into bytes and resuming it                     |
| [checklist](#ch-agents-checklist)     | what to check before handing a script over                              |

Every Ruby block of the topics is run by `go test ./tests`. The full
documentation is on the site,
[cameo-lang.github.io/cameo](#ch-overview); the
complete list of differences from Ruby is
[doc/cameo-vs-ruby.md](#ch-reference-cameo-vs-ruby).
