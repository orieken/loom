package cmd

import (
	"os"
	"strings"
	"testing"
	"time"
)

const megabyte = 1 << 20

func liveWorld(t *testing.T) *captureWorld {
	t.Helper()
	world := newCaptureWorld(t)
	world.installCapture()
	return world
}

func (w *captureWorld) sparseDayFile(megabytes float64) {
	w.t.Helper()
	path := w.write(".claude/memory/sessions/2026-10-18.jsonl", "", 0o600)
	if err := os.Truncate(path, int64(megabytes*megabyte)); err != nil {
		w.t.Fatal(err)
	}
}

func assertContains(t *testing.T, text string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Errorf("output does not contain %q:\n%s", want, text)
		}
	}
}

func TestHealthSaysCaptureIsOffWhenNothingIsRegistered(t *testing.T) {
	world := newCaptureWorld(t)
	if _, err := world.install(installFlags{level: 1}); err != nil {
		t.Fatal(err)
	}

	output := world.health()

	assertContains(t, output, "capture: off")
	if strings.Contains(output, "capture: store") {
		t.Fatalf("an off capture reported a store:\n%s", output)
	}
}

func TestHealthSaysCaptureIsLive(t *testing.T) {
	output := liveWorld(t).health()

	assertContains(t, output, "capture: live (4 hook entries registered)", "capture: store 0.0 MB of 50 MB")
}

func TestHealthIsOffAgainAfterTheHooksAreRemovedByHandButReportsTheLeftovers(t *testing.T) {
	world := liveWorld(t)
	world.write(settingsLocal, "{}\n", 0o600)

	output := world.health()

	assertContains(t, output, "capture: off", "capture: store")
}

func TestHealthReportsTheFailureCount(t *testing.T) {
	world := liveWorld(t)
	world.write(".claude/memory/capture.failures", "dda", 0o600)

	assertContains(t, world.health(), "⚠ capture: 3 hook failures recorded")
}

func TestHealthReportsAStallMarkerOnlyWhileItIsYoungerThanTheWindow(t *testing.T) {
	cases := map[string]struct {
		age       time.Duration
		wantStall bool
	}{"fresh": {10 * time.Second, true}, "expired": {2 * time.Minute, false}}
	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			world := liveWorld(t)
			path := world.write(".claude/memory/capture.failures", "t", 0o600)
			stamp := time.Now().Add(-tt.age)
			if err := os.Chtimes(path, stamp, stamp); err != nil {
				t.Fatal(err)
			}

			got := strings.Contains(world.health(), "stall marker set")

			if got != tt.wantStall {
				t.Fatalf("stall reported = %v, want %v", got, tt.wantStall)
			}
		})
	}
}

func TestHealthFailsForASaltTheHookCannotUse(t *testing.T) {
	cases := map[string]struct {
		prepare func(w *captureWorld)
		want    string
	}{
		"missing": {func(w *captureWorld) { _ = os.Remove(w.layout().SaltPath()) }, "the salt file is missing"},
		"wrong mode": {func(w *captureWorld) {
			if err := os.Chmod(w.layout().SaltPath(), 0o644); err != nil {
				w.t.Fatal(err)
			}
		}, "can be read by other users"},
		"wrong size": {func(w *captureWorld) { w.write(".claude/memory/capture.salt", "short", 0o600) }, "is not 32 bytes"},
	}
	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			world := liveWorld(t)
			tt.prepare(world)

			output := world.health()

			assertContains(t, output, "✗ capture: ", tt.want)
		})
	}
}

func TestHealthWarnsAtEightyPercentOfTheCap(t *testing.T) {
	cases := map[string]struct {
		megabytes float64
		wantWarn  bool
	}{"below": {39.9, false}, "at": {40, true}, "above": {49, true}}
	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			world := liveWorld(t)
			world.sparseDayFile(tt.megabytes)

			output := world.health()

			if got := strings.Contains(output, "⚠ capture: store"); got != tt.wantWarn {
				t.Fatalf("warned = %v, want %v:\n%s", got, tt.wantWarn, output)
			}
		})
	}
}

func TestHealthUsesTheCapFromCaptureYaml(t *testing.T) {
	world := liveWorld(t)
	world.write(".claude/capture.yaml", "capMegabytes: 2\n", 0o644)
	world.sparseDayFile(1.7)

	assertContains(t, world.health(), "⚠ capture: store 1.7 MB of 2 MB")
}

func TestHealthWarnsAboutAnInvalidCaptureYaml(t *testing.T) {
	world := liveWorld(t)
	world.write(".claude/capture.yaml", "nonsense: [", 0o644)

	assertContains(t, world.health(), "capture.yaml is invalid")
}

func TestHealthCountsOnlyDayFilesInTheStore(t *testing.T) {
	world := liveWorld(t)
	world.write(".claude/memory/sessions/notes.txt", strings.Repeat("x", 4096), 0o600)
	world.write(".claude/memory/sessions/2026-10-18.jsonl", strings.Repeat("x", 1024), 0o600)

	if got := storeBytes(world.layout()); got != 1024 {
		t.Fatalf("store bytes = %d, want 1024", got)
	}
}

// The check lists the directory and stats files. It must never read a day
// file: with read permission removed it still reports the size.
func TestHealthDoesNotReadDayFiles(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads every file")
	}
	world := liveWorld(t)
	path := world.write(".claude/memory/sessions/2026-10-18.jsonl", strings.Repeat("x", 2*megabyte), 0o600)
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	assertContains(t, world.health(), "capture: store 2.0 MB of 50 MB")
}

func TestHealthFailsTheRunForALiveCaptureWithoutASalt(t *testing.T) {
	world := liveWorld(t)
	_ = os.Remove(world.layout().SaltPath())
	var output strings.Builder
	err := executeHealth(healthRequest{target: world.target, display: world.target}, healthContent(), &output)
	if err == nil {
		t.Fatal("a live capture that records nothing must fail health")
	}
}
