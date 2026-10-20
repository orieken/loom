package cmd

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fitness function of architecture-notes.md decisions 4 and 5: payload
// text and the Capture salt never reach the database, its journal files or
// any report. These tests run the built binary, as a person would.

// saltForms lists the raw, hex and base64 forms of the Capture salt.
func saltForms() []string {
	salt := []byte(captureSalt)
	return []string{
		captureSalt,
		hex.EncodeToString(salt),
		base64.StdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(salt),
	}
}

// printed is everything a run wrote to standard output and standard error.
// The memory commands write their reports to standard error.
func (r hookResult) printed() string { return r.stdout + r.stderr }

// runMemory runs `loom memory <args>` in the project root, as a person does.
func (p *captureProject) runMemory(args ...string) hookResult {
	p.t.Helper()
	call := binaryCall{binary: captureBinary(p.t), args: append([]string{"memory"}, args...), dir: p.root}
	return runCall(p.t, call)
}

// humanRevertsTheEdit rewrites the edited line to the old text and lets the
// hook see it, so ingest has a Session Correction to store.
func (p *captureProject) humanRevertsTheEdit(source string) {
	p.t.Helper()
	p.write("app/main.go", "head\n"+canaryOldText+"\ntail\n", 0o600)
	assertSilentSuccess(p.t, p.runHook("PreToolUse", readPayload("s1", source)))
}

// everyFileUnder lists every file of the tree, the database and its -wal,
// -shm and -journal files included, except the files named in skip.
func everyFileUnder(t *testing.T, root string, skip ...string) []string {
	t.Helper()
	skipped := map[string]bool{}
	for _, path := range skip {
		skipped[path] = true
	}
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && !skipped[path] {
			paths = append(paths, path)
		}
		return err
	})
	equal(t, "walk error", err, nil)
	return paths
}

func TestCaptureCanaryReachesNeitherDatabaseNorReport(t *testing.T) {
	project := newCaptureProject(t)
	hookOutput, ownFiles := runCanaryHookCalls(t, project)
	project.humanRevertsTheEdit(ownFiles[0])

	ingest := project.runMemory("ingest")
	text := project.runMemory("corrections")
	asJSON := project.runMemory("corrections", "--sessions", "--json")

	for name, result := range map[string]hookResult{"ingest": ingest, "corrections": text, "corrections --sessions --json": asJSON} {
		equal(t, name+" exit code", result.code, 0)
	}
	equal(t, "ingest stored the corrections", strings.Contains(ingest.printed(), "2 session corrections ingested"), true)
	equal(t, "the text report lists a kind", strings.Contains(text.printed(), "corrective-prompt"), true)
	equal(t, "the json report lists a kind", strings.Contains(asJSON.printed(), `"corrective-prompt"`), true)
	scanned := everyFileUnder(t, project.root, append(ownFiles, project.layout.SaltPath())...)
	forbidden := append([]string{canaryPrompt, canaryOldText, canaryNewText, canaryContent, canaryCommand, canaryFileBody, canaryRedacted}, saltForms()...)
	assertNoneOf(t, forbidden, scannedSurfaces(t, scanned, hookOutput, ingest, text, asJSON))
	equal(t, "the scan covered the stored revert", strings.Contains(databaseText(t, project, scanned), "app/main.go"), true)
}

// scannedSurfaces pairs a name with the text of every file and every output.
func scannedSurfaces(t *testing.T, paths []string, hookOutput string, results ...hookResult) map[string]string {
	t.Helper()
	surfaces := map[string]string{"hook output": hookOutput}
	for _, path := range paths {
		surfaces[path] = readText(t, path)
	}
	for index, result := range results {
		surfaces[fmt.Sprintf("memory output %d", index)] = result.printed()
	}
	return surfaces
}

func assertNoneOf(t *testing.T, forbidden []string, surfaces map[string]string) {
	t.Helper()
	for name, text := range surfaces {
		for _, value := range forbidden {
			if strings.Contains(text, value) {
				t.Errorf("%s holds %q", name, value)
			}
		}
	}
}

// databaseText joins the text of episodes.db and of its -wal, -shm and
// -journal files, so the test can show that the scan reached stored rows.
func databaseText(t *testing.T, project *captureProject, paths []string) string {
	t.Helper()
	var text strings.Builder
	for _, path := range paths {
		if strings.HasPrefix(path, project.layout.DatabasePath()) {
			text.WriteString(readText(t, path))
		}
	}
	return text.String()
}
