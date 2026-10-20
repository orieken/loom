package capture

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPromptClassifier(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
		want   PromptClass
	}{
		{"opens with no comma", "no, use the other file", PromptCorrective},
		{"capital and leading space", "  No, use the other file", PromptCorrective},
		{"opens with dont", "don't do that", PromptCorrective},
		{"curly apostrophe", "Don’t do that", PromptCorrective},
		{"note that is not corrective", "note that the file moved", PromptOther},
		{"no without comma is other", "no problem", PromptOther},
		{"nothing is other", "nothing to fix", PromptOther},
		{"no later in the text is other", "please say no, thanks", PromptOther},
		{"empty prompt", "", PromptOther},
		{"known false positive stays corrective", "no, problem, continue", PromptCorrective},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (PromptClassifier{}).Classify(tt.prompt); got != tt.want {
				t.Errorf("Classify(%q) = %q, want %q", tt.prompt, got, tt.want)
			}
		})
	}
}

func TestSafeNames(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		isKeyOK  bool
		isToolOK bool
	}{
		{"file_path", "file_path", true, true},
		{"leading underscore", "_x", true, true},
		{"digit first", "9a", false, false},
		{"empty", "", false, false},
		{"64 characters", strings.Repeat("a", 64), true, true},
		{"65 characters", strings.Repeat("a", 65), false, false},
		{"hyphen", "a-b", false, true},
		{"dot and colon", "a.b:c", false, true},
		{"hyphen first", "-x", false, false},
		{"space", "a b", false, false},
		{"non-ASCII", "café", false, false},
		{"slash", "a/b", false, false},
		{"digits after the first", "a09", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSafeKeyName(tt.input); got != tt.isKeyOK {
				t.Errorf("IsSafeKeyName(%q) = %v, want %v", tt.input, got, tt.isKeyOK)
			}
			if got := IsSafeToolName(tt.input); got != tt.isToolOK {
				t.Errorf("IsSafeToolName(%q) = %v, want %v", tt.input, got, tt.isToolOK)
			}
		})
	}
}

func TestSafeToolNameReplacesAnUnsafeName(t *testing.T) {
	if got := SafeToolName("Edit"); got != "Edit" {
		t.Errorf("SafeToolName(Edit) = %q", got)
	}
	if got := SafeToolName("rm -rf /; echo CANARY"); got != UnsafeNamePlaceholder {
		t.Errorf("SafeToolName(unsafe) = %q, want %q", got, UnsafeNamePlaceholder)
	}
}

func TestToolCatalogFileWriters(t *testing.T) {
	for _, tool := range []string{ToolEdit, ToolWrite, ToolMultiEdit} {
		if !IsFileWriter(tool) || IsShell(tool) {
			t.Errorf("%s must be a file writer and not a shell", tool)
		}
	}
}

func TestToolCatalogShellAndOthers(t *testing.T) {
	if IsFileWriter(ToolBash) || !IsShell(ToolBash) {
		t.Error("Bash must be a shell and not a file writer")
	}
	if IsFileWriter("Read") || IsShell("Read") {
		t.Error("Read must be neither")
	}
}

func TestLayoutPaths(t *testing.T) {
	layout := NewLayout("/proj")
	tests := map[string]string{
		layout.SessionsDir():       "/proj/.claude/memory/sessions",
		layout.SaltPath():          "/proj/.claude/memory/capture.salt",
		layout.FailuresPath():      "/proj/.claude/memory/capture.failures",
		layout.PurgeStampPath():    "/proj/.claude/memory/capture.purge",
		layout.DatabasePath():      "/proj/.claude/memory/episodes.db",
		layout.ConfigPath():        "/proj/.claude/capture.yaml",
		layout.SettingsLocalPath(): "/proj/.claude/settings.local.json",
		layout.Root():              "/proj",
	}
	for got, want := range tests {
		if got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	}
}

func TestLayoutDayFileUsesTheUTCDate(t *testing.T) {
	layout := NewLayout("/proj")
	plusFive := time.FixedZone("UTC+5", 5*3600)
	tests := []struct {
		name string
		at   time.Time
		want string
	}{
		{"just before midnight UTC", time.Date(2026, 10, 18, 23, 59, 59, 900_000_000, time.UTC), "2026-10-18.jsonl"},
		{"just after midnight UTC", time.Date(2026, 10, 19, 0, 0, 0, 100_000_000, time.UTC), "2026-10-19.jsonl"},
		{"local zone ahead of UTC", time.Date(2026, 10, 19, 2, 0, 0, 0, plusFive), "2026-10-18.jsonl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := filepath.Join("/proj/.claude/memory/sessions", tt.want)
			if got := layout.DayFile(tt.at); got != want {
				t.Errorf("DayFile = %q, want %q", got, want)
			}
		})
	}
}

// TestPathLiteralsLiveInOneFile is the fitness function of decision 12.
func TestPathLiteralsLiveInOneFile(t *testing.T) {
	for _, literal := range []string{"capture.salt", "capture.failures", "capture.purge"} {
		if files := nonTestFilesContaining(t, literal); len(files) != 1 || files[0] != "layout.go" {
			t.Errorf("%q appears in %v, want layout.go only", literal, files)
		}
	}
}

func nonTestFilesContaining(t *testing.T, literal string) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, entry := range entries {
		if isDomainSource(entry.Name()) && fileContains(t, entry.Name(), literal) {
			found = append(found, entry.Name())
		}
	}
	return found
}

func isDomainSource(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

func fileContains(t *testing.T, name, literal string) bool {
	t.Helper()
	body, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Contains(string(body), literal)
}

func TestLimitsArePinned(t *testing.T) {
	tests := []struct {
		name string
		got  int64
		want int64
	}{
		{"HookStopAfter ms", HookStopAfter.Milliseconds(), 500},
		{"CounterWriteWait ms", CounterWriteWait.Milliseconds(), 50},
		{"StallWindow s", int64(StallWindow.Seconds()), 60},
		{"MaxStdinBytes", MaxStdinBytes, 4 << 20},
		{"MaxReadBudgetBytes", MaxReadBudgetBytes, 4 << 20},
		{"MaxTargetFileBytes", MaxTargetFileBytes, 1 << 20},
		{"MaxTailBytes", MaxTailBytes, 256 << 10},
		{"TailChunkBytes", TailChunkBytes, 32 << 10},
		{"MaxRehashedHunks", MaxRehashedHunks, 20},
		{"MaxBatchBytes", MaxBatchBytes, 16 << 10},
		{"MaxLineBytes", MaxLineBytes, 4096},
		{"MaxPathBytes", MaxPathBytes, 512},
		{"MaxArgKeys", MaxArgKeys, 16},
		{"MaxNameBytes", MaxNameBytes, 64},
		{"PurgeInterval min", int64(PurgeInterval.Minutes()), 60},
		{"ClockAnomalyThreshold s", int64(ClockAnomalyThreshold.Seconds()), 5},
		{"DefaultCorrectionWindow min", int64(DefaultCorrectionWindow.Minutes()), 10},
		{"StallCause", int64(StallCause), int64('t')},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Window != 10*time.Minute {
		t.Errorf("Window = %v, want 10m", cfg.Window)
	}
	for signal, want := range map[Signal]bool{
		SignalHunkEdit: true, SignalRevert: true, SignalCorrectivePrompt: true, SignalDeniedCall: false,
	} {
		if cfg.Signals.IsOn(signal) != want {
			t.Errorf("signal %s on = %v, want %v", signal, !want, want)
		}
	}
	if (SignalSwitches{}).IsOn(SignalRevert) {
		t.Error("a signal that is not listed must be off")
	}
}

func TestMinutesEstimator(t *testing.T) {
	estimator := NewMinutesEstimator(DefaultWeights())
	tests := []struct {
		kind  CorrectionKind
		count int
		want  int
	}{
		{KindHunkEdit, 3, 6},
		{KindRevert, 2, 6},
		{KindDeniedCall, 4, 4},
		{KindCorrectivePrompt, 5, 5},
		{KindUnknown, 9, 0},
		{CorrectionKind("other"), 9, 0},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			if got := estimator.EstimateKind(tt.kind, tt.count); got != tt.want {
				t.Errorf("EstimateKind(%s, %d) = %d, want %d", tt.kind, tt.count, got, tt.want)
			}
		})
	}
}

func TestMinutesEstimatorTotalSkipsUnknownAndUsesConfiguredWeights(t *testing.T) {
	counts := map[CorrectionKind]int{KindHunkEdit: 2, KindRevert: 1, KindUnknown: 10}

	defaults := NewMinutesEstimator(DefaultWeights()).EstimateTotal(counts)
	custom := NewMinutesEstimator(Weights{HunkEdit: 10, Revert: 20}).EstimateTotal(counts)

	if defaults != 7 || custom != 40 {
		t.Errorf("totals = %d and %d, want 7 and 40", defaults, custom)
	}
}

func TestMinutesMethodNamesTheEstimate(t *testing.T) {
	got := NewMinutesEstimator(DefaultWeights()).Method()

	if got != "fixed weight per kind (estimate)" {
		t.Errorf("Method() = %q", got)
	}
}

func TestDefaultWeights(t *testing.T) {
	want := Weights{HunkEdit: 2, Revert: 3, DeniedCall: 1, CorrectivePrompt: 1}

	if DefaultWeights() != want {
		t.Errorf("DefaultWeights() = %+v, want %+v", DefaultWeights(), want)
	}
}

func TestRandomIDIsHexAndUnique(t *testing.T) {
	first, err1 := RandomID()
	second, err2 := RandomID()

	if err1 != nil || err2 != nil || len(first) != 16 || first == second {
		t.Errorf("ids = %q and %q (errors %v, %v)", first, second, err1, err2)
	}
	assertLowercaseHex(t, first)
	assertLowercaseHex(t, second)
}

// assertLowercaseHex fails unless the id decodes as hex and encodes back to
// the same text, which holds for lowercase digits only.
func assertLowercaseHex(t *testing.T, id string) {
	t.Helper()
	decoded, err := hex.DecodeString(id)
	if err != nil || hex.EncodeToString(decoded) != id {
		t.Errorf("id %q is not lowercase hex (decode error %v)", id, err)
	}
}
