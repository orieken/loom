package capture

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const testRoot = "/proj"

// memorySink collects the events of every Append, in order. It doubles as the
// tail: the hook reads back what it wrote, as the day file does.
type memorySink struct {
	mu       sync.Mutex
	events   []SessionEvent
	appends  int
	failWith error
}

func (s *memorySink) Append(_ context.Context, events []SessionEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appends++
	if s.failWith != nil {
		return s.failWith
	}
	s.events = append(s.events, events...)
	return nil
}

func (s *memorySink) Recent(_ context.Context, _ time.Time, _ time.Duration) ([]SessionEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]SessionEvent(nil), s.events...), nil
}

// memoryFiles serves files from a map and counts the reads per path. It
// refuses a file above the cap, as the real reader does.
type memoryFiles struct {
	files map[string]string
	reads map[string]int
}

func newMemoryFiles() *memoryFiles {
	return &memoryFiles{files: map[string]string{}, reads: map[string]int{}}
}

func (m *memoryFiles) put(relative, content string) {
	m.files[filepath.Join(testRoot, relative)] = content
}

func (m *memoryFiles) ReadCapped(_ context.Context, path string, maxBytes int64) ([]byte, error) {
	m.reads[path]++
	content, isPresent := m.files[path]
	if !isPresent {
		return nil, errors.New("not found")
	}
	if int64(len(content)) > maxBytes {
		return nil, ErrFileTooLarge
	}
	return []byte(content), nil
}

func (m *memoryFiles) totalReads() int {
	total := 0
	for _, count := range m.reads {
		total += count
	}
	return total
}

type staticSalt struct {
	salt Salt
	err  error
}

func (s staticSalt) Salt() (Salt, error) { return s.salt, s.err }

type staticConfig struct {
	cfg Config
	err error
}

func (c staticConfig) Load() (Config, error) { return c.cfg, c.err }

type staticEdits struct{ text EditText }

func (s staticEdits) EditTextOf([]byte) EditText { return s.text }

type scriptedDecoder struct {
	obs Observation
	err error
}

func (d scriptedDecoder) Decode(string, []byte) (Observation, error) { return d.obs, d.err }

type recordingPurger struct {
	calls     int
	retention Retention
	err       error
}

func (p *recordingPurger) PurgeIfDue(_ context.Context, _ time.Time, retention Retention) error {
	p.calls++
	p.retention = retention
	return p.err
}

// rig bundles a Recorder with the fakes that a test inspects.
type rig struct {
	recorder *Recorder
	sink     *memorySink
	files    *memoryFiles
	purger   *recordingPurger
	clock    *steppingClock
	hasher   *Hasher
	deps     RecorderDeps
}

// steppingClock moves forward on request, so a test can place calls in time.
type steppingClock struct{ now time.Time }

func (c *steppingClock) Now() time.Time { return c.now }

func (c *steppingClock) advance(by time.Duration) { c.now = c.now.Add(by) }

func newRig(t *testing.T, obs Observation, edits EditText) *rig {
	t.Helper()
	sink := &memorySink{}
	files := newMemoryFiles()
	purger := &recordingPurger{}
	clock := &steppingClock{now: time.Now().UTC()}
	salt := newTestSalt(t)
	deps := RecorderDeps{
		Layout: NewLayout(testRoot), Clock: clock, Sink: sink, Tail: sink, Files: files,
		Salts: staticSalt{salt: salt}, Config: staticConfig{cfg: DefaultConfig()},
		Edits: staticEdits{text: edits}, Decoder: scriptedDecoder{obs: obs},
		Purger: purger, NewEventID: counterIDs(), Home: "/home/user",
	}
	return &rig{
		recorder: NewRecorder(deps), sink: sink, files: files, purger: purger,
		clock: clock, hasher: NewHasher(salt), deps: deps,
	}
}

// call runs the hook once with the given observation.
func (r *rig) call(t *testing.T, obs Observation) error {
	t.Helper()
	r.deps.Decoder = scriptedDecoder{obs: obs}
	r.recorder = NewRecorder(r.deps)
	return r.recorder.Record(context.Background(), "event", []byte("{}"))
}

// mustCall runs the hook once and fails the test on an error.
func (r *rig) mustCall(t *testing.T, obs Observation) {
	t.Helper()
	if err := r.call(t, obs); err != nil {
		t.Fatal(err)
	}
}

// storedLines encodes the events as the day file would hold them.
func storedLines(t *testing.T, events []SessionEvent) string {
	t.Helper()
	var stored strings.Builder
	for _, event := range events {
		line, err := MarshalLine(event)
		if err != nil {
			t.Fatal(err)
		}
		stored.Write(line)
	}
	return stored.String()
}

func (r *rig) pathRef(relative string) string {
	return r.hasher.Hash(HashPath, relative)
}

func (r *rig) lineHash(content string, start, end int) string {
	hash, _ := r.hasher.HashLineRange([]byte(content), start, end)
	return hash
}

func edit(path string) Observation {
	return Observation{Kind: EventToolResult, Tool: ToolEdit, SessionID: "s1", FilePath: filepath.Join(testRoot, path)}
}

func prompt() Observation {
	return Observation{Kind: EventPrompt, SessionID: "s1", Prompt: "no, undo that"}
}

// equal fails the test when two values differ. Moving the comparison here
// keeps each test free of the branches that a failure message needs.
func equal[T comparable](t *testing.T, what string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", what, got, want)
	}
}
