# Context Manifest: intake-slice-check

## Scope

- **Tier**: analyst
- **Built for**: analyst

## Pinned Files

- `internal/intake/slice.go` — New file (salvaged, now applied and staged) holding the core slice parsing, project-root path guard, manifest parsing, and SHA-256 digest helpers the digest/manifest acceptance criteria describe.
- `internal/intake/slice_check.go` — New file implementing CheckSlices: cross-order coverage (missing/double-covered levels, unknown covers entries), digest/manifest verification, and depends-on cycle/dangling checks — the central logic every acceptance criterion maps to.
- `internal/intake/slice_check_test.go` — Fixture-based tests (reagent-map-style split) exercising the happy path and each violation by breaking the fixture one way, per the spec's Test Approach.
- `internal/intake/slice_test.go` — Unit tests for slice.go's parsing, path-guard, and digest helpers — needed to confirm the smaller building blocks are each covered, not only the integration-level slice_check_test.go.
- `internal/intake/order.go` — Defines Order.Slice *Slice and the Slice struct's YAML frontmatter fields — the schema for AC1's optional slice block, and what makes a slice-less order validate exactly as before (Slice == nil).
- `internal/intake/check.go` — Existing single-file Check entry point, unmodified by the patch; the analyst needs it to confirm the NFR that a directory with no slice blocks reads no extra files, and that single-file validation truly is unchanged.
- `cmd/loom/cmd/intake.go` — CLI wiring modified to collect parsed Documents, call CheckSlices per slice.of group, and print the one-line-per-spec summary (AC7); this is where the new check is invoked end to end.
- `cmd/loom/cmd/intake_test.go` — Command-level test running `loom intake check` over a fixture directory, per the spec's stated Test Approach of one cmd/loom/cmd test over the fixture.
- `templates/claude-feature-team/intake/TEMPLATE.md` — Must document the new optional slice: block (AC8); the analyst needs to confirm the documented shape matches order.go's actual fields.
- `templates/claude-feature-team/intake/README.md` — Must document the cross-order slice check's behavior (AC8), paired with TEMPLATE.md for consistency checking.

## Token Budget

- **Status**: OK — ~16458 tokens, 13.7% of the analyst tier budget (120000 tokens)
- Measured by loom from the files above (bytes ÷ 4), not estimated by an agent.

| File | Bytes | ~Tokens |
|---|---:|---:|
| `internal/intake/slice.go` | 4620 | 1155 |
| `internal/intake/slice_check.go` | 13489 | 3372 |
| `internal/intake/slice_check_test.go` | 8696 | 2174 |
| `internal/intake/slice_test.go` | 4208 | 1052 |
| `internal/intake/order.go` | 3570 | 892 |
| `internal/intake/check.go` | 1192 | 298 |
| `cmd/loom/cmd/intake.go` | 7701 | 1925 |
| `cmd/loom/cmd/intake_test.go` | 8811 | 2202 |
| `templates/claude-feature-team/intake/TEMPLATE.md` | 6764 | 1691 |
| `templates/claude-feature-team/intake/README.md` | 6790 | 1697 |

## Knowledge Items

None

## ADRs

- `docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md`
- `docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md`

## Pruned

- `internal/intake/vocabulary.go — trivial additive change (two new closed-vocabulary sets, knownObservableVia/knownLevel), already reviewed in full during manifest-building; low risk, not load-bearing enough to occupy a pin slot`
- `internal/intake/vocabulary_test.go — the 'existing template vocabulary test' AC8 regression target, but it checks only the pre-existing order-level fields (type/status/source/trust/finish/autonomy), not the new slice vocabulary, so it's a regression check rather than new-behavior evidence`
- `internal/intake/parse.go — unchanged by the salvaged patch; only its test file (parse_test.go, which is still excluded here) gained slice-frontmatter coverage`
- `internal/intake/parse_test.go — new slice-field parsing assertions, but parse.go itself didn't change and order.go already shows the schema directly`
- `internal/intake/validate.go / validate_test.go — single-order validation, untouched by this feature; slice validation is deliberately separate per the spec ('Today intake.Check validates one file at a time, so this adds the first check across orders')`
- `internal/intake/check_test.go — unmodified; check.go's own small test, not relevant to cross-order behavior`
- `tests/e2e/harness/*, internal/factory/* — distance-1 importers of internal/intake (confirmed via grep for the import path), but they belong to the factory/e2e-harness bounded contexts, not intake, and the only exported-surface change (Order gaining an additive Slice field) is not a breaking change for keyed struct literals`
- `docs/features/i-4-vocabulary-test/*, docs/features/vocabulary-mismatch-message/* — the two prior deliveries in the intake bounded context; already read in full during manifest-building (see note below), full artifacts not needed again downstream`
