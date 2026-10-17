# Teamwork — agents and humans

**The human is a reviewer, not a co-driver.** Between "here is the task" and
"here is a pull request to look at" there should be nothing they have to do.

## What the agent decides, and what the human decides

Do it yourself: rebasing, restacking, fixing a failing test, resolving a
conflict, rewriting a stale description, splitting a branch that grew into
two concerns, chasing CI to green.

Stop and ask when the decision is genuinely theirs:

- merging a pull request, and anything else that is irreversible for other
  people;
- choosing between designs with real trade-offs, where the answer changes what
  gets built rather than how it is spelled — above all what the language
  accepts and how it behaves;
- anything that spends money, touches a credential, or publishes outside this
  repository;
- a request whose two readings would produce materially different work.

A question that has an obvious default is not a decision. Take the default,
say which one you took, and carry on.

## Report in a few lines

Lead with what changed and what state it is in, then what needs the human, if
anything. Detail belongs in the pull request description, not in the
terminal.

Say what you ran and what you did not. A green suite is a number; quote it. A
step you skipped is a sentence; write it. Reporting a task complete when part
of it is unfinished costs the most trust, because it moves the checking back
onto the person who asked.

The tone rules are in [WRITING.md](WRITING.md).
