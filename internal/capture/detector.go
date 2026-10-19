package capture

// Detector finds Session Corrections in an ordered stream of Session Events.
// It is a streaming forward pass (architecture-notes.md decision 9):
// Observe takes one event and returns the corrections that event completes,
// Close ends the stream. Memory is proportional to the open hunks.
//
// The Detector reads no clock. Every event carries its own UTC time, so the
// result is a pure function of the stream and the Config. Feed the events in
// day-file order, then position in file. Never sort by time.
//
// The four signals:
//   - hunk-edit: a person changed the lines of an agent-written hunk inside
//     the Correction window;
//   - revert: the lines returned to their pre-agent hash;
//   - denied-call: a tool call ended in a denial (off by default);
//   - corrective-prompt: a prompt opens with a corrective phrase.
//
// A hunk closes after its first correction. One hunk therefore gives at most
// one correction. A revert after a partial edit of the same hunk does not
// count again; the hunk-edit already counted it.
//
// It is not safe for concurrent use.
type Detector struct {
	cfg          Config
	ledger       *HunkLedger
	position     int
	lastBashSeen int
}

// NewDetector builds a Detector for one stream.
func NewDetector(cfg Config) *Detector {
	return &Detector{cfg: cfg, ledger: NewHunkLedger()}
}

// OpenHunks counts the hunks still inside their Correction window.
func (d *Detector) OpenHunks() int { return d.ledger.Len() }

// Observe feeds the next event and returns the corrections it completes.
// The result holds at most one correction. A correction of a signal that is
// switched off is not returned.
func (d *Detector) Observe(event SessionEvent) []Correction {
	d.position++
	d.ledger.expire(event.At, d.cfg.Window)
	switch event.Kind {
	case EventToolResult:
		d.noteToolResult(event)
	case EventHunkObserved:
		return d.allowed(d.judgeObservation(event))
	case EventToolDenied:
		return d.allowed(deniedCall(event))
	case EventPrompt:
		return d.allowed(correctivePrompt(event))
	}
	return nil
}

// Close ends the stream and forgets the open hunks. All corrections are
// already returned by Observe, so the result is empty. The method exists so
// a caller treats the Detector as a stream.
func (d *Detector) Close() []Correction {
	d.ledger.reset()
	return nil
}

func (d *Detector) allowed(correction *Correction) []Correction {
	if correction == nil || !d.cfg.Signals.IsOn(correction.Signal) {
		return nil
	}
	return []Correction{*correction}
}

// noteToolResult records what an agent tool result means for attribution.
func (d *Detector) noteToolResult(event SessionEvent) {
	switch {
	case IsShell(event.Tool):
		d.lastBashSeen = d.position
	case IsFileWriter(event.Tool) && event.PathRef != "":
		d.ledger.dropPath(event.PathRef, event.At)
		d.openHunkFrom(event)
	}
}

func (d *Detector) openHunkFrom(event SessionEvent) {
	d.ledger.openFrom(event, d.position)
}

func hunkFacts(event SessionEvent) HunkFacts {
	return HunkFacts{
		RangeStart: event.RangeStart,
		RangeEnd:   event.RangeEnd,
		PreHash:    event.PreHash,
		PostHash:   event.PostHash,
	}
}

// judgeObservation compares an observed hash with the hunk it belongs to.
func (d *Detector) judgeObservation(event SessionEvent) *Correction {
	hunk, isOpen := d.ledger.find(event.PathRef, event.RangeStart, event.RangeEnd)
	if !isOpen || event.PostHash == hunk.facts.PostHash {
		return nil
	}
	d.ledger.close(event.PathRef, event.RangeStart, event.RangeEnd)
	kind, signal := d.classifyChange(hunk, event.PostHash)
	return &Correction{
		Kind: kind, Signal: signal, At: event.At,
		Tool: hunk.tool, Path: hunk.path, PathRef: hunk.pathRef,
	}
}

// classifyChange attributes a changed hash. A Bash result after the hunk
// write may explain the change, so the author is unknown. Otherwise a hash
// equal to the pre-image is a revert, and any other hash is a hunk edit.
func (d *Detector) classifyChange(hunk openHunk, observedHash string) (CorrectionKind, Signal) {
	switch {
	case d.lastBashSeen > hunk.writePosition:
		return KindUnknown, SignalHunkEdit
	case observedHash == hunk.facts.PreHash:
		return KindRevert, SignalRevert
	}
	return KindHunkEdit, SignalHunkEdit
}

func deniedCall(event SessionEvent) *Correction {
	return &Correction{
		Kind: KindDeniedCall, Signal: SignalDeniedCall,
		At: event.At, Tool: event.Tool,
	}
}

func correctivePrompt(event SessionEvent) *Correction {
	if event.PromptClass != PromptCorrective {
		return nil
	}
	return &Correction{
		Kind: KindCorrectivePrompt, Signal: SignalCorrectivePrompt, At: event.At,
	}
}
