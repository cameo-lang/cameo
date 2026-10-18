# Task: module path github.com/cameo-lang/cameo

**Started:** 2026-10-18 12:15:00
**Ended:** 2026-10-18 12:25:00
**Strategy:** Refactoring
**Status:** Completed
**Complexity:** Simple
**Used Models:** Opus
**Token usage (Estimated):** 30k input, 5k output

## Objective
The repository moved to the cameo-lang organization; the module path
follows it.

## Progress
- [x] Replace github.com/radiospiel/cameo with github.com/cameo-lang/cameo
      everywhere but doc/history and agents/logs
- [x] The site's install line names v0.2.1, the first tag with the new path
- [x] make check; the site's Go programs against the new path

## Obstacles
- **Issue:** v0.2.0 declares the old path in its go.mod, so it cannot be
  fetched under the new one. **Resolution:** the site names v0.2.1, to be
  tagged after this merges.

## Outcome
One pull request; v0.2.1 to be tagged on its merge commit.
