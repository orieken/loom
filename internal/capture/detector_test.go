package capture

import (
	"reflect"
	"testing"
	"time"
)

const (
	pathP     = "ref-p"
	pathQ     = "ref-q"
	hashPre   = "hash-A"
	hashPost  = "hash-B"
	hashHuman = "hash-C"
)

func agentWrite(tool, pathRef string, when time.Time) SessionEvent {
	return NewSessionEvent("w", when, EventToolResult).
		WithTool(tool).
		WithPath(pathRef, strPtr("src/"+pathRef+".go")).
		WithHunk(HunkFacts{RangeStart: 10, RangeEnd: 14, PreHash: hashPre, PostHash: hashPost})
}

func observedHash(pathRef string, when time.Time, hash string) SessionEvent {
	return NewSessionEvent("o", when, EventHunkObserved).
		WithPath(pathRef, nil).
		WithHunk(HunkFacts{RangeStart: 10, RangeEnd: 14, PostHash: hash})
}

func bashResult(when time.Time) SessionEvent {
	return NewSessionEvent("b", when, EventToolResult).WithTool(ToolBash)
}

func deniedEdit(when time.Time) SessionEvent {
	return NewSessionEvent("d", when, EventToolDenied).WithTool(ToolEdit)
}

func promptOf(class PromptClass, when time.Time) SessionEvent {
	return NewSessionEvent("p", when, EventPrompt).WithPromptClass(class)
}

func strPtr(text string) *string { return &text }

func runDetector(cfg Config, events []SessionEvent) []Correction {
	detector := NewDetector(cfg)
	var found []Correction
	for _, event := range events {
		found = append(found, detector.Observe(event)...)
	}
	return append(found, detector.Close()...)
}

func kindsOf(corrections []Correction) []CorrectionKind {
	kinds := make([]CorrectionKind, 0, len(corrections))
	for _, correction := range corrections {
		kinds = append(kinds, correction.Kind)
	}
	return kinds
}

func withSignals(on ...Signal) Config {
	cfg := DefaultConfig()
	cfg.Signals = SignalSwitches{}
	for _, signal := range on {
		cfg.Signals[signal] = true
	}
	return cfg
}

// TestDetectorSpecificationByExample pins every row of the Specification by
// Example table in analysis.md, plus the window edge and attribution cases.
func TestDetectorSpecificationByExample(t *testing.T) {
	deniedOn := DefaultConfig()
	deniedOn.Signals[SignalDeniedCall] = true
	tests := []struct {
		name   string
		cfg    Config
		events []SessionEvent
		want   []CorrectionKind
	}{
		{"edit inside window is a hunk-edit", DefaultConfig(), []SessionEvent{
			agentWrite(ToolEdit, pathP, at(0)), observedHash(pathP, at(6*time.Minute), hashHuman),
		}, []CorrectionKind{KindHunkEdit}},
		{"edit outside window is not a correction", DefaultConfig(), []SessionEvent{
			agentWrite(ToolEdit, pathP, at(0)), observedHash(pathP, at(11*time.Minute), hashHuman),
		}, []CorrectionKind{}},
		{"edit at exactly ten minutes still counts", DefaultConfig(), []SessionEvent{
			agentWrite(ToolEdit, pathP, at(0)), observedHash(pathP, at(10*time.Minute), hashHuman),
		}, []CorrectionKind{KindHunkEdit}},
		{"edit one millisecond after ten minutes does not count", DefaultConfig(), []SessionEvent{
			agentWrite(ToolEdit, pathP, at(0)),
			observedHash(pathP, at(10*time.Minute+time.Millisecond), hashHuman),
		}, []CorrectionKind{}},
		{"lines that return to the pre-image are a revert", DefaultConfig(), []SessionEvent{
			agentWrite(ToolEdit, pathP, at(0)), observedHash(pathP, at(6*time.Minute), hashPre),
		}, []CorrectionKind{KindRevert}},
		{"bash between write and check makes the change unknown", DefaultConfig(), []SessionEvent{
			agentWrite(ToolEdit, pathP, at(0)), bashResult(at(3 * time.Minute)),
			observedHash(pathP, at(6*time.Minute), hashHuman),
		}, []CorrectionKind{KindUnknown}},
		{"bash before the write does not hide a human edit", DefaultConfig(), []SessionEvent{
			bashResult(at(-2 * time.Minute)), agentWrite(ToolEdit, pathP, at(0)),
			observedHash(pathP, at(6*time.Minute), hashHuman),
		}, []CorrectionKind{KindHunkEdit}},
		{"denied call counts when the signal is on", deniedOn, []SessionEvent{
			deniedEdit(at(0)),
		}, []CorrectionKind{KindDeniedCall}},
		{"denied call is ignored by default", DefaultConfig(), []SessionEvent{
			deniedEdit(at(0)),
		}, []CorrectionKind{}},
		{"corrective prompt counts", DefaultConfig(), []SessionEvent{
			promptOf(PromptCorrective, at(0)),
		}, []CorrectionKind{KindCorrectivePrompt}},
		{"non-corrective prompt does not count", DefaultConfig(), []SessionEvent{
			promptOf(PromptOther, at(0)),
		}, []CorrectionKind{}},
		{"agent edit from a second session explains the change", DefaultConfig(), []SessionEvent{
			agentWrite(ToolEdit, pathP, at(0)),
			NewSessionEvent("w2", at(3*time.Minute), EventToolResult).WithTool(ToolWrite).
				WithPath(pathP, nil).
				WithHunk(HunkFacts{RangeStart: 20, RangeEnd: 22, PreHash: "x", PostHash: "y"}),
			observedHash(pathP, at(5*time.Minute), hashHuman),
		}, []CorrectionKind{}},
		{"one hunk gives one correction however many checks follow", DefaultConfig(), []SessionEvent{
			agentWrite(ToolEdit, pathP, at(0)),
			observedHash(pathP, at(2*time.Minute), hashHuman),
			observedHash(pathP, at(3*time.Minute), "hash-D"),
			observedHash(pathP, at(4*time.Minute), hashPre),
		}, []CorrectionKind{KindHunkEdit}},
		{"revert after a partial edit counts once as the first correction", DefaultConfig(), []SessionEvent{
			agentWrite(ToolEdit, pathP, at(0)),
			observedHash(pathP, at(2*time.Minute), hashHuman),
			observedHash(pathP, at(4*time.Minute), hashPre),
		}, []CorrectionKind{KindHunkEdit}},
		{"an unchanged hash is not a correction", DefaultConfig(), []SessionEvent{
			agentWrite(ToolEdit, pathP, at(0)), observedHash(pathP, at(2*time.Minute), hashPost),
		}, []CorrectionKind{}},
		{"a check on another path finds no hunk", DefaultConfig(), []SessionEvent{
			agentWrite(ToolEdit, pathP, at(0)), observedHash(pathQ, at(2*time.Minute), hashHuman),
		}, []CorrectionKind{}},
		{"a hunk-edit switched off gives nothing", withSignals(SignalRevert), []SessionEvent{
			agentWrite(ToolEdit, pathP, at(0)), observedHash(pathP, at(2*time.Minute), hashHuman),
		}, []CorrectionKind{}},
		{"a revert switched off gives nothing", withSignals(SignalHunkEdit), []SessionEvent{
			agentWrite(ToolEdit, pathP, at(0)), observedHash(pathP, at(2*time.Minute), hashPre),
		}, []CorrectionKind{}},
		{"a bash result without a path never opens a hunk", DefaultConfig(), []SessionEvent{
			bashResult(at(0)), observedHash(pathP, at(2*time.Minute), hashHuman),
		}, []CorrectionKind{}},
		{"a write without a line range opens no hunk", DefaultConfig(), []SessionEvent{
			NewSessionEvent("w", at(0), EventToolResult).WithTool(ToolWrite).WithPath(pathP, nil),
			observedHash(pathP, at(2*time.Minute), hashHuman),
		}, []CorrectionKind{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kindsOf(runDetector(tt.cfg, tt.events))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("kinds = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectorCorrectionCarriesTheFactsOfTheHunk(t *testing.T) {
	events := []SessionEvent{
		agentWrite(ToolEdit, pathP, at(0)), observedHash(pathP, at(6*time.Minute), hashHuman),
	}

	got := runDetector(DefaultConfig(), events)

	want := Correction{
		Kind: KindHunkEdit, Signal: SignalHunkEdit, At: at(6 * time.Minute),
		Tool: ToolEdit, Path: strPtr("src/ref-p.go"), PathRef: pathP,
	}
	if len(got) != 1 || !reflect.DeepEqual(got[0], want) {
		t.Errorf("corrections = %+v, want [%+v]", got, want)
	}
}

func TestDetectorUnknownChangeIsNotCountable(t *testing.T) {
	events := []SessionEvent{
		agentWrite(ToolEdit, pathP, at(0)), bashResult(at(time.Minute)),
		observedHash(pathP, at(2*time.Minute), hashHuman),
	}

	got := runDetector(DefaultConfig(), events)

	if len(got) != 1 || got[0].IsCountable() {
		t.Errorf("corrections = %+v, want one that is not countable", got)
	}
	if got[0].Signal != SignalHunkEdit {
		t.Errorf("signal = %q, want hunk-edit", got[0].Signal)
	}
}

func TestDetectorRevertAndEditAreCountable(t *testing.T) {
	for _, kind := range []CorrectionKind{KindHunkEdit, KindRevert, KindDeniedCall, KindCorrectivePrompt} {
		if !(Correction{Kind: kind}).IsCountable() {
			t.Errorf("kind %q must be countable", kind)
		}
	}
}

func TestDetectorForgetsHunksAfterTheWindow(t *testing.T) {
	detector := NewDetector(DefaultConfig())
	detector.Observe(agentWrite(ToolEdit, pathP, at(0)))
	if detector.OpenHunks() != 1 {
		t.Fatalf("open hunks = %d, want 1", detector.OpenHunks())
	}

	detector.Observe(promptOf(PromptOther, at(10*time.Minute)))
	if detector.OpenHunks() != 1 {
		t.Errorf("open hunks at the edge = %d, want 1", detector.OpenHunks())
	}

	detector.Observe(promptOf(PromptOther, at(10*time.Minute+time.Second)))
	if detector.OpenHunks() != 0 {
		t.Errorf("open hunks after the window = %d, want 0", detector.OpenHunks())
	}
}

func TestDetectorKeepsMemoryProportionalToOpenHunks(t *testing.T) {
	detector := NewDetector(DefaultConfig())
	for minute := 0; minute < 500; minute++ {
		detector.Observe(agentWrite(ToolEdit, pathP+"-"+string(rune('a'+minute%26)), at(time.Duration(minute)*time.Minute)))
	}

	if got := detector.OpenHunks(); got > 11 {
		t.Errorf("open hunks = %d after 500 writes, want at most 11 (one window)", got)
	}
}

func TestDetectorCloseForgetsOpenHunks(t *testing.T) {
	detector := NewDetector(DefaultConfig())
	detector.Observe(agentWrite(ToolEdit, pathP, at(0)))

	flushed := detector.Close()

	if len(flushed) != 0 || detector.OpenHunks() != 0 {
		t.Errorf("Close left %d flushed and %d open hunks", len(flushed), detector.OpenHunks())
	}
}

func TestDetectorIsDeterministic(t *testing.T) {
	events := []SessionEvent{
		agentWrite(ToolEdit, pathP, at(0)), agentWrite(ToolWrite, pathQ, at(time.Minute)),
		promptOf(PromptCorrective, at(2*time.Minute)),
		observedHash(pathQ, at(3*time.Minute), hashPre),
		observedHash(pathP, at(4*time.Minute), hashHuman),
		bashResult(at(5 * time.Minute)),
	}

	first := runDetector(DefaultConfig(), events)
	second := runDetector(DefaultConfig(), events)

	if len(first) != 3 || !reflect.DeepEqual(first, second) {
		t.Errorf("runs differ or wrong count:\nfirst  %+v\nsecond %+v", first, second)
	}
}

func TestDetectorRedactedPathOpensNoHunk(t *testing.T) {
	assembler := newTestAssembler(t, "/proj")
	write, err := assembler.Assemble(Observation{
		Kind: EventToolResult, Tool: ToolWrite, FilePath: "/proj/.env.local",
		Hunk: HunkFacts{RangeStart: 1, RangeEnd: 3, PreHash: hashPre, PostHash: hashPost},
	})
	if err != nil {
		t.Fatal(err)
	}
	check, err := assembler.Assemble(Observation{
		Kind: EventHunkObserved, FilePath: "/proj/.env.local",
		Hunk: HunkFacts{RangeStart: 1, RangeEnd: 3, PostHash: hashHuman},
	})
	if err != nil {
		t.Fatal(err)
	}
	detector := NewDetector(DefaultConfig())

	found := append(detector.Observe(write), detector.Observe(check)...)

	if len(found) != 0 || detector.OpenHunks() != 0 {
		t.Errorf("redacted path gave %d corrections and %d open hunks", len(found), detector.OpenHunks())
	}
}
