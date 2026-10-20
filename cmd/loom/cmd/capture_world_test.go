package cmd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	loom "github.com/orieken/loom"
	"github.com/orieken/loom/cmd/loom/internal/manifest"
	"github.com/orieken/loom/internal/capture"
	"github.com/orieken/loom/internal/memory"
)

// canonicalLocalSettings is a user's settings.local.json with a flat
// "afterEdit" string and a second user hook. It is already in the layout
// that the adapter writes, so install followed by uninstall MUST give back
// exactly these bytes.
const canonicalLocalSettings = `{
  "hooks": {
    "PreToolUse": [
      {
        "hooks": [
          {
            "command": "echo checking && true",
            "type": "command"
          }
        ],
        "matcher": "Bash"
      }
    ],
    "afterEdit": "npm test --changed"
  },
  "permissions": {
    "allow": [
      "Bash(go test:*)"
    ]
  }
}
`

// captureWorld is a temporary project. Every path lives under t.TempDir().
type captureWorld struct {
	t      *testing.T
	target string
}

// newCaptureWorld makes a project where Claude Code is detected and
// .claude/memory/ is ignored by git.
func newCaptureWorld(t *testing.T) *captureWorld {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	previousFramework, previousMCP := frameworkFS, mcpFS
	frameworkFS, mcpFS = loom.FrameworkFS, loom.MCPFS
	t.Cleanup(func() { frameworkFS, mcpFS = previousFramework, previousMCP })
	resolved, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	world := &captureWorld{t: t, target: resolved}
	world.write(".gitignore", ".claude/memory/\n", 0o644)
	world.write(".claude/keep.txt", "claude dir\n", 0o644)
	return world
}

func (w *captureWorld) path(relative string) string {
	return filepath.Join(w.target, filepath.FromSlash(relative))
}

func (w *captureWorld) write(relative, content string, mode os.FileMode) string {
	w.t.Helper()
	path := w.path(relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		w.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		w.t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		w.t.Fatal(err)
	}
	return path
}

func (w *captureWorld) read(relative string) string {
	w.t.Helper()
	content, err := os.ReadFile(w.path(relative))
	if err != nil {
		w.t.Fatal(err)
	}
	return string(content)
}

func (w *captureWorld) exists(relative string) bool {
	_, err := os.Lstat(w.path(relative))
	return err == nil
}

// install runs the whole install command path and returns its output.
func (w *captureWorld) install(flags installFlags) (string, error) {
	w.t.Helper()
	flags.target = w.target
	flags.isCopy = true
	var output bytes.Buffer
	err := installProject(flags, &output)
	return output.String(), err
}

func (w *captureWorld) installCapture() string {
	w.t.Helper()
	output, err := w.install(installFlags{level: 1, withCapture: true})
	if err != nil {
		w.t.Fatalf("install --capture: %v\n%s", err, output)
	}
	return output
}

func (w *captureWorld) uninstall(request uninstallRequest) string {
	w.t.Helper()
	request.target, request.display = w.target, w.target
	var output bytes.Buffer
	if err := executeUninstall(request, &output); err != nil {
		w.t.Fatalf("uninstall: %v\n%s", err, output.String())
	}
	return output.String()
}

func (w *captureWorld) health() string {
	w.t.Helper()
	var output bytes.Buffer
	_ = executeHealth(healthRequest{target: w.target, display: w.target}, healthContent(), &output)
	return output.String()
}

// healthAt runs health with a clock that the test sets.
func (w *captureWorld) healthAt(now time.Time) string {
	w.t.Helper()
	var output bytes.Buffer
	request := healthRequest{target: w.target, display: w.target, timeSource: healthClock{now: now}}
	_ = executeHealth(request, healthContent(), &output)
	return output.String()
}

type healthClock struct{ now time.Time }

func (c healthClock) Now() time.Time { return c.now }

func (w *captureWorld) layout() capture.Layout { return capture.NewLayout(w.target) }

func (w *captureWorld) manifest() manifest.Manifest {
	w.t.Helper()
	installed, err := manifest.Read(w.target)
	if err != nil {
		w.t.Fatal(err)
	}
	return installed
}

// simulateHookActivity makes what the running hooks leave behind: a day
// file, a failure log line, the purge stamp.
func (w *captureWorld) simulateHookActivity() {
	w.write(".claude/memory/sessions/2026-10-18.jsonl", `{"v":1}`+"\n", 0o600)
	w.write(".claude/memory/capture.failures.log", capture.NewFailureEntry(time.Now(), capture.CauseDecode).Line(), 0o600)
	w.write(".claude/memory/capture.purge", "", 0o600)
}

// withDatabase creates episodes.db with the tables every store has.
func (w *captureWorld) withDatabase() {
	w.t.Helper()
	store, err := memory.Open(w.layout().DatabasePath())
	if err != nil {
		w.t.Fatal(err)
	}
	_ = store.Close()
}

// withSessionTable opens episodes.db like ingest does and creates the
// session table beside the tables every store has.
func (w *captureWorld) withSessionTable() {
	w.t.Helper()
	store, err := memory.Open(w.layout().DatabasePath())
	if err != nil {
		w.t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	_, err = store.IngestSessions(context.Background(), newDaySessionSource(w.layout()), capture.DefaultConfig())
	if err != nil {
		w.t.Fatal(err)
	}
}

// tables lists the table names of episodes.db.
func (w *captureWorld) tables() []string {
	w.t.Helper()
	db, err := sql.Open("sqlite", w.layout().DatabasePath())
	if err != nil {
		w.t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name`)
	if err != nil {
		w.t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			w.t.Fatal(err)
		}
		names = append(names, name)
	}
	return names
}

// snapshot describes the tree: every path with its mode and content hash.
// The bytes of episodes.db are left out, because SQLite rewrites them.
func (w *captureWorld) snapshot() map[string]string {
	w.t.Helper()
	seen := map[string]string{}
	err := filepath.WalkDir(w.target, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || path == w.target {
			return walkErr
		}
		relative, _ := filepath.Rel(w.target, path)
		seen[filepath.ToSlash(relative)] = describe(path, entry)
		return nil
	})
	if err != nil {
		w.t.Fatal(err)
	}
	return seen
}

func describe(path string, entry fs.DirEntry) string {
	info, _ := entry.Info()
	if entry.IsDir() {
		return "dir " + info.Mode().Perm().String()
	}
	if strings.HasPrefix(entry.Name(), "episodes.db") {
		// Session ingest tightens the mode to 0600 (architecture decision 8)
		// and uninstall leaves it so; only the existence is compared.
		return "file"
	}
	content, _ := os.ReadFile(path)
	digest := sha256.Sum256(content)
	return "file " + info.Mode().Perm().String() + " " + hex.EncodeToString(digest[:8])
}

func diffSnapshots(before, after map[string]string) string {
	var lines []string
	for path, want := range before {
		if got, ok := after[path]; !ok {
			lines = append(lines, "missing: "+path)
		} else if got != want {
			lines = append(lines, "changed: "+path+" ("+want+" -> "+got+")")
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			lines = append(lines, "extra: "+path)
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func (w *captureWorld) assertTreeUnchangedSince(before map[string]string) {
	w.t.Helper()
	if difference := diffSnapshots(before, w.snapshot()); difference != "" {
		w.t.Fatalf("the tree changed:\n%s", difference)
	}
}

func (w *captureWorld) saltBytes() []byte {
	w.t.Helper()
	salt, err := os.ReadFile(w.layout().SaltPath())
	if err != nil {
		w.t.Fatal(err)
	}
	return salt
}
