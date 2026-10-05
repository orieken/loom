package repodoc

import (
	"testing"

	"github.com/orieken/loom/internal/factsheet"
	"github.com/orieken/loom/internal/state"
)

// ledger is the sheet the verifier cases are written against.
func ledger() factsheet.FactSheet {
	return factsheet.FactSheet{
		Path: "internal/ledger", Files: []string{"ledger.go", "verify.go", "ledger_test.go"},
		Exports: []string{"Digest", "Ledger", "Record", "Verify"}, Imports: []string{"internal/telemetry"},
		ApproximateImports: []string{"internal/legacy"}, Dependents: []string{"cmd/loom"},
		Entrypoints: []string{"main.go"}, Tests: []string{"ledger_test.go"}, Children: []string{"internal/ledger/store"},
		InputHash: "sha256:h",
	}
}

func ledgerVocabulary() Vocabulary {
	return newVocabulary([]Tag{{Name: "integrity"}, {Name: "state"}})
}

// goodDoc is a document the verifier must accept as it stands.
func goodDoc() state.RepoDocState {
	return state.RepoDocState{
		SchemaVersion: state.SchemaVersion, Path: "internal/ledger", InputHash: "sha256:h",
		Purpose:          "Records entries and checks them with `Verify`.",
		Responsibilities: []string{"Computes a `Digest` for each `Record`"},
		KeyEntrypoints:   []string{"Verify", "main.go"},
		Tags:             []string{"integrity"}, Layer: state.LayerApplication,
		Invariants: []state.DocInvariant{{Statement: "Only `cmd/loom` depends on it", Evidence: []string{"dependents:cmd/loom"}}},
		Evidence: []state.DocEvidence{
			{Claim: "Records entries and checks them with `Verify`.", Fields: []string{"exports:Verify", "exports:Record"}},
			{Claim: "Computes a `Digest` for each `Record`", Fields: []string{"exports:Digest", "files:ledger.go"}},
			{Claim: "Only `cmd/loom` depends on it", Fields: []string{"dependents"}},
		},
	}
}

func codes(violations []Violation) []string {
	var out []string
	for _, violation := range violations {
		out = append(out, violation.Code)
	}
	return out
}

func hasCode(violations []Violation, code string) bool {
	for _, violation := range violations {
		if violation.Code == code {
			return true
		}
	}
	return false
}

func verifyWith(mutate func(*state.RepoDocState)) []Violation {
	doc := goodDoc()
	mutate(&doc)
	return Verify(ledger(), doc, ledgerVocabulary())
}

func TestACleanDocumentHasNoViolations(t *testing.T) {
	if violations := Verify(ledger(), goodDoc(), ledgerVocabulary()); len(violations) != 0 {
		t.Errorf("a faithful document was rejected: %+v", violations)
	}
}

func TestEveryNamedTagMustBeInTheVocabulary(t *testing.T) {
	violations := verifyWith(func(d *state.RepoDocState) { d.Tags = []string{"integrity", "audit"} })
	if len(violations) != 1 || violations[0].Code != CodeUnknownTag || violations[0].Where != "tags" {
		t.Errorf("violations = %+v", violations)
	}
}

func TestKeyEntrypointsMustBeExportsOrEntrypoints(t *testing.T) {
	violations := verifyWith(func(d *state.RepoDocState) { d.KeyEntrypoints = []string{"Verify", "Rollback", "ledger.go"} })
	if len(violations) != 2 || !hasCode(violations, CodeUnknownEntrypoint) {
		t.Errorf("Rollback and a plain file are not entrypoints or exports: %+v", violations)
	}
}

func TestEveryClaimNeedsAnEvidenceEntryWithItsExactText(t *testing.T) {
	cases := map[string]func(*state.RepoDocState){
		"purpose": func(d *state.RepoDocState) { d.Purpose = "Something new." },
		"responsibility": func(d *state.RepoDocState) {
			d.Responsibilities = append(d.Responsibilities, "Also does a second thing")
		},
		"invariant": func(d *state.RepoDocState) { d.Invariants[0].Statement = "Reworded invariant" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			if violations := verifyWith(mutate); !hasCode(violations, CodeUnevidencedClaim) {
				t.Errorf("an unevidenced %s must be caught: %+v", name, violations)
			}
		})
	}
}

func TestCitedValuesMustExistInTheFactSheet(t *testing.T) {
	violations := verifyWith(func(d *state.RepoDocState) {
		d.Evidence[0].Fields = []string{"exports:Verify", "exports:Rollback", "imports:internal/auth", "tests:other_test.go"}
	})
	if len(violations) != 3 || !hasCode(violations, CodeUnknownCitation) || violations[0].Where != "evidence[0].fields" {
		t.Errorf("violations = %+v", violations)
	}
}

func TestACitationOfAnEmptyFieldSupportsNothing(t *testing.T) {
	sheet := ledger()
	sheet.Tests = nil
	doc := goodDoc()
	doc.Evidence[0].Fields = []string{"tests"}
	if violations := Verify(sheet, doc, ledgerVocabulary()); !hasCode(violations, CodeEmptyCitation) {
		t.Errorf("citing `tests` for a directory with none is fabricated evidence: %+v", violations)
	}
}

func TestFilesCitationAcceptsTheNameOrTheRepoRelativePath(t *testing.T) {
	violations := verifyWith(func(d *state.RepoDocState) {
		d.Evidence[0].Fields = []string{"files:verify.go", "files:internal/ledger/verify.go"}
	})
	if len(violations) != 0 {
		t.Errorf("violations = %+v", violations)
	}
}

func TestUnknownCitationRootIsCaughtEvenIfSchemaValidationWasSkipped(t *testing.T) {
	violations := verifyWith(func(d *state.RepoDocState) { d.Evidence[0].Fields = []string{"source:ledger.go"} })
	if !hasCode(violations, CodeUnknownCitation) {
		t.Errorf("%+v", violations)
	}
}

func TestInvariantEvidenceMayBeALineRangeOnlyIfTheReadIsRecorded(t *testing.T) {
	cited := func(d *state.RepoDocState) { d.Invariants[0].Evidence = []string{"internal/ledger/verify.go:10-20"} }
	if violations := verifyWith(cited); !hasCode(violations, CodeUnrecordedRead) {
		t.Errorf("a line range with no matching filesRead is unverifiable: %+v", violations)
	}
	recorded := func(d *state.RepoDocState) {
		cited(d)
		d.FilesRead = []state.DocFileRead{{Path: "verify.go", StartLine: 5, EndLine: 30, Reason: "Verify was ambiguous"}}
	}
	if violations := verifyWith(recorded); len(violations) != 0 {
		t.Errorf("a recorded read covering the range is fine: %+v", violations)
	}
	tooNarrow := func(d *state.RepoDocState) {
		cited(d)
		d.FilesRead = []state.DocFileRead{{Path: "verify.go", StartLine: 12, EndLine: 18, Reason: "r"}}
	}
	if violations := verifyWith(tooNarrow); !hasCode(violations, CodeUnrecordedRead) {
		t.Errorf("a read that does not cover the cited lines must not count: %+v", violations)
	}
}

func TestReadsMustBeOfFilesInThisDirectory(t *testing.T) {
	violations := verifyWith(func(d *state.RepoDocState) {
		d.FilesRead = []state.DocFileRead{{Path: "internal/other/x.go", StartLine: 1, EndLine: 2, Reason: "r"}, {Path: "internal/ledger/ledger.go", StartLine: 1, EndLine: 2, Reason: "r"}}
	})
	if len(violations) != 1 || violations[0].Code != CodeReadOutsideDir || violations[0].Where != "filesRead[0]" {
		t.Errorf("violations = %+v", violations)
	}
}

// The planted false claim: a dependency that is not in the fact sheet.
func TestANamedDependencyThatIsNotInTheFactSheetIsRejected(t *testing.T) {
	violations := verifyWith(func(d *state.RepoDocState) {
		d.Responsibilities = []string{"Signs entries using `internal/auth`"}
		d.Evidence = append(d.Evidence, state.DocEvidence{Claim: "Signs entries using `internal/auth`", Fields: []string{"imports"}})
	})
	if len(violations) != 1 || violations[0].Code != CodeUnknownName || violations[0].Where != "responsibilities[0]" {
		t.Errorf("violations = %+v", violations)
	}
}

func TestNamesMayBeQualifiedOrBeDirectoryBasenames(t *testing.T) {
	violations := verifyWith(func(d *state.RepoDocState) {
		d.Purpose = "Used by `loom` and wraps `telemetry` via `ledger.Verify`, `internal/ledger/ledger.go` and `store`."
		d.Evidence[0].Claim = d.Purpose
	})
	if len(violations) != 0 {
		t.Errorf("a basename, a qualified export and a repo path are all names the sheet contains: %+v", violations)
	}
}

func TestAnApproximateEdgeMayBeMentionedOnlyAsUncertain(t *testing.T) {
	flat := verifyWith(func(d *state.RepoDocState) {
		d.Purpose = "Depends on `internal/legacy` for storage."
		d.Evidence[0].Claim = d.Purpose
	})
	if !hasCode(flat, CodeApproximateAsFact) {
		t.Errorf("a heuristic edge stated as fact: %+v", flat)
	}
	hedged := verifyWith(func(d *state.RepoDocState) {
		d.Purpose = "Possibly depends on `internal/legacy` (an approximate edge)."
		d.Evidence[0].Claim = d.Purpose
	})
	if len(hedged) != 0 {
		t.Errorf("a hedged mention is allowed: %+v", hedged)
	}
}

func TestBacktickedNamesAreOnlyCheckedInClaimsNotInOpenQuestions(t *testing.T) {
	violations := verifyWith(func(d *state.RepoDocState) { d.OpenQuestions = []string{"Is `Reconcile` still used?"} })
	if len(violations) != 0 {
		t.Errorf("an open question may ask about a name the sheet lacks: %+v", violations)
	}
}

func TestVerifyReportsEveryProblemNotJustTheFirst(t *testing.T) {
	violations := verifyWith(func(d *state.RepoDocState) {
		d.Tags = []string{"audit"}
		d.KeyEntrypoints = []string{"Rollback"}
		d.Purpose = "Uses `Snapshot`."
	})
	want := map[string]bool{CodeUnknownTag: true, CodeUnknownEntrypoint: true, CodeUnevidencedClaim: true, CodeUnknownName: true}
	for _, code := range codes(violations) {
		delete(want, code)
	}
	if len(want) != 0 {
		t.Errorf("missing codes %v in %+v", want, violations)
	}
}

func TestAReadThatCoversExactlyTheCitedRangeCounts(t *testing.T) {
	violations := verifyWith(func(d *state.RepoDocState) {
		d.Invariants[0].Evidence = []string{"verify.go:10-20"}
		d.FilesRead = []state.DocFileRead{{Path: "verify.go", StartLine: 10, EndLine: 20, Reason: "r"}}
	})
	if len(violations) != 0 {
		t.Errorf("the boundaries of a read are inclusive: %+v", violations)
	}
	for name, read := range map[string]state.DocFileRead{
		"starts one line late": {Path: "verify.go", StartLine: 11, EndLine: 20, Reason: "r"},
		"ends one line early":  {Path: "verify.go", StartLine: 10, EndLine: 19, Reason: "r"},
	} {
		violations := verifyWith(func(d *state.RepoDocState) {
			d.Invariants[0].Evidence = []string{"verify.go:10-20"}
			d.FilesRead = []state.DocFileRead{read}
		})
		if !hasCode(violations, CodeUnrecordedRead) {
			t.Errorf("a read that %s does not cover the range: %+v", name, violations)
		}
	}
}
