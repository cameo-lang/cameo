# WRITING.md — how to write in cameo

Everything an agent writes — pull request titles and descriptions, commit
messages, code comments, documentation, and messages to the human in a session —
follows these rules. This is how work in this repository is communicated, not a
preference about prose.

## Principles

**Lead with the answer**, then the evidence — not the investigation in the
order it happened.

**Say it once.** A point made in the summary does not need repeating in the
bullet under it and again in the closing line. Three phrasings of one idea read
as three ideas, and the reader spends effort discovering they are not.

**Cut what the reader already knows.** Restating the question, recapping the
last message, narrating what you are about to do. Start with the content.

**No filler openers.** Not "This PR introduces…", not "Great question!".

**No editorializing, personification or invented causality.** Not "in order to
finally tame the beast", not "the parser had been struggling", not "that is the
whole point". State the reason with nothing dressed around it.

**No flowery wording.** Plain words, in generally agreed-upon terms. Where a
shorter sentence says the same thing, it is the better sentence.

**Claim only what you verified.** "Should work" is not a result. Say what you
ran, and say what you did not run.

**Use the reader's vocabulary.** A word that means something specific in this
repository must not be used for something else without saying so — a "value"
is a cameo value here, so a Go value is "a Go value". Pick one word per
concept and keep it: if a comment says "repository", the next sentence does not
say "repo", "checkout" or "project" for the same thing.

**One problem per paragraph, stated as a problem and its impact.** Two
sentences: what is wrong, what it costs. Add the fix if you have one. Several
problems braided into one narrative are unactionable.

## Word choice

Describe behaviour with the verb the code uses.

- A function **returns** a value. Not "answers", "hands back", "yields" or
  "reports", unless it is a `yield`.
- A function **returns an error**, or **fails**. Not "complains".
- Code **calls**, **reads**, **writes**, **opens**, **starts**, **stops**. Not
  "kicks off", "spins up", "tears down".
- A value **is** nil, empty or true. Not "comes back nil".

A comment on a function starts with its name and what it returns:

```go
// gitBinaryFor returns the path of a git binary to read a repository with
// when go-git refuses it, and false when there is none.
```

not `// gitBinaryFor answers whether ...`.

Name the thing, not the feeling:

- Say what is unsupported, missing or wrong. Not "unfortunately", "sadly",
  "surprisingly", "interestingly".
- No intensifiers: "very", "really", "simply", "just", "obviously",
  "actually". If a sentence needs one to be true, the sentence is vague.
- No hedges in place of a check: "should work", "probably", "seems to".

Sentences: active voice, present tense, subject first ("go-git rejects the
extension", not "the extension is rejected by go-git"). Two short sentences
beat one with three clauses. A list is for parallel items; a reason and its
consequence are a sentence.

## Session replies

What changed, what state it is in, and what needs a decision from the human.
Detail belongs in the pull request description, where a reviewer wants it — not
in the terminal.

Name a pull request as a link, not a bare number, so it can be followed without
retyping it: `[cameo-lang/cameo#35](https://github.com/cameo-lang/cameo/pull/35)`.

## Comments in code

A comment states what the code cannot: a constraint, an invariant, an external
behaviour (a git version difference, an API quirk), the one non-obvious reason
a line is the way it is. Keep it concise and factual.

- No history ("an earlier version…", "this used to…", "previously", "now" —
  `git log` carries it) and no narration of the change.
- Do not restate the code: `// increment i` says nothing.
- Name the external thing precisely: the git version, the extension, the error
  value. "Some repositories" is not a constraint.

Two placement rules keep comments short:

- **Needs more than a few sentences?** Write it once in a doc — the README or
  `doc/` — and link it from the comment instead of repeating it.
- **One explanation covering many functions in a file?** State it once in a
  comment at the top of the file (in Go, the package comment), not at each
  function.

## Documentation

Use the reader's vocabulary, one problem per paragraph, and no history that
`git log` already carries.

## Commit messages

The subject says what changed and why, and reads on its own in `git log
--oneline`, in the imperative, with the area it touches as a prefix where one helps
(`parser: accept a rescue modifier after a method call`). The body gives the cause
first, then the change, then what you ran and what you did not run.

## Pull request titles and descriptions

A title is short and descriptive: `<area>: <verb>s <what>`, completing
the sentence "This pull request …". It is not a command, so "stdlib: adds
String#squeeze", not "stdlib: add String#squeeze". It reads on
its own in a list. "adds LogFirstParent, a walk with no far end" is a riddle;
"renames FirstParentCommits to BranchHistory and gives it an optional base" is
the same length and tells you something. Commit subjects stay in the
imperative (see above), but the same test applies to them: ask whether
someone scanning `git log --oneline` would learn what changed.

Write the title last, from the finished description: it is the description's
summary sentence cut down to one line, not the branch name or the first
commit's subject. Five checks:

1. **It names the change, not a part of it.** "a Value is a tagged struct,
   and integers no longer box" names an implementation detail and one of its
   effects; the change "keeps integers and floats off the heap", and that is
   the title. If
   the summary sentence and the title would surprise each other, the title
   is wrong.
2. **It uses the reader's words.** A reviewer who has not read the diff
   knows the language and its behaviour, not the names of new types. Prefer
   the behaviour or the area ("parser: ...", "stdlib: ...") over the
   identifier.
3. **It is a statement, not a hint.** No "improvements", "fixes",
   "tweaks", "misc", "WIP". If the change is two things, the title says
   both or the pull request is two pull requests.
4. **It names what the pull request achieves, not what was missing.**
   "stdlib: Hash#min loses ties" describes the state before the change;
   "stdlib: Hash#min returns the first of equal elements" is the title. Read on its own in a list, a title that states a
   shortcoming looks like a bug report.
5. **It says what the pull request does, not how the product behaves
   afterwards, and leaves the mechanism to the description.** Flags,
   settings paths, hand-offs between parts and model names a reviewer
   does not need to recognise the change belong in *What*:

   | Not | But |
   |---|---|
   | eval: a call site keeps a cache of the method it found, keyed on the receiver's class | eval: caches method lookup per call site |
   | cli: cameo check prints every error it finds, through a new reporter type | cli: cameo check reports every error, not only the first |

**No person names in pull request titles and descriptions.** It is not
important who asked for a change, but why someone asked for it. Name no
person: not the author, not a reviewer, not the human who asked for the
change, and no handle or `@mention`. Do not address anyone either — no "as you asked", "you
chose", "per your request". A pull request is read by people who were not in
the conversation, and outlives it; it states what changes and why, and that
is true whoever asked. The same holds for commit messages.

Descriptions use this template, which also ships as
`.github/pull_request_template.md` so GitHub pre-fills it:

```markdown
<one sentence: what this pull request does and why>

## What
- <change 1>
- <change 2>

## Why
<the actual reason: bug, ticket link, requirement — 1-3 sentences, factual>

## Test
<what was verified and how — commands run, suites green — and what was not>

## Notes (optional)
<breaking changes, follow-ups>
```

1. **Summary** — one sentence, with no heading, that says what the pull
   request does and why. A reviewer who reads only this sentence knows what
   they are looking at.
2. **What** — a bullet list of concrete changes: files and modules touched,
   behaviour changed. Not a narrative.
3. **Why** — the actual reason, stated as fact. The bug, the ticket, the
   requirement. "Because someone asked for it" is never the reason: a
   person asks for a change because of something, and that something —
   what was failing, what users could not do — is what goes here. If you
   do not know it, ask before you write the description.
4. **Test** — what was verified and what was not. A description that claims
   more than was run is worse than a short one.

The principles above apply in full; a pull request description is not a
special case of them.

## Before sending

Read it once as the reader who has not seen the session. Cut every sentence
that does not change what they know or do.
