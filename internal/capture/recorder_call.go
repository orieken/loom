package capture

import (
	"context"
	"time"
)

// hookCall holds the state of one hook call. It is built per call and
// dropped at the end, so no state lives between two processes.
type hookCall struct {
	ctx       context.Context
	deps      *RecorderDeps
	cfg       Config
	now       time.Time
	hasher    *Hasher
	paths     PathPolicy
	assembler *Assembler
	locator   HunkLocator
	files     *callFiles
	obs       Observation
	payload   []byte
}

func (r *Recorder) buildCall(ctx context.Context, cfg Config, salt Salt) (*hookCall, error) {
	redactor, err := NewRedactor(cfg.RedactionGlobs, r.deps.Home)
	if err != nil {
		return nil, err
	}
	now := r.deps.Clock.Now().UTC()
	hasher := NewHasher(salt)
	paths := NewPathPolicy(r.deps.Layout.Root(), redactor)
	return &hookCall{
		ctx: ctx, deps: &r.deps, cfg: cfg, now: now, hasher: hasher, paths: paths,
		locator: NewHunkLocator(hasher),
		files:   newCallFiles(r.deps.Files, r.deps.Layout.Root()),
		assembler: NewAssembler(AssemblerDeps{
			Hasher: hasher, Paths: paths, Clock: instantClock{at: now}, NewEventID: r.deps.NewEventID,
		}),
	}, nil
}

// events builds the main events first and the hunk observations after them.
// The sink keeps the main events when the batch cap drops lines.
func (c *hookCall) events() ([]SessionEvent, error) {
	tail := c.recentEvents()
	primary, err := c.primaryEvents(tail)
	if err != nil {
		return nil, err
	}
	return append(primary, c.observationEvents(tail, primary[0])...), nil
}

// recentEvents reads the tail for the calls that can need it. A tail that
// cannot be read loses recall only.
func (c *hookCall) recentEvents() []SessionEvent {
	if !c.needsTail() {
		return nil
	}
	events, err := c.deps.Tail.Recent(c.ctx, c.now, c.cfg.Window)
	if err != nil {
		return nil
	}
	return events
}

// needsTail says whether the call has a use for the tail. A prompt re-hashes
// every open hunk. A call with a file path re-hashes the hunks of that path
// and, for a Write, looks for the pre-image. Any other call, such as Bash or
// Stop, has no path and sees no hunk, so the read would be wasted.
func (c *hookCall) needsTail() bool {
	return c.obs.Kind == EventPrompt || c.obs.FilePath != ""
}

func (c *hookCall) primaryEvents(tail []SessionEvent) ([]SessionEvent, error) {
	facts := c.hunkFacts(tail)
	if len(facts) == 0 {
		event, err := c.assembler.Assemble(c.obs)
		return []SessionEvent{event}, err
	}
	events := make([]SessionEvent, 0, len(facts))
	for _, hunk := range facts {
		obs := c.obs
		obs.Hunk = hunk
		event, err := c.assembler.Assemble(obs)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

// target is a file the call may read: inside the project, not redacted.
type target struct {
	relative string
	pathRef  string
}

func (c *hookCall) target() (target, bool) {
	if c.obs.FilePath == "" {
		return target{}, false
	}
	decision := c.paths.Decide(c.obs.FilePath)
	if decision.IsRedacted {
		return target{}, false
	}
	return target{
		relative: *decision.Display,
		pathRef:  c.hasher.Hash(HashPath, decision.HashInput),
	}, true
}

// hunkFacts derives the line ranges of the call. A file that is over the
// cap, unreadable or ambiguous gives no range: the event is recorded
// without one and the detector sees an unknown change.
func (c *hookCall) hunkFacts(tail []SessionEvent) []HunkFacts {
	file, isReadable := c.target()
	if !isReadable {
		return nil
	}
	switch {
	case c.obs.Kind == EventToolUse && c.obs.Tool == ToolWrite:
		return c.wholeFileFacts(file, "")
	case c.obs.Kind == EventToolResult && c.obs.Tool == ToolWrite:
		return c.wholeFileFacts(file, snapshotBeforeWrite(tail, file.pathRef))
	case c.isFileWriterResult():
		return c.editedLineFacts(file)
	}
	return nil
}

func (c *hookCall) isFileWriterResult() bool {
	return c.obs.Kind == EventToolResult && IsFileWriter(c.obs.Tool)
}

func (c *hookCall) wholeFileFacts(file target, preHash string) []HunkFacts {
	read := c.files.read(c.ctx, file.relative)
	if read.err != nil {
		return nil
	}
	facts, isInside := c.locator.WholeFile(read.splitLines())
	if !isInside {
		return nil
	}
	facts.PreHash = preHash
	return []HunkFacts{facts}
}

func (c *hookCall) editedLineFacts(file target) []HunkFacts {
	read := c.files.read(c.ctx, file.relative)
	text := c.deps.Edits.EditTextOf(c.payload)
	if read.err != nil || text.Len() == 0 {
		return nil
	}
	return c.locator.LocateAll(read.content, read.splitLines(), text)
}

// snapshotBeforeWrite finds the whole-file hash that the PreToolUse call of
// the same Write recorded. It is the pre-image of the write.
func snapshotBeforeWrite(tail []SessionEvent, pathRef string) string {
	for i := len(tail) - 1; i >= 0; i-- {
		event := tail[i]
		if event.Kind == EventToolUse && event.Tool == ToolWrite && event.PathRef == pathRef && event.HasHunk() {
			return event.PostHash
		}
	}
	return ""
}
