package repodoc

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/contextindex"
	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
	"github.com/orieken/loom/internal/state"
)

func TestRunnerDocumentsBottomUpAndShowsParentsTheirChildrensSummaries(t *testing.T) {
	provider := scriptedProvider(t, tree())
	record, err := newRunner(provider, t.TempDir()).Run(context.Background(), tasksFor(tree()))
	if err != nil || record.Failed() != 0 {
		t.Fatalf("run: %+v %v", record, err)
	}
	want := []string{stageID("svc/api"), stageID("svc/store"), stageID("svc"), stageID(".")}
	if !reflect.DeepEqual(provider.Invocations(), want) {
		t.Errorf("invocation order = %v, want %v", provider.Invocations(), want)
	}
	parent := provider.InputFor(stageID("svc")).UpstreamState[inputChildSummaries]
	if !strings.Contains(string(parent), "does svc/api") || !strings.Contains(string(parent), "does svc/store") {
		t.Errorf("the parent must be shown what its children were documented as:\n%s", parent)
	}
}

func TestRunnerWritesACandidatePerDirectoryAndARunRecord(t *testing.T) {
	root := t.TempDir()
	record, err := newRunner(scriptedProvider(t, tree()), root).Run(context.Background(), tasksFor(tree()))
	if err != nil {
		t.Fatal(err)
	}
	runDir := RunDir(root, record.RunID)
	for _, dir := range []string{"svc/api", "svc", "."} {
		if _, err := os.Stat(CandidatePath(runDir, dir)); err != nil {
			t.Errorf("candidate for %s missing: %v", dir, err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(runDir, "run.json"))
	if err != nil || !strings.Contains(string(raw), `"agent": "repo-documenter"`) || record.RunID != "20261005T120000Z" {
		t.Errorf("run record: %v %s id=%s", err, raw, record.RunID)
	}
}

func TestRunnerTouchesNoIndex(t *testing.T) {
	root := t.TempDir()
	if _, err := newRunner(scriptedProvider(t, tree()), root).Run(context.Background(), tasksFor(tree())); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := contextindex.Load(root); found {
		t.Error("a documenter run must not create or alter the index: results are unverified candidates")
	}
}

func TestAFailedChildIsRecordedAndItsParentIsToldItIsUndocumented(t *testing.T) {
	provider := scriptedProvider(t, tree())
	provider.SetScript(stageID("svc/api"), mock.Script{Err: errors.New("model unavailable")})
	record, _ := newRunner(provider, t.TempDir()).Run(context.Background(), tasksFor(tree()))
	if record.Failed() != 1 || record.Outcomes[0].Status != StatusFailed || !strings.Contains(record.Outcomes[0].Error, "model unavailable") {
		t.Fatalf("outcomes = %+v", record.Outcomes)
	}
	if len(record.Outcomes) != 4 {
		t.Errorf("one failure must not stop the run: %d outcomes", len(record.Outcomes))
	}
	parent := string(provider.InputFor(stageID("svc")).UpstreamState[inputChildSummaries])
	if !strings.Contains(parent, `"status": "undocumented"`) {
		t.Errorf("the parent must be told svc/api is undocumented:\n%s", parent)
	}
}

func TestRunnerRejectsOutputThatDocumentsTheWrongDirectoryOrVersion(t *testing.T) {
	cases := map[string]state.RepoDocState{
		"wrong path": validDoc("svc/store", "h-api"),
		"wrong hash": validDoc("svc/api", "h-old"),
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			provider := mock.New(map[string]mock.Script{stageID("svc/api"): {Payload: encode(t, doc)}})
			record, _ := newRunner(provider, t.TempDir()).Run(context.Background(), tasksFor(tree()[2:3]))
			if record.Failed() != 1 {
				t.Fatalf("%s must be rejected: %+v", name, record.Outcomes)
			}
		})
	}
}

func TestRunnerRejectsAPayloadThatFailsTheSchema(t *testing.T) {
	invalid := validDoc("svc/api", "h-api")
	invalid.Layer = "mystery"
	for name, payload := range map[string][]byte{"invalid layer": encode(t, invalid), "not json": []byte("prose, not json"), "unknown field": []byte(`{"extra": 1}`)} {
		t.Run(name, func(t *testing.T) {
			provider := mock.New(map[string]mock.Script{stageID("svc/api"): {Payload: payload}})
			root := t.TempDir()
			record, _ := newRunner(provider, root).Run(context.Background(), tasksFor(tree()[2:3]))
			if record.Failed() != 1 {
				t.Fatalf("outcomes = %+v", record.Outcomes)
			}
			if _, err := os.Stat(CandidatePath(RunDir(root, record.RunID), "svc/api")); err == nil {
				t.Error("a rejected payload must not be stored as a candidate")
			}
		})
	}
}

func TestRunnerTotalsReportedUsageAndCountsRecordedSourceReads(t *testing.T) {
	doc := validDoc("svc/api", "h-api")
	doc.FilesRead = []state.DocFileRead{{Path: "svc/api/x.go", StartLine: 1, EndLine: 9, Reason: "ambiguous export"}}
	provider := mock.New(map[string]mock.Script{
		stageID("svc/api"):   {Payload: encode(t, doc), Usage: &orchestrator.Usage{InputTokens: 100, CostUSD: 0.25}},
		stageID("svc/store"): {Payload: encode(t, validDoc("svc/store", "h-store")), Usage: &orchestrator.Usage{InputTokens: 50, CostUSD: 0.5}},
	})
	record, _ := newRunner(provider, t.TempDir()).Run(context.Background(), tasksFor(tree()[2:]))
	if record.TotalUsage == nil || record.TotalUsage.InputTokens != 150 || record.TotalUsage.CostUSD != 0.75 {
		t.Errorf("total usage = %+v", record.TotalUsage)
	}
	if record.Outcomes[0].FilesRead != 1 || record.Outcomes[1].FilesRead != 0 {
		t.Errorf("files read = %d, %d", record.Outcomes[0].FilesRead, record.Outcomes[1].FilesRead)
	}
}

func TestRunnerReportsNoTotalWhenNothingReportedUsage(t *testing.T) {
	record, _ := newRunner(scriptedProvider(t, tree()), t.TempDir()).Run(context.Background(), tasksFor(tree()))
	if record.TotalUsage != nil {
		t.Errorf("unreported usage is unmeasured, not zero: %+v", record.TotalUsage)
	}
}

func TestRunnerStopsWhenCancelledAndSaysSo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	provider := scriptedProvider(t, tree())
	cancelAfterFirst := 0
	provider.SetHook(func(string, int) *mock.Script {
		cancelAfterFirst++
		if cancelAfterFirst == 1 {
			cancel()
		}
		return nil
	})
	root := t.TempDir()
	record, err := newRunner(provider, root).Run(ctx, tasksFor(tree()))
	if err != nil || !record.Interrupted || len(record.Outcomes) >= 4 {
		t.Fatalf("interrupted=%v outcomes=%d err=%v", record.Interrupted, len(record.Outcomes), err)
	}
	if _, statErr := os.Stat(filepath.Join(RunDir(root, record.RunID), "run.json")); statErr != nil {
		t.Errorf("an interrupted run must still leave its record: %v", statErr)
	}
}

func TestRunRecordSerializesWithoutEmptyUsage(t *testing.T) {
	raw, err := json.Marshal(RunRecord{RunID: "r", Outcomes: []Outcome{{Path: "a", Status: StatusDocumented}}})
	if err != nil || strings.Contains(string(raw), "usage") || strings.Contains(string(raw), "totalUsage") {
		t.Errorf("%s %v", raw, err)
	}
}

func TestStageIDsAreFilesystemSafeAndInjective(t *testing.T) {
	ids := map[string]string{}
	for _, dir := range []string{".", "svc/api", "svc__api", "svc_api", "svc/_api", "_root", "root", "svc"} {
		id := stageID(dir)
		if strings.Contains(id, "/") {
			t.Errorf("%q yields a path separator: %q", dir, id)
		}
		if other, clash := ids[id]; clash {
			t.Errorf("%q and %q share the stage id %q", dir, other, id)
		}
		ids[id] = dir
	}
	if stageID(".") != "repo-doc-_root" || stageID("svc/api") != "repo-doc-svc_sapi" {
		t.Errorf("%q %q", stageID("."), stageID("svc/api"))
	}
}

func TestCandidatePathMirrorsTheDirectory(t *testing.T) {
	if got := CandidatePath("/r", "svc/api"); got != filepath.Join("/r", "docs", "svc/api.json") {
		t.Error(got)
	}
	if got := CandidatePath("/r", "."); got != filepath.Join("/r", "docs", "_root.json") {
		t.Error(got)
	}
}

// deadlineProvider records the deadline each invocation was given.
type deadlineProvider struct {
	inner     orchestrator.Provider
	deadlines []time.Duration
	timeouts  []time.Duration
}

func (p *deadlineProvider) Invoke(ctx context.Context, stage orchestrator.Stage, input orchestrator.StageInput) (orchestrator.StageOutput, error) {
	if deadline, ok := ctx.Deadline(); ok {
		p.deadlines = append(p.deadlines, time.Until(deadline))
	}
	p.timeouts = append(p.timeouts, stage.Timeout)
	return p.inner.Invoke(ctx, stage, input)
}

func runWithTimeout(t *testing.T, configured time.Duration) *deadlineProvider {
	t.Helper()
	sheets := tree()[2:3]
	provider := &deadlineProvider{inner: scriptedProvider(t, sheets)}
	runner := newRunner(provider, t.TempDir())
	runner.Timeout = configured
	if record, _ := runner.Run(context.Background(), tasksFor(sheets)); record.Failed() != 0 {
		t.Fatalf("a bounded invocation must still succeed: %+v", record.Outcomes)
	}
	return provider
}

func assertBoundedBy(t *testing.T, provider *deadlineProvider, want time.Duration) {
	t.Helper()
	if len(provider.deadlines) != 1 || provider.timeouts[0] != want {
		t.Fatalf("deadlines %v timeouts %v, want %v", provider.deadlines, provider.timeouts, want)
	}
	if remaining := provider.deadlines[0]; remaining <= want-10*time.Second || remaining > want {
		t.Errorf("deadline %v is not about %v away", remaining, want)
	}
}

func TestAnInvocationWithNoConfiguredTimeoutGetsTheDefault(t *testing.T) {
	assertBoundedBy(t, runWithTimeout(t, 0), DefaultTimeout)
}

func TestAnInvocationGetsTheConfiguredTimeout(t *testing.T) {
	assertBoundedBy(t, runWithTimeout(t, 90*time.Second), 90*time.Second)
}
