package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/orieken/loom/cmd/loom/internal/manifest"
	"github.com/orieken/loom/internal/memory"
)

// userProject is the state before any install: the user's own files only.
func userProject(t *testing.T) *captureWorld {
	t.Helper()
	world := newCaptureWorld(t)
	world.write(settingsLocal, canonicalLocalSettings, 0o600)
	world.write(".claude/capture.yaml", "window: 10m\n", 0o644)
	world.withDatabase()
	return world
}

func TestInstallThenUninstallLeavesTheTreeAsItWas(t *testing.T) {
	world := userProject(t)
	before := world.snapshot()
	tablesBefore := world.tables()
	world.installCapture()
	world.simulateHookActivity()
	world.withSessionTable()

	output := world.uninstall(uninstallRequest{})

	world.assertTreeUnchangedSince(before)
	if world.read(settingsLocal) != canonicalLocalSettings {
		t.Fatal("settings.local.json is not byte for byte what the user had")
	}
	if strings.Join(world.tables(), ",") != strings.Join(tablesBefore, ",") {
		t.Fatalf("tables = %v, want %v", world.tables(), tablesBefore)
	}
	if !strings.Contains(output, "left in place .claude/capture.yaml") {
		t.Fatalf("output does not list the config left in place:\n%s", output)
	}
}

func TestUninstallRemovesEachCaptureArtifact(t *testing.T) {
	world := userProject(t)
	world.installCapture()
	world.simulateHookActivity()
	world.withSessionTable()

	world.uninstall(uninstallRequest{})

	for _, gone := range []string{".claude/memory/sessions", ".claude/memory/capture.salt", ".claude/memory/capture.failures", ".claude/memory/capture.purge"} {
		if world.exists(gone) {
			t.Errorf("%s stayed behind", gone)
		}
	}
	tables, _ := memory.PlanSessionTableDrop(world.layout().DatabasePath())
	if len(tables) != 0 {
		t.Fatalf("session table stayed: %v", tables)
	}
	if world.manifestExists() {
		t.Fatal("the manifest stayed after a full uninstall")
	}
}

func (w *captureWorld) manifestExists() bool { return w.exists(".loom-manifest.json") }

func TestUninstallRemovesTheMemoryDirectoryItMadeWhenNothingElseIsInIt(t *testing.T) {
	world := newCaptureWorld(t)
	world.installCapture()

	world.uninstall(uninstallRequest{})

	if world.exists(".claude/memory") || world.exists(settingsLocal) {
		t.Fatal("residue: the memory directory or the settings file loom created stayed")
	}
}

func TestUninstallKeepsASettingsFileThatHoldsOtherKeysEvenIfLoomMadeIt(t *testing.T) {
	world := newCaptureWorld(t)
	world.installCapture()
	merged := strings.Replace(world.read(settingsLocal), "{\n", "{\n  \"model\": \"x\",\n", 1)
	world.write(settingsLocal, merged, 0o600)

	world.uninstall(uninstallRequest{})

	if !strings.Contains(world.read(settingsLocal), `"model": "x"`) || strings.Contains(world.read(settingsLocal), "loom capture") {
		t.Fatalf("settings = %s", world.read(settingsLocal))
	}
}

func TestUninstallDryRunChangesNothingAndListsEverything(t *testing.T) {
	world := userProject(t)
	world.installCapture()
	world.simulateHookActivity()
	world.withSessionTable()
	before := world.snapshot()
	tablesBefore := world.tables()

	output := world.uninstall(uninstallRequest{isDryRun: true})

	world.assertTreeUnchangedSince(before)
	if strings.Join(world.tables(), ",") != strings.Join(tablesBefore, ",") {
		t.Fatal("a dry run dropped a table")
	}
	for _, listed := range []string{"would remove 4 capture hook entries", "would remove .claude/memory/sessions", "would remove .claude/memory/capture.salt", "would remove table session_corrections", "left in place .claude/capture.yaml"} {
		if !strings.Contains(output, listed) {
			t.Errorf("dry run does not list %q:\n%s", listed, output)
		}
	}
}

func TestPartialPlatformUninstallOfAnotherPlatformKeepsCapture(t *testing.T) {
	world := newCaptureWorld(t)
	world.write(".cursor/keep.txt", "x", 0o644)
	if _, err := world.install(installFlags{level: 1, withCapture: true}); err != nil {
		t.Fatal(err)
	}

	output := world.uninstall(uninstallRequest{platform: "cursor"})

	if !world.exists(".claude/memory/capture.salt") || world.manifest().Capture == nil {
		t.Fatal("uninstalling cursor removed capture")
	}
	if strings.Contains(output, "capture") {
		t.Fatalf("output mentions capture:\n%s", output)
	}
}

func TestPartialPlatformUninstallOfClaudeCodeRemovesCapture(t *testing.T) {
	world := newCaptureWorld(t)
	world.write(".cursor/keep.txt", "x", 0o644)
	if _, err := world.install(installFlags{level: 1, withCapture: true}); err != nil {
		t.Fatal(err)
	}

	world.uninstall(uninstallRequest{platform: claudePlatform})

	if world.exists(".claude/memory/capture.salt") || world.exists(settingsLocal) {
		t.Fatal("capture stayed after uninstalling claude-code")
	}
	if world.manifest().Capture != nil {
		t.Fatal("the manifest still holds the capture record")
	}
}

func TestUninstallWithoutCaptureTouchesNothingAboutIt(t *testing.T) {
	world := newCaptureWorld(t)
	world.write(settingsLocal, canonicalLocalSettings, 0o600)
	if _, err := world.install(installFlags{level: 1}); err != nil {
		t.Fatal(err)
	}

	output := world.uninstall(uninstallRequest{})

	if strings.Contains(output, "capture") || world.read(settingsLocal) != canonicalLocalSettings {
		t.Fatalf("output = %s", output)
	}
}

func TestUninstallFindsCaptureArtifactsEvenWithoutAManifestRecord(t *testing.T) {
	world := newCaptureWorld(t)
	world.installCapture()
	world.simulateHookActivity()
	installed := world.manifest()
	installed.Capture = nil
	if err := manifest.Write(world.target, installed); err != nil {
		t.Fatal(err)
	}

	world.uninstall(uninstallRequest{})

	if world.exists(".claude/memory/sessions") || world.exists(".claude/memory/capture.salt") {
		t.Fatal("artifacts without a record stayed")
	}
	if strings.Contains(world.read(settingsLocal), "loom capture") {
		t.Fatal("hook entries without a record stayed (the prefix fallback failed)")
	}
}

func TestUninstallTellsTheUserWhenItCannotEditTheSettingsFileAndGoesOn(t *testing.T) {
	world := newCaptureWorld(t)
	world.installCapture()
	odd := `{"hooks": "not an object"}`
	world.write(settingsLocal, odd, 0o600)

	output := world.uninstall(uninstallRequest{})

	if world.read(settingsLocal) != odd || !strings.Contains(output, "left .claude/settings.local.json as it is") {
		t.Fatalf("output = %s", output)
	}
	if world.exists(".claude/memory/capture.salt") {
		t.Fatal("the rest of the removal did not run")
	}
}

func TestUninstallKeepsTheOtherTablesOfEpisodesDatabase(t *testing.T) {
	world := userProject(t)
	world.installCapture()
	world.withSessionTable()
	if len(world.tables()) < 2 {
		t.Fatalf("fixture has too few tables: %v", world.tables())
	}

	world.uninstall(uninstallRequest{})

	for _, name := range world.tables() {
		if name == "session_corrections" {
			t.Fatal("session table stayed")
		}
	}
	if len(world.tables()) == 0 {
		t.Fatal("the other tables were dropped")
	}
	if _, err := os.Stat(world.layout().DatabasePath()); err != nil {
		t.Fatalf("episodes.db was deleted: %v", err)
	}
}
