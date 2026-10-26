package memory_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/memory"
	"github.com/orieken/loom/internal/orchestrator"
)

// A record that exists but cannot be read fails the archive, naming it:
// skipping it would archive a run without its state and say nothing.
func TestArchiveReportsARecordItCannotRead(t *testing.T) {
	workspace := t.TempDir()
	if err := os.Mkdir(filepath.Join(workspace, orchestrator.RunStateFileName), 0o755); err != nil {
		t.Fatal(err)
	}

	err := memory.ArchiveRecords(workspace, filepath.Join(t.TempDir(), "user-auth"), workspace)

	if err == nil || !strings.Contains(err.Error(), "read "+filepath.Join(workspace, orchestrator.RunStateFileName)) {
		t.Errorf("err = %v, want a read failure naming the run state", err)
	}
}

func TestArchiveReportsARecordItCannotWrite(t *testing.T) {
	workspace := t.TempDir()
	state, events := fixtureRun()
	writeWorkspace(t, workspace, state, events)
	archive := filepath.Join(t.TempDir(), "user-auth")
	if err := os.MkdirAll(filepath.Join(archive, orchestrator.RunStateFileName), 0o755); err != nil {
		t.Fatal(err)
	}

	err := memory.ArchiveRecords(workspace, archive, workspace)

	if err == nil || !strings.Contains(err.Error(), "write "+filepath.Join(archive, orchestrator.RunStateFileName)) {
		t.Errorf("err = %v, want a write failure naming the archived run state", err)
	}
}
