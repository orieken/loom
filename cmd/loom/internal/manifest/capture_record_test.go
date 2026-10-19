package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOldManifestWithoutCaptureStillDecodesAndRoundTripsWithoutTheKey(t *testing.T) {
	target := t.TempDir()
	old := `{"version":"v3.3.14","installedAt":"2026-08-16T12:00:00Z","platforms":[{"name":"cursor","paths":["a"]}]}`
	if err := os.WriteFile(filepath.Join(target, Filename), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Read(target)
	if err != nil || got.Capture != nil || got.Version != "v3.3.14" {
		t.Fatalf("manifest = %+v, err = %v", got, err)
	}
	if err := Write(target, got); err != nil {
		t.Fatal(err)
	}
	rewritten, _ := os.ReadFile(filepath.Join(target, Filename))
	if strings.Contains(string(rewritten), "capture") {
		t.Fatalf("an old manifest gained a capture key: %s", rewritten)
	}
}

func TestCaptureRecordRoundTrips(t *testing.T) {
	target := t.TempDir()
	record := &CaptureRecord{
		HookCommands: []string{"loom capture hook Stop --root '/p'"}, SettingsPath: ".claude/settings.local.json", SettingsCreated: true,
		SessionsDir: ".claude/memory/sessions", SaltPath: ".claude/memory/capture.salt",
		FailuresPath: ".claude/memory/capture.failures", PurgeStampPath: ".claude/memory/capture.purge",
	}
	if err := Write(target, Manifest{Version: "v1", Capture: record}); err != nil {
		t.Fatal(err)
	}
	got, err := Read(target)
	if err != nil || got.Capture == nil {
		t.Fatalf("capture record = %+v, err = %v", got.Capture, err)
	}
	if got.Capture.SaltPath != record.SaltPath || !got.Capture.SettingsCreated || got.Capture.HookCommands[0] != record.HookCommands[0] {
		t.Fatalf("capture record = %+v, want %+v", got.Capture, record)
	}
}

// An older loom reads the manifest with a struct that has no Capture field.
// The record MUST NOT break that decode.
func TestAnOlderBinaryIgnoresTheCaptureKey(t *testing.T) {
	target := t.TempDir()
	if err := Write(target, Manifest{Version: "v1", Capture: &CaptureRecord{SaltPath: "x"}}); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(filepath.Join(target, Filename))
	type olderManifest struct {
		Version   string           `json:"version"`
		Platforms []PlatformRecord `json:"platforms"`
	}
	var older olderManifest
	if err := json.Unmarshal(content, &older); err != nil || older.Version != "v1" {
		t.Fatalf("older decode: %v %+v", err, older)
	}
}
