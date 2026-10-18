# Task: Field numbers derived from names, number:, and merging two definitions

**Started:** 2026-10-18 14:30:00
**Ended:** 2026-10-18 15:30:00
**Strategy:** Feature (TDD)
**Status:** Completed
**Complexity:** Medium
**Used Models:** Opus
**Token usage (Estimated):** 400k input, 40k output

## Objective
Number a class's fields from a hash of their names instead of declaration
order, let a declaration give a number from 1 to 4096 with `number:`, and
give hosts a merge of two definitions of one message, the rules ark's
datastore uses for its document types.

## Progress
- [x] `protogen.FieldNumber`: 5535 + 60000 * fnv32a(name) / MaxUint32, past 19000 to 19999, next free on a collision
- [x] `number:` on a field declaration of one field, 1 to 4096, checked against fields and `reserved`
- [x] `protogen.Merge`: fields matched by name, added fields numbered by `FieldNumber`, their types and imports copied
- [x] Expected outputs, the site sample and the design document updated

## Obstacles
- **Issue:** Every class's wire form changes: field numbers were 1, 2, 3, ...
  **Resolution:** Accepted as the point of the change; `number:` keeps a number fixed.

## Outcome
Fields are numbered by name or by `number:`. `protogen.Merge` returns the
merged `FileDescriptorProto`; it does not render `.proto` text.

## Insights
A derived number takes a three-byte tag; `number:` up to 15 takes one.
