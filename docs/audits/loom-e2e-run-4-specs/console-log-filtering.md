# Feature: Filter captured console logs by type

Add a `getLogsByType(type: string)` method to the existing `ConsoleLogger` class in
`packages/saturday-core/src/utils/console-logger.ts`, returning only the captured log
entries whose `type` matches the argument.

## Why
Scenario debugging is dominated by console errors, and callers currently have to filter
`getLogs()` by hand at every call site. A test that wants to assert "no console errors
occurred" should not have to know the shape of the internal log entry.

## Acceptance criteria
- `getLogsByType('error')` returns only entries whose `type` is `error`
- An unmatched type returns an empty array, never `undefined`
- The method performs no I/O and does not mutate the captured logs
- A unit test in `packages/saturday-core/tests/utils/console-logger.spec.ts` covers
  the match case and the empty-result case
