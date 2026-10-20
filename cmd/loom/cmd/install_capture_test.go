package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/orieken/loom/cmd/loom/internal/manifest"
	"github.com/orieken/loom/cmd/loom/internal/settings"
	"github.com/orieken/loom/internal/capture/claudehook"
)

const settingsLocal = ".claude/settings.local.json"

func TestInstallCaptureRegistersTheHooks(t *testing.T) {
	world := newCaptureWorld(t)

	world.installCapture()

	entries, err := settings.CountEntries(world.path(settingsLocal), settings.Selector{Prefix: captureCommandPrefix})
	if err != nil || entries != len(claudehook.RegisteredEvents()) {
		t.Fatalf("hook entries = %d, err = %v", entries, err)
	}
	for _, event := range claudehook.RegisteredEvents() {
		want := captureHookCommand(event, world.target)
		if !strings.Contains(world.read(settingsLocal), want) {
			t.Errorf("settings.local.json has no command %q", want)
		}
	}
}

func TestInstallCaptureRecordsEverythingInTheManifest(t *testing.T) {
	world := newCaptureWorld(t)

	output := world.installCapture()

	record := world.manifest().Capture
	if record == nil || len(record.HookCommands) != 4 || !record.SettingsCreated {
		t.Fatalf("manifest capture record = %+v", record)
	}
	if !strings.Contains(output, "salt created") || !strings.Contains(output, "4 hook entries added") {
		t.Fatalf("output = %s", output)
	}
}

func TestInstallCaptureCreatesAPrivate32ByteSaltAndNoConfigFile(t *testing.T) {
	world := newCaptureWorld(t)

	world.installCapture()

	info, err := os.Stat(world.layout().SaltPath())
	if err != nil || info.Size() != 32 || info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("salt info = %v, err = %v", info, err)
	}
	if world.exists(".claude/capture.yaml") {
		t.Fatal("install created capture.yaml; a missing file must be fine")
	}
}

func TestSaltValueNeverAppearsInOutputManifestOrHealth(t *testing.T) {
	world := newCaptureWorld(t)
	output := world.installCapture()
	salt := world.saltBytes()
	surfaces := map[string]string{"install output": output, "health output": world.health(), "manifest": world.read(".loom-manifest.json"), "settings": world.read(settingsLocal)}
	forms := []string{string(salt), hex.EncodeToString(salt), base64.StdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(salt)}
	for name, text := range surfaces {
		for _, form := range forms {
			if strings.Contains(text, form) {
				t.Errorf("the salt value appears in the %s", name)
			}
		}
	}
}

func TestInstallCaptureRefusesWhenClaudeCodeIsNotDetectedAndWritesNothing(t *testing.T) {
	for name, platform := range map[string]string{"not detected": "", "other platform chosen": "cursor"} {
		t.Run(name, func(t *testing.T) {
			world := newCaptureWorld(t)
			if err := os.RemoveAll(world.path(".claude")); err != nil {
				t.Fatal(err)
			}
			before := world.snapshot()

			output, err := world.install(installFlags{level: 1, platform: platform, withCapture: true})

			if err == nil || !strings.Contains(err.Error(), "Claude Code") || !strings.Contains(err.Error(), "Nothing was installed") {
				t.Fatalf("err = %v", err)
			}
			world.assertTreeUnchangedSince(before)
			if strings.Contains(output, "installing framework") {
				t.Fatalf("the installer started: %s", output)
			}
		})
	}
}

func TestInstallCaptureRefusesWhenMemoryIsNotIgnoredAndPrintsTheExactLine(t *testing.T) {
	for name, content := range map[string]string{"no ignore file": "", "other rules": "node_modules\n", "rule undone": ".claude/memory/\n!.claude/memory/\n"} {
		t.Run(name, func(t *testing.T) {
			world := newCaptureWorld(t)
			if content == "" {
				_ = os.Remove(world.path(".gitignore"))
			} else {
				world.write(".gitignore", content, 0o644)
			}
			before := world.snapshot()

			_, err := world.install(installFlags{level: 1, withCapture: true})

			if err == nil || !strings.Contains(err.Error(), "\n\n  .claude/memory/\n\n") {
				t.Fatalf("err = %v; want the exact line, indented, alone", err)
			}
			world.assertTreeUnchangedSince(before)
			if world.exists(".gitignore") && world.read(".gitignore") != content {
				t.Fatal("the installer edited .gitignore; the human decided it must only print the line")
			}
		})
	}
}

func TestInstallLevelThreeWithoutCaptureRegistersNothingAndHintsTheFlag(t *testing.T) {
	world := newCaptureWorld(t)

	output, err := world.install(installFlags{level: 3, platform: "claude-code"})

	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !strings.Contains(output, "--capture") || strings.Count(output, "capture:") != 1 {
		t.Fatalf("want exactly one capture hint naming --capture:\n%s", output)
	}
	if world.exists(settingsLocal) || world.exists(".claude/memory/capture.salt") || world.manifest().Capture != nil {
		t.Fatal("level 3 turned capture on")
	}
}

func TestInstallBelowLevelThreeSaysNothingAboutCapture(t *testing.T) {
	world := newCaptureWorld(t)

	output, err := world.install(installFlags{level: 2, platform: "claude-code"})

	if err != nil || strings.Contains(output, "capture") {
		t.Fatalf("err = %v, output mentions capture: %v", err, strings.Contains(output, "capture"))
	}
}

func TestInstallCaptureTwiceChangesNothingAndKeepsTheSalt(t *testing.T) {
	world := newCaptureWorld(t)
	world.installCapture()
	settingsBefore, saltBefore, record := world.read(settingsLocal), world.saltBytes(), world.manifest().Capture

	output := world.installCapture()

	if world.read(settingsLocal) != settingsBefore {
		t.Fatal("the second install changed settings.local.json")
	}
	if !bytes.Equal(world.saltBytes(), saltBefore) {
		t.Fatal("the second install rotated the salt")
	}
	assertSameRecord(t, world.manifest().Capture, record)
	if !strings.Contains(output, "kept the existing salt") || !strings.Contains(output, "0 hook entries added") {
		t.Fatalf("output = %s", output)
	}
}

func TestPlainInstallAfterCaptureKeepsTheCaptureRecord(t *testing.T) {
	world := newCaptureWorld(t)
	world.installCapture()

	if _, err := world.install(installFlags{level: 1}); err != nil {
		t.Fatal(err)
	}

	if world.manifest().Capture == nil {
		t.Fatal("a later install without --capture erased the capture record")
	}
}

func TestInstallCaptureKeepsUserHooksAndTheFlatAfterEditString(t *testing.T) {
	world := newCaptureWorld(t)
	world.write(settingsLocal, canonicalLocalSettings, 0o600)

	world.installCapture()

	merged := world.read(settingsLocal)
	for _, kept := range []string{`"afterEdit": "npm test --changed"`, `"command": "echo checking && true"`, `"Bash(go test:*)"`} {
		if !strings.Contains(merged, kept) {
			t.Errorf("settings.local.json lost %s:\n%s", kept, merged)
		}
	}
	record := world.manifest().Capture
	if record.SettingsCreated {
		t.Fatal("the manifest says loom created a file the user had")
	}
}

func TestInstallCaptureRefusesAnOddHooksShapeAndTouchesNothing(t *testing.T) {
	world := newCaptureWorld(t)
	odd := `{"hooks": {"PreToolUse": "echo not an array"}}`
	world.write(settingsLocal, odd, 0o600)
	before := world.snapshot()

	_, err := world.install(installFlags{level: 1, withCapture: true})

	var refusal *settings.NotMergeableError
	if !errors.As(err, &refusal) || !strings.Contains(err.Error(), "Add these entries by hand") {
		t.Fatalf("err = %v", err)
	}
	world.assertTreeUnchangedSince(before)
}

func TestInstallCaptureDryRunWritesNothingAndSaysWhatItWouldDo(t *testing.T) {
	world := newCaptureWorld(t)
	before := world.snapshot()

	output, err := world.install(installFlags{level: 1, withCapture: true, isDryRun: true})

	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "would create the salt") || !strings.Contains(output, "would register the hooks") {
		t.Fatalf("output = %s", output)
	}
	world.assertTreeUnchangedSince(before)
}

func TestInstallCaptureDryRunStillRefusesWhenMemoryIsNotIgnored(t *testing.T) {
	world := newCaptureWorld(t)
	world.write(".gitignore", "", 0o644)

	_, err := world.install(installFlags{level: 1, withCapture: true, isDryRun: true})

	if err == nil {
		t.Fatal("a dry run must refuse for the same reasons")
	}
}

func TestPreflightRefusesOnWindows(t *testing.T) {
	stage := captureInstall{isEnabled: true, goos: "windows", platforms: []string{claudePlatform}}
	if err := stage.preflight(); err == nil || !strings.Contains(err.Error(), "Windows") {
		t.Fatalf("err = %v", err)
	}
}

func TestPreflightDoesNothingWhenCaptureIsOff(t *testing.T) {
	if err := (captureInstall{goos: "windows"}).preflight(); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestCaptureFlagIsRegisteredAndOffByDefault(t *testing.T) {
	flag := installCmd.Flags().Lookup("capture")
	if flag == nil || flag.DefValue != "false" {
		t.Fatalf("flag = %+v", flag)
	}
}

func TestHookCommandQuotesTheRoot(t *testing.T) {
	cases := map[string]string{
		"/p":         `loom capture hook Stop --root '/p'`,
		"/my dir":    `loom capture hook Stop --root '/my dir'`,
		"/it's here": `loom capture hook Stop --root '/it'\''s here'`,
	}
	for root, want := range cases {
		if got := captureHookCommand("Stop", root); got != want {
			t.Errorf("command = %s, want %s", got, want)
		}
	}
}

func TestRelativeSlashFallsBackToTheInputWhenNoRelationExists(t *testing.T) {
	if got := relativeSlash("rel/root", "/abs/file"); got != "/abs/file" {
		t.Fatalf("got %s", got)
	}
}

func assertSameRecord(t *testing.T, got, want *manifest.CaptureRecord) {
	t.Helper()
	if got == nil || !got.SettingsCreated || len(got.HookCommands) != len(want.HookCommands) {
		t.Fatalf("record after second install = %+v, want %+v", got, want)
	}
}
