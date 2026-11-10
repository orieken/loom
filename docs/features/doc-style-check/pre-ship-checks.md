# Pre-Ship Checks

## changed-line coverage

- Command: `scripts/pre-ship-check.sh`
- Duration: 1m44.789712334s
- Status: exit 0

```
ok  	github.com/orieken/loom/internal/mutation	3.317s	coverage: 100.0% of statements
ok  	github.com/orieken/loom/internal/orchestrator	4.863s	coverage: 88.6% of statements
ok  	github.com/orieken/loom/internal/patternspec	2.803s	coverage: 96.2% of statements
ok  	github.com/orieken/loom/internal/planfile	2.784s	coverage: 59.1% of statements
ok  	github.com/orieken/loom/internal/policy	2.806s	coverage: 92.6% of statements
ok  	github.com/orieken/loom/internal/provider/claude	14.446s	coverage: 95.7% of statements
ok  	github.com/orieken/loom/internal/provider/mock	2.654s	coverage: 53.5% of statements
ok  	github.com/orieken/loom/internal/references	3.221s	coverage: 96.8% of statements
ok  	github.com/orieken/loom/internal/repodoc	2.631s	coverage: 95.7% of statements
ok  	github.com/orieken/loom/internal/repograph	3.148s	coverage: 92.8% of statements
ok  	github.com/orieken/loom/internal/repoindex	2.712s	coverage: 90.9% of statements
ok  	github.com/orieken/loom/internal/repoquery	7.122s	coverage: 98.1% of statements
	github.com/orieken/loom/internal/repoquery/fixture		coverage: 0.0% of statements
ok  	github.com/orieken/loom/internal/state	2.065s	coverage: 91.0% of statements
ok  	github.com/orieken/loom/internal/telemetry	2.506s	coverage: 91.8% of statements
ok  	github.com/orieken/loom/internal/testlint	2.010s	coverage: 93.1% of statements
ok  	github.com/orieken/loom/internal/worktree	5.164s	coverage: 89.1% of statements
?   	github.com/orieken/loom/shared/mcp	[no test files]
	github.com/orieken/loom/shared/mcp/cmd/mcp-server		coverage: 0.0% of statements
ok  	github.com/orieken/loom/shared/mcp/internal/analyzers	2.666s	coverage: 69.2% of statements
ok  	github.com/orieken/loom/shared/mcp/internal/domain	1.865s	coverage: 0.0% of statements
	github.com/orieken/loom/shared/mcp/internal/logging		coverage: 0.0% of statements
ok  	github.com/orieken/loom/shared/mcp/internal/server	2.446s	coverage: 90.4% of statements
ok  	github.com/orieken/loom/shared/mcp/internal/tools	4.542s	coverage: 90.0% of statements
ok  	github.com/orieken/loom/shared/mcp/register	2.448s	coverage: 66.7% of statements
ok  	github.com/orieken/loom/tests/e2e/cmd/factory-e2e	2.571s	coverage: 94.4% of statements
ok  	github.com/orieken/loom/tests/e2e/harness	10.530s	coverage: 90.9% of statements
ok  	github.com/orieken/loom/tools	2.388s	coverage: 95.3% of statements
changed-line coverage: 93.3% (279 of 299 executable changed lines; threshold 85%)
  UNCOVERED   internal/docstyle/check.go:35
  UNCOVERED   internal/docstyle/check.go:36
  UNCOVERED   internal/docstyle/check.go:42
  UNCOVERED   internal/docstyle/check.go:43
  UNCOVERED   internal/docstyle/check.go:54
  UNCOVERED   internal/docstyle/check.go:55
  UNCOVERED   internal/docstyle/check.go:83
  UNCOVERED   internal/docstyle/check.go:84
  UNCOVERED   internal/docstyle/check.go:94
  UNCOVERED   internal/docstyle/check.go:95
  UNCOVERED   internal/docstyle/check.go:112
  UNCOVERED   internal/docstyle/check.go:113
  UNCOVERED   internal/docstyle/check.go:114
  UNCOVERED   internal/docstyle/check.go:115
  UNCOVERED   internal/docstyle/dictionary.go:52
  UNCOVERED   internal/docstyle/dictionary.go:53
  UNCOVERED   internal/docstyle/dictionary.go:83
  UNCOVERED   internal/docstyle/dictionary.go:84
  UNCOVERED   internal/docstyle/sentences.go:61
  UNCOVERED   internal/docstyle/sentences.go:62
PASS
```

