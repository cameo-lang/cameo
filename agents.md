# Writing cameo programs: a guide for agents

This file is for a model that writes cameo scripts or embeds cameo. The
guidance for agents that work on cameo itself is in
[CLAUDE.md](CLAUDE.md).

**Write Ruby, and leave out everything that changes a value in place.**
cameo has Ruby's syntax and most of its core classes. Every value is
immutable, so a program is written as values computed from values.

The guide is built into cameo, as short markdown topics:

```sh
cameo agent-docs             # the overview and the list of topics
cameo agent-docs <topic>     # one topic, such as rules, async or embedding
```

A Go program that embeds cameo reaches the same text through
`github.com/cameo-lang/cameo/src/agentdocs` (`agentdocs.Text(topic)`), to
hand it to the models it runs, and the site publishes it:
[llms.txt](https://cameo-lang.github.io/cameo/llms.txt) links each topic
as markdown, such as
[agent-docs/async.md](https://cameo-lang.github.io/cameo/agent-docs/async.md).
The topics are the files of [src/agentdocs/topics](src/agentdocs/topics):

| Topic | Says |
|---|---|
| [overview](src/agentdocs/topics/overview.md) | what cameo is, the one rule, and the commands to check and run a script |
| [rules](src/agentdocs/topics/rules.md) | what to leave out of Ruby, and what to write instead |
| [classes](src/agentdocs/topics/classes.md) | classes with typed fields, and `with` for a changed copy |
| [collections](src/agentdocs/topics/collections.md) | computing with collections without accumulators |
| [async](src/agentdocs/topics/async.md) | `async` and `await`, for work that does not depend on other work |
| [text](src/agentdocs/topics/text.md) | text and bytes: Strings that are UTF-8 or binary |
| [recursion](src/agentdocs/topics/recursion.md) | tail calls: a recursion that ends in the call runs at any depth |
| [io](src/agentdocs/topics/io.md) | what I/O a script may do, and who allows it |
| [stdlib](src/agentdocs/topics/stdlib.md) | the libraries a script may `require` |
| [embedding](src/agentdocs/topics/embedding.md) | running cameo from Go: `Load`, `Run`, `Call` and host modules |
| [pause](src/agentdocs/topics/pause.md) | pausing a running script into bytes and resuming it |
| [checklist](src/agentdocs/topics/checklist.md) | what to check before handing a script over |

Every Ruby block of the topics is run by `go test ./tests`. The full
documentation is on the site,
[cameo-lang.github.io/cameo](https://cameo-lang.github.io/cameo/); the
complete list of differences from Ruby is
[doc/cameo-vs-ruby.md](doc/cameo-vs-ruby.md).
