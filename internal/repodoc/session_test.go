package repodoc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
	"github.com/orieken/loom/internal/state"
)

// leafOnly is a one-directory run: svc/api.
func leafOnly() []Task { return tasksFor(tree()[2:3]) }

func apiDoc(mutate func(*state.RepoDocState)) []byte {
	doc := validDoc("svc/api", "h-api")
	if mutate != nil {
		mutate(&doc)
	}
	raw, _ := jsonMarshal(doc)
	return raw
}

// scriptedRounds answers the documenter with a different payload each round
// and the auditor with a different verdict each round; the last one repeats.
func scriptedRounds(t *testing.T, documents [][]byte, audits []state.RepoDocAuditState) *mock.Provider {
	t.Helper()
	provider := mock.New(nil)
	provider.SetHook(func(stage string, visit int) *mock.Script {
		switch stage {
		case stageID("svc/api"):
			return &mock.Script{Payload: documents[min(visit, len(documents))-1]}
		case auditStageID("svc/api"):
			return &mock.Script{Payload: encodeAudit(t, audits[min(visit, len(audits))-1])}
		}
		return nil
	})
	return provider
}

func runLeaf(t *testing.T, provider *mock.Provider, configure func(*Runner)) (RunRecord, string) {
	t.Helper()
	root := t.TempDir()
	runner := newRunner(provider, root)
	if configure != nil {
		configure(&runner)
	}
	record, err := runner.Run(context.Background(), leafOnly())
	if err != nil {
		t.Fatal(err)
	}
	return record, root
}

func TestAVerifierViolationIsFedBackAndTheNextRoundCanVerify(t *testing.T) {
	bad := apiDoc(func(d *state.RepoDocState) { d.Tags = []string{"audit"} })
	provider := scriptedRounds(t, [][]byte{bad, apiDoc(nil)}, []state.RepoDocAuditState{approval("svc/api", "h-api")})
	record, _ := runLeaf(t, provider, nil)
	outcome := record.Outcomes[0]
	if outcome.Status != StatusVerified || len(outcome.Rounds) != 2 {
		t.Fatalf("outcome = %+v", outcome)
	}
	if outcome.Rounds[0].Violations[0].Code != CodeUnknownTag || outcome.Rounds[0].Audited {
		t.Errorf("round 1 must stop at the cheap check, before the auditor: %+v", outcome.Rounds[0])
	}
	if !outcome.Rounds[1].Audited || outcome.Rounds[1].Verdict != "APPROVED" {
		t.Errorf("round 2 = %+v", outcome.Rounds[1])
	}
}

func TestTheSecondRoundIsShownThePreviousAttemptAndTheTypedFinding(t *testing.T) {
	bad := apiDoc(func(d *state.RepoDocState) { d.Tags = []string{"audit"} })
	provider := scriptedRounds(t, [][]byte{bad, apiDoc(nil)}, []state.RepoDocAuditState{approval("svc/api", "h-api")})
	runLeaf(t, provider, nil)
	feedback := string(provider.InputFor(stageID("svc/api")).UpstreamState[inputFeedback])
	for _, want := range []string{`"round": 2`, `"code": "unknown-tag"`, `"previous"`, "audit"} {
		if !strings.Contains(feedback, want) {
			t.Errorf("round 2 was not shown %q:\n%s", want, feedback)
		}
	}
}

func TestTheFirstRoundGetsNoFeedbackBlock(t *testing.T) {
	provider := scriptedRounds(t, [][]byte{apiDoc(nil)}, []state.RepoDocAuditState{approval("svc/api", "h-api")})
	runLeaf(t, provider, nil)
	if _, present := provider.InputFor(stageID("svc/api")).UpstreamState[inputFeedback]; present {
		t.Error("a first attempt has nothing to revise")
	}
}

func TestAnAuditorRejectionIsFedBackAsATypedFinding(t *testing.T) {
	provider := scriptedRounds(t, [][]byte{apiDoc(nil), apiDoc(nil)},
		[]state.RepoDocAuditState{rejection("svc/api", "h-api"), approval("svc/api", "h-api")})
	record, _ := runLeaf(t, provider, nil)
	outcome := record.Outcomes[0]
	if outcome.Status != StatusVerified || outcome.Rounds[0].Verdict != "CHANGES_REQUESTED" {
		t.Fatalf("outcome = %+v", outcome)
	}
	feedback := string(provider.InputFor(stageID("svc/api")).UpstreamState[inputFeedback])
	if !strings.Contains(feedback, `"source": "auditor"`) || !strings.Contains(feedback, "the exports do not say that") {
		t.Errorf("the auditor's finding must reach the documenter:\n%s", feedback)
	}
}

func TestANonBlockingAuditSuggestionDoesNotCostARound(t *testing.T) {
	suggestion := approval("svc/api", "h-api")
	suggestion.Findings = []state.DocAuditFinding{{Claim: "c", Problem: "p", Instruction: "i"}}
	record, _ := runLeaf(t, scriptedRounds(t, [][]byte{apiDoc(nil)}, []state.RepoDocAuditState{suggestion}), nil)
	if outcome := record.Outcomes[0]; outcome.Status != StatusVerified || len(outcome.Rounds) != 1 {
		t.Errorf("outcome = %+v", outcome)
	}
}

func neverVerifies(t *testing.T) (RunRecord, string) {
	t.Helper()
	bad := apiDoc(func(d *state.RepoDocState) { d.Tags = []string{"audit"} })
	return runLeaf(t, scriptedRounds(t, [][]byte{bad}, nil), nil)
}

func TestADirectoryThatNeverVerifiesStopsAtTheRoundLimit(t *testing.T) {
	record, _ := neverVerifies(t)
	outcome := record.Outcomes[0]
	if outcome.Status != StatusUnverified || len(outcome.Rounds) != DefaultMaxRounds || record.Unverified() != 1 {
		t.Errorf("outcome = %+v", outcome)
	}
}

func TestAnUnverifiedDocumentIsKeptApartAndNeverAcceptedAsACandidate(t *testing.T) {
	record, root := neverVerifies(t)
	runDir := RunDir(root, record.RunID)
	if _, err := os.Stat(CandidatePath(runDir, "svc/api")); err == nil {
		t.Error("an unverified document must never be stored as a candidate")
	}
	if _, err := os.Stat(UnverifiedPath(runDir, "svc/api")); err != nil {
		t.Errorf("the last attempt is kept for a human: %v", err)
	}
}

func TestEveryRoundIsRetained(t *testing.T) {
	record, root := neverVerifies(t)
	for round := 1; round <= DefaultMaxRounds; round++ {
		if _, err := os.Stat(RoundPath(RunDir(root, record.RunID), "svc/api", round)); err != nil {
			t.Errorf("round %d was not retained: %v", round, err)
		}
	}
}

func TestMaxRoundsIsConfigurable(t *testing.T) {
	bad := apiDoc(func(d *state.RepoDocState) { d.Tags = []string{"audit"} })
	record, _ := runLeaf(t, scriptedRounds(t, [][]byte{bad}, nil), func(r *Runner) { r.MaxRounds = 1 })
	if outcome := record.Outcomes[0]; len(outcome.Rounds) != 1 || outcome.Status != StatusUnverified {
		t.Errorf("outcome = %+v", outcome)
	}
}

func TestSkipAuditRunsOnlyTheDeterministicVerifier(t *testing.T) {
	provider := scriptedRounds(t, [][]byte{apiDoc(nil)}, nil)
	record, _ := runLeaf(t, provider, func(r *Runner) { r.SkipAudit = true })
	for _, id := range provider.Invocations() {
		if strings.HasPrefix(id, "repo-doc-audit-") {
			t.Errorf("the auditor ran despite SkipAudit: %v", provider.Invocations())
		}
	}
	if outcome := record.Outcomes[0]; outcome.Status != StatusVerified || outcome.Rounds[0].Audited {
		t.Errorf("outcome = %+v", outcome)
	}
}

// The planted false claim from the plan: the verifier rejects it before any
// auditor is spent on it.
func TestAPlantedFalseDependencyNeverVerifies(t *testing.T) {
	planted := apiDoc(func(d *state.RepoDocState) {
		d.Responsibilities = []string{"Authenticates callers through `internal/auth`"}
		d.Evidence = append(d.Evidence, state.DocEvidence{Claim: d.Responsibilities[0], Fields: []string{"exports"}})
	})
	provider := scriptedRounds(t, [][]byte{planted}, []state.RepoDocAuditState{approval("svc/api", "h-api")})
	record, _ := runLeaf(t, provider, nil)
	outcome := record.Outcomes[0]
	if outcome.Status != StatusUnverified || outcome.Rounds[0].Violations[0].Code != CodeUnknownName {
		t.Fatalf("outcome = %+v", outcome)
	}
	for _, id := range provider.Invocations() {
		if strings.HasPrefix(id, "repo-doc-audit-") {
			t.Error("a document the verifier rejects must not reach the auditor")
		}
	}
}

func TestAnUnverifiedChildIsPresentedToItsParentAsUndocumented(t *testing.T) {
	provider := scriptedProvider(t, tree())
	bad := validDoc("svc/api", "h-api")
	bad.Tags = []string{"audit"}
	provider.SetScript(stageID("svc/api"), mock.Script{Payload: encode(t, bad)})
	record, _ := newRunner(provider, t.TempDir()).Run(context.Background(), tasksFor(tree()))
	if record.Unverified() != 1 {
		t.Fatalf("outcomes = %+v", record.Outcomes)
	}
	parent := string(provider.InputFor(stageID("svc")).UpstreamState[inputChildSummaries])
	if strings.Contains(parent, "does svc/api") || !strings.Contains(parent, `"status": "undocumented"`) {
		t.Errorf("a parent must not build on unverified text:\n%s", parent)
	}
}

func TestAnAuditorProviderErrorFailsTheDirectoryAndSaysWhy(t *testing.T) {
	provider := scriptedRounds(t, [][]byte{apiDoc(nil)}, nil)
	provider.SetHook(nil)
	provider.SetScript(stageID("svc/api"), mock.Script{Payload: apiDoc(nil)})
	provider.SetScript(auditStageID("svc/api"), mock.Script{Err: errors.New("auditor down")})
	record, _ := runLeaf(t, provider, nil)
	if outcome := record.Outcomes[0]; outcome.Status != StatusFailed || !strings.HasPrefix(outcome.Error, "audit:") || !strings.Contains(outcome.Error, "auditor down") {
		t.Errorf("outcome = %+v", outcome)
	}
}

func TestAnAuditOfTheWrongDirectoryOrVersionFailsTheDirectory(t *testing.T) {
	for name, wrong := range map[string]state.RepoDocAuditState{
		"wrong path": approval("svc/store", "h-api"), "wrong hash": approval("svc/api", "h-old"),
	} {
		t.Run(name, func(t *testing.T) {
			provider := mock.New(map[string]mock.Script{
				stageID("svc/api"):      {Payload: apiDoc(nil)},
				auditStageID("svc/api"): {Payload: encodeAudit(t, wrong)},
			})
			record, _ := runLeaf(t, provider, nil)
			if record.Outcomes[0].Status != StatusFailed {
				t.Errorf("outcome = %+v", record.Outcomes[0])
			}
		})
	}
}

func TestUsageIsTotalledAcrossEveryRoundAndTheAuditor(t *testing.T) {
	bad := apiDoc(func(d *state.RepoDocState) { d.Tags = []string{"audit"} })
	provider := mock.New(nil)
	provider.SetHook(func(stage string, visit int) *mock.Script {
		usage := &orchestrator.Usage{InputTokens: 10, CostUSD: 1}
		switch stage {
		case stageID("svc/api"):
			payload := bad
			if visit == 2 {
				payload = apiDoc(nil)
			}
			return &mock.Script{Payload: payload, Usage: usage}
		case auditStageID("svc/api"):
			return &mock.Script{Payload: encodeAudit(t, approval("svc/api", "h-api")), Usage: usage}
		}
		return nil
	})
	record, _ := runLeaf(t, provider, nil)
	if record.TotalUsage == nil || record.TotalUsage.InputTokens != 30 || record.TotalUsage.CostUSD != 3 {
		t.Errorf("two documenter calls and one audit: %+v", record.TotalUsage)
	}
}

func TestRoundAndUnverifiedPathsMirrorTheDirectory(t *testing.T) {
	if got := RoundPath("/r", "svc/api", 2); got != filepath.Join("/r", "rounds", "svc/api.json", "round-2.json") {
		t.Error(got)
	}
	if got := UnverifiedPath("/r", "."); got != filepath.Join("/r", "unverified", "_root.json") {
		t.Error(got)
	}
	if auditStageID("svc/api") != "repo-doc-audit-svc_sapi" {
		t.Error(auditStageID("svc/api"))
	}
}

func TestTheAuditorIsGivenTheCandidateButNoTagsAndNoFeedback(t *testing.T) {
	task := Task{Path: "svc/api", Sheet: tree()[2]}
	input, err := auditInput(task, summaries{thisRun: map[string]state.RepoDocState{}, recorded: emptyIndex()}, newVocabulary([]Tag{{Name: "api"}}), validDoc("svc/api", "h-api"))
	if err != nil {
		t.Fatal(err)
	}
	if _, has := input[inputAllowedTags]; has {
		t.Error("the auditor judges support, not tag choice: it needs no vocabulary")
	}
	if _, has := input[inputFeedback]; has {
		t.Error("each audit is fresh: it is told nothing about earlier rounds")
	}
	if !strings.Contains(string(input[inputCandidate]), "does svc/api") || len(input[inputFactSheet]) == 0 || len(input[inputChildSummaries]) == 0 {
		t.Errorf("candidate, fact sheet and child summaries are required: %v", input)
	}
}
