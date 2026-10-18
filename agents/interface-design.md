# Interface design — lean interfaces, deep modules, facades

How to shape the public surface of anything in cameo: a Go package, a Ruby
module, an HTTP handler, a CLI command, a C export. The rules are the same in
every language, so they live here. They follow the "deep modules" school of
John Ousterhout's *A Philosophy of Software Design*: the value of a module is
its functionality divided by the size of its interface.

Read this before you add a public function, rename one, or add a parameter to
one. The public surface is the expensive part of a component — implementations
can be rewritten; interfaces have callers.

## The four rules

### 1. Keep the interface as lean as possible

A caller should be able to state their business case, and nothing else. Every
parameter is a question the caller has to answer; a parameter they cannot
answer from their own context is a parameter that does not belong in the
interface.

Concretely:

- A function that most callers invoke with the same constants is telling you
  those parameters want defaults — or want to be gone. Look at the call sites
  before you add a knob: if you cannot name the second caller that needs it,
  the first one should get its own narrow entry point instead.
- Prefer one options struct / keyword argument with defaults over a growing
  positional list, and prefer *no* option over either. An option whose
  meaning needs the implementation open in another window
  (`MergeBase: o.head == ""`) has already failed.
- Do not export something because a test wants it. Test through the public
  surface; when that is genuinely impossible, keep the helper internal to the
  component (`internal/` in Go, `private` in Ruby) rather than widening the
  contract.

The test: read the call site alone, without the callee. If a reviewer cannot
tell what it does, the interface is not lean — it is merely small.

### 2. Speak the caller's language, not the callee's

Name public things after what the caller is trying to do, never after how the
callee does it. Implementation vocabulary in a public name is a leak: it
freezes today's implementation into the contract, and it forces every caller
to learn your internals before they can express their intent.

Concretely:

- `BranchCommits` (what the caller wants: the commits on this branch) beats
  `FirstParentCommits` (how git walks to produce them) — `gitutils` names it
  that way for this reason.
- A caller asking "is this file untracked?" should get an answer in those
  terms. Handing them `IsTracked` ("is it in the index") makes the caller do
  git-internals reasoning at every call site — and getting that reasoning
  wrong is precisely how staged deletions ended up rendered as unreadable
  untracked files.
- The same applies to CLI flags and YAML keys: they are interfaces too.
  `cameo check --all` says what the user wants; a flag named after the
  traversal it uses would not survive the traversal changing.

The test: could you swap the implementation for a completely different one
without any public name becoming a lie? If a name would turn false, it was
describing the callee.

### 3. Lean interface, fat module

Complexity has to live somewhere. Put it *below* the interface, once, instead
of above it in every caller. A module that is easy to write but pushes
decisions to its callers ("shallow") multiplies its complexity by the number
of call sites; a module that is hard to write but easy to call ("deep") pays
the cost once.

Concretely:

- When a caller must call your functions in a particular order, or combine
  two of them to get the obviously-wanted result, fold that sequence into the
  module as the real operation. Three primitives plus a README paragraph is a
  worse interface than one verb.
- Policy belongs with the module that can enforce it uniformly. In ark,
  where cameo started, critic's "re-read the diff without whitespace flags when the summary said the file
  changed" lives once, above the git backends, precisely so that every
  backend behaves the same instead of each caller (or each backend) deciding
  for itself. That is the pattern.
- Handle the edge cases inside. An interface whose documentation is mostly
  "note that you must first check X" is billing its callers for its own
  shallowness.

The test: count what a *new* call site needs — lines, prior calls, facts about
the internals. A deep module needs one line and no facts.

### 4. Facades over implementations

We really, really like facades. A caller binds to a small, stable surface that
names the *capability*; the implementation lives behind it and can be swapped
without any caller noticing. This is what lets a project replace a subprocess with a
library, a real service with a fake, an old engine with a new one — without a
repo-wide refactor.

The example from ark is its `gitutils` part: callers see one `Repository`
surface; behind it sit two interchangeable backends, `impl/exec` (shells out
to git) and `impl/gogit` (pure Go, works on an iPad where there is no git
binary). critic is written against the facade and does not know which one it
got.

The obligations that come with a facade — a facade is a *promise*, and these
keep it honest:

- **The contract lives in the facade, not in a backend's habits.** Document
  each operation's semantics on the interface, and hold every implementation
  to it. Two backends answering the same call differently is a broken facade,
  even while all tests pass.
- **Test implementations against each other.** Run the same suite over every
  backend (as `gitutils`' worktree tests do) so divergence fails loudly. A
  backend without the shared suite is a fork, not an implementation.
- **Do not let backend types, errors, or vocabulary escape.** The moment a
  caller type-switches on the concrete implementation, or a public name
  mentions one (`exec`, `gogit`, a vendor name), swapping is off the table.
- **Facades stay lean too.** A facade that mirrors one implementation's every
  method is that implementation wearing a mask; design its surface from the
  callers' needs (rule 1) and its names from the callers' language (rule 2).

## When you touch an interface

1. **Adding a parameter or method?** Justify it from a real caller, today.
   Speculative generality is interface debt with no borrower.
2. **Renaming or extracting?** Move toward caller vocabulary, and keep the
   old name only as a documented, deprecated alias if callers exist outside
   your change.
3. **Adding a second implementation?** Put it behind the existing facade, run
   it through the shared conformance tests, and fix the *contract* wherever
   the two disagree — never the caller.
4. **Reviewing?** Read each changed public signature as its call site will
   read. "What would confuse the caller?" finds more real bugs than "is this
   implementation clever?".

A good interface is boring at the call site and interesting inside. When in
doubt, make the module deeper and the surface smaller.
