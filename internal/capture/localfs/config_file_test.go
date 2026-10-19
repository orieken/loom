package localfs

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
)

func TestConfigFileMissingFileGivesTheDefaults(t *testing.T) {
	got, err := NewConfigFile(filepath.Join(t.TempDir(), "capture.yaml")).Load()

	if err != nil || !reflect.DeepEqual(got, capture.DefaultConfig()) {
		t.Fatalf("got %+v, err %v; want the defaults and no error", got, err)
	}
}

func TestConfigFileEmptyFileGivesTheDefaults(t *testing.T) {
	got, err := NewConfigFile(writeTemp(t, "capture.yaml", "")).Load()

	if err != nil || !reflect.DeepEqual(got, capture.DefaultConfig()) {
		t.Fatalf("got %+v, err %v; want the defaults", got, err)
	}
}

func TestConfigFileValuesLieOverTheDefaults(t *testing.T) {
	yamlText := `
window: 5m
retentionDays: 30
capMegabytes: 20
weights:
  hunk-edit: 4
  revert: 0
signals:
  denied-call: true
  corrective-prompt: false
redact:
  - "internal/**"
  - "*.secret"
`
	want := capture.DefaultConfig()
	want.Window = 5 * time.Minute
	want.Retention = capture.Retention{MaxAgeDays: 30, MaxBytes: 20 << 20}
	want.Weights.HunkEdit, want.Weights.Revert = 4, 0
	want.Signals[capture.SignalDeniedCall] = true
	want.Signals[capture.SignalCorrectivePrompt] = false
	want.RedactionGlobs = []string{"internal/**", "*.secret"}

	got, err := NewConfigFile(writeTemp(t, "capture.yaml", yamlText)).Load()

	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, err %v;\nwant %+v", got, err, want)
	}
}

func TestConfigFileRejectsInvalidSettingsWithATypedError(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{name: "unknown key", yaml: "windo: 5m\n"},
		{name: "unknown nested key in a mapping", yaml: "weights:\n  hunk-edit: 1\n  mystery: 1\n"},
		{name: "not yaml", yaml: "window: [unclosed\n"},
		{name: "window not a duration", yaml: "window: soon\n"},
		{name: "window zero", yaml: "window: 0s\n"},
		{name: "window negative", yaml: "window: -5m\n"},
		{name: "retention zero", yaml: "retentionDays: 0\n"},
		{name: "cap zero", yaml: "capMegabytes: 0\n"},
		{name: "negative weight", yaml: "weights:\n  revert: -1\n"},
		{name: "weight of the wrong type", yaml: "weights:\n  revert: lots\n"},
		{name: "unknown signal", yaml: "signals:\n  mind-reading: true\n"},
		{name: "bad glob", yaml: "redact:\n  - \"[bad\"\n"},
		{name: "wrong type for redact", yaml: "redact: yes\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewConfigFile(writeTemp(t, "capture.yaml", tt.yaml)).Load()

			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("err = %v, want ErrInvalidConfig", err)
			}
		})
	}
}

func TestConfigFileRejectsAFileThatIsTooLargeOrNotRegular(t *testing.T) {
	large := writeTemp(t, "capture.yaml", "# "+strings.Repeat("x", maxConfigBytes)+"\n")

	_, largeErr := NewConfigFile(large).Load()
	_, dirErr := NewConfigFile(t.TempDir()).Load()

	if !errors.Is(largeErr, ErrInvalidConfig) || !errors.Is(dirErr, ErrInvalidConfig) {
		t.Fatalf("large: %v, directory: %v; want ErrInvalidConfig for both", largeErr, dirErr)
	}
}

func TestConfigFileDefaultsStayIntactAfterALoad(t *testing.T) {
	_, _ = NewConfigFile(writeTemp(t, "capture.yaml", "signals:\n  denied-call: true\n")).Load()

	if capture.DefaultConfig().Signals.IsOn(capture.SignalDeniedCall) {
		t.Fatal("a load must not change the compiled defaults")
	}
}
