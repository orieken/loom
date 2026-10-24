package mock_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
)

// Roadmap L3.73: the mock can deliver a real implementation, so a harness's
// hidden acceptance tests have something to judge.

func invoke(t *testing.T, script mock.Script) (string, orchestrator.StageOutput) {
	t.Helper()
	root := t.TempDir()
	provider := mock.New(map[string]mock.Script{"stage": script})
	output, err := provider.Invoke(context.Background(), orchestrator.Stage{ID: "stage"},
		orchestrator.StageInput{ProjectRoot: root, WorkspaceDir: t.TempDir()})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	return root, output
}

func TestImplementationWritingClaimsAndWritesTheGivenFiles(t *testing.T) {
	files := map[string][]byte{"calc/divide.go": []byte("package calc\n"), "calc/b.go": []byte("package calc // b\n")}
	root, output := invoke(t, mock.ImplementationWriting(files))

	for path, want := range files {
		got, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || string(got) != string(want) {
			t.Errorf("%s = %q, %v; want %q", path, got, err, want)
		}
	}
	var payload struct{ FilesCreated []string }
	if err := json.Unmarshal(output.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.FilesCreated) != 2 || payload.FilesCreated[0] != "calc/b.go" {
		t.Errorf("claimed %v, want both files in order", payload.FilesCreated)
	}
}

func TestQAWritingClaimsTheTestFiles(t *testing.T) {
	root, output := invoke(t, mock.QAWriting(map[string][]byte{"calc/divide_test.go": []byte("package calc\n")}))
	if _, err := os.Stat(filepath.Join(root, "calc", "divide_test.go")); err != nil {
		t.Errorf("test file not written: %v", err)
	}
	var payload struct{ TestFilesCreated []string }
	if err := json.Unmarshal(output.Payload, &payload); err != nil || len(payload.TestFilesCreated) != 1 {
		t.Errorf("claimed %v, %v", payload.TestFilesCreated, err)
	}
}

func TestAClaimWithoutContentGetsThePlaceholder(t *testing.T) {
	script, _ := mock.TypedScript("implementation")
	root, _ := invoke(t, script)
	got, err := os.ReadFile(filepath.Join(root, "internal", "mock", "thing.go"))
	if err != nil || string(got) != "// written by the mock provider\n" {
		t.Errorf("placeholder = %q, %v", got, err)
	}
}
