---
intake: 1
id: divide
type: feature
plan: deliver-feature
status: ready
priority: 2
created: 2026-10-24
requested-by: e2e-harness
source: manual
trust: internal
base-branch: main
finish: branch
autonomy: inherit
budget:
  max-attempts: 2
  max-usd: 5
  max-hours: 1
scope:
  paths: [calc/**]
---
# Divide two integers

## Problem

`calc` can add but not divide, and callers have each written their own
division with their own handling of a zero divisor.

## Acceptance criteria

1. `calc.Divide(a, b int) (int, error)` returns `a / b`, truncated toward zero
   as Go's integer division is.
2. Dividing by zero returns the sentinel error `calc.ErrDivideByZero` and a
   quotient of 0, and never panics.
3. Nothing outside `calc/` changes.

## Out of scope

Floating-point division, overflow handling, and any change to `Add`.
