package hooktiming

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
)

const (
	timingSalt    = "0123456789abcdef0123456789abcdef"
	fileLineText  = "0123456789abcde\n" // 16 bytes
	hunkFirstLine = 10
	hunkLastLine  = 14
	uniqueLine    = "UNIQUE-LINE-FOR-THE-EDIT"
)

func stringReader(text string) io.Reader { return strings.NewReader(text) }

// project is a fixture root with a salt and a way to seed the day file.
type project struct {
	mu     sync.Mutex // the Hasher is not safe for concurrent use
	t      *testing.T
	root   string
	layout capture.Layout
	hasher *capture.Hasher
}

func newProject(t *testing.T) *project {
	t.Helper()
	root := t.TempDir()
	p := &project{t: t, root: root, layout: capture.NewLayout(root)}
	p.writeRaw(filepath.Join(".claude", "memory", "capture.salt"), timingSalt)
	if err := os.MkdirAll(p.layout.SessionsDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	salt, err := capture.NewSalt([]byte(timingSalt))
	if err != nil {
		t.Fatal(err)
	}
	p.hasher = capture.NewHasher(salt)
	return p
}

func (p *project) writeRaw(relative, content string) {
	p.t.Helper()
	path := filepath.Join(p.root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		p.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		p.t.Fatal(err)
	}
}

// writeSource writes a source file of about the given size in bytes. The
// marker text, when set, replaces line 20, so the file has one line that no
// other line equals.
func (p *project) writeSource(relative string, size int, marker string) string {
	p.t.Helper()
	lines := make([]string, size/len(fileLineText))
	for i := range lines {
		lines[i] = fileLineText
	}
	if marker != "" {
		lines[19] = marker + "\n"
	}
	content := strings.Join(lines, "")
	p.writeRaw(relative, content)
	return content
}

// hunkEvent is an agent write of lines 10-14 of a file, as an earlier Edit
// would have recorded it. The hash is the hash of the lines on disk now, so
// the hunk is unchanged until a test changes the file.
func (p *project) hunkEvent(relative, content string, at time.Time, start, end int) capture.SessionEvent {
	path := relative
	p.mu.Lock()
	defer p.mu.Unlock()
	hash, _ := p.hasher.HashLineRange([]byte(content), start, end)
	return capture.NewSessionEvent(randomID(at), at, capture.EventToolResult).
		WithTool(capture.ToolEdit).
		WithSessionRef("0123456789abcdef0123456789abcdef").
		WithPath(p.hasher.Hash(capture.HashPath, relative), &path).
		WithHunk(capture.HunkFacts{RangeStart: start, RangeEnd: end, PreHash: "pre", PostHash: hash})
}

func (p *project) fillerEvent(at time.Time) capture.SessionEvent {
	return capture.NewSessionEvent(randomID(at), at, capture.EventToolUse).
		WithTool("Read").
		WithSessionRef("0123456789abcdef0123456789abcdef").
		WithArgShape([]string{"file_path"}, []string{"string"})
}

func randomID(at time.Time) string { return fmt.Sprintf("%016x", at.UnixNano()) }

// writeDayFile replaces today's day file with the given events.
func (p *project) writeDayFile(events []capture.SessionEvent) {
	p.t.Helper()
	p.writeDayFileFor(time.Now(), events)
}

// writeDayFileFor replaces the day file of the UTC date of the given instant.
func (p *project) writeDayFileFor(day time.Time, events []capture.SessionEvent) {
	p.t.Helper()
	var body strings.Builder
	for _, event := range events {
		line, err := capture.MarshalLine(event)
		if err != nil {
			p.t.Fatal(err)
		}
		body.Write(line)
	}
	if err := os.WriteFile(p.layout.DayFile(day), []byte(body.String()), 0o600); err != nil {
		p.t.Fatal(err)
	}
}

func jsonOf(value map[string]string) string {
	body, _ := json.Marshal(value)
	return string(body)
}

func promptPayload() string {
	return jsonOf(map[string]string{"session_id": "timing-session", "prompt": "continue with the plan"})
}

// editPayload is the payload of an Edit result that wrote the unique line.
func editPayload(file string) string {
	body, _ := json.Marshal(map[string]any{
		"session_id": "timing-session", "tool_name": "Edit",
		"tool_input": map[string]string{"file_path": file, "old_string": "old", "new_string": uniqueLine},
	})
	return string(body)
}

func (p *project) hookArgs(event string) []string {
	return []string{"capture", "hook", event, "--root", p.root}
}
