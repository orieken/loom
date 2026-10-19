package capture

import (
	"slices"
	"time"
)

// ledgerFromTail replays the recent events into a ledger of the hunks that
// are still open at now. It follows the same rules as the Detector, so the
// hook and the ingest agree on what an open hunk is. The last hash seen for
// a hunk is the hash of its newest observation.
func ledgerFromTail(events []SessionEvent, now time.Time, window time.Duration) *HunkLedger {
	ledger := NewHunkLedger()
	for position, event := range events {
		replayEvent(ledger, event, position+1, window)
	}
	ledger.expire(now, window)
	return ledger
}

func replayEvent(ledger *HunkLedger, event SessionEvent, position int, window time.Duration) {
	ledger.expire(event.At, window)
	switch {
	case event.Kind == EventToolResult && IsFileWriter(event.Tool) && event.PathRef != "":
		ledger.dropPath(event.PathRef, event.At)
		ledger.openFrom(event, position)
	case event.Kind == EventHunkObserved:
		ledger.updateLastHash(event)
	}
}

// openFrom opens the hunk that a file-writer result describes. An event
// without a usable line range opens nothing.
func (l *HunkLedger) openFrom(event SessionEvent, position int) {
	if !event.HasHunk() {
		return
	}
	l.open(openHunk{
		facts:         hunkFacts(event),
		writtenAt:     event.At,
		writePosition: position,
		tool:          event.Tool,
		path:          event.Path,
		pathRef:       event.PathRef,
	})
}

// updateLastHash stores the hash that an observation saw for its hunk. The
// next hook call compares against it, so an unchanged hunk writes nothing.
func (l *HunkLedger) updateLastHash(event SessionEvent) {
	index := l.indexOf(event.PathRef, event.RangeStart, event.RangeEnd)
	if index >= 0 {
		l.byPath[event.PathRef][index].facts.PostHash = event.PostHash
	}
}

// newest returns up to limit open hunks, the most recent write first. An
// empty pathRef selects the hunks of every path.
func (l *HunkLedger) newest(pathRef string, limit int) []openHunk {
	var selected []openHunk
	for ref, hunks := range l.byPath {
		if pathRef == "" || ref == pathRef {
			selected = append(selected, hunks...)
		}
	}
	slices.SortFunc(selected, func(a, b openHunk) int { return b.writePosition - a.writePosition })
	return selected[:min(len(selected), limit)]
}

// pathGroup holds the selected hunks of one file, so the file is read once.
type pathGroup struct {
	path  string
	hunks []openHunk
}

// groupByPath keeps the order of the first hunk of each path.
func groupByPath(hunks []openHunk) []pathGroup {
	var groups []pathGroup
	index := make(map[string]int)
	for _, hunk := range hunks {
		if hunk.path == nil {
			continue
		}
		position, isKnown := index[hunk.pathRef]
		if !isKnown {
			position = len(groups)
			index[hunk.pathRef] = position
			groups = append(groups, pathGroup{path: *hunk.path})
		}
		groups[position].hunks = append(groups[position].hunks, hunk)
	}
	return groups
}
