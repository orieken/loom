package capture

import "time"

// openHunk is an agent-written hunk that is still inside the Correction
// window and has no correction yet.
type openHunk struct {
	facts     HunkFacts
	writtenAt time.Time
	// writePosition is the stream position of the write event, so
	// attribution compares integers (performance-report.md finding 3).
	writePosition int
	tool          string
	path          *string
	pathRef       string
}

// HunkLedger holds the open hunks, grouped by salted path hash. Its size is
// proportional to the open hunks, never to the number of events. It is not
// safe for concurrent use.
type HunkLedger struct {
	byPath map[string][]openHunk
}

// NewHunkLedger returns an empty ledger.
func NewHunkLedger() *HunkLedger {
	return &HunkLedger{byPath: make(map[string][]openHunk)}
}

// Len counts the open hunks.
func (l *HunkLedger) Len() int {
	count := 0
	for _, hunks := range l.byPath {
		count += len(hunks)
	}
	return count
}

func (l *HunkLedger) open(hunk openHunk) {
	l.byPath[hunk.pathRef] = append(l.byPath[hunk.pathRef], hunk)
}

// dropPath forgets every hunk on a path. A later agent write on the path
// explains any change after it, so older hunks stop being human-edit
// candidates.
func (l *HunkLedger) dropPath(pathRef string) {
	delete(l.byPath, pathRef)
}

// expire forgets the hunks whose window ended before now. A hunk written at
// 12:00 stays open at 12:10:00 exactly (the window is inclusive).
func (l *HunkLedger) expire(now time.Time, window time.Duration) {
	for pathRef, hunks := range l.byPath {
		kept := hunks[:0]
		for _, hunk := range hunks {
			if now.Sub(hunk.writtenAt) <= window {
				kept = append(kept, hunk)
			}
		}
		l.store(pathRef, kept)
	}
}

func (l *HunkLedger) store(pathRef string, hunks []openHunk) {
	if len(hunks) == 0 {
		delete(l.byPath, pathRef)
		return
	}
	l.byPath[pathRef] = hunks
}

// find returns the open hunk with this path and range.
func (l *HunkLedger) find(pathRef string, start, end int) (openHunk, bool) {
	index := l.indexOf(pathRef, start, end)
	if index < 0 {
		return openHunk{}, false
	}
	return l.byPath[pathRef][index], true
}

// close removes the hunk with this path and range. A hunk leaves the ledger
// at its first correction.
func (l *HunkLedger) close(pathRef string, start, end int) {
	index := l.indexOf(pathRef, start, end)
	if index < 0 {
		return
	}
	hunks := l.byPath[pathRef]
	l.store(pathRef, append(hunks[:index:index], hunks[index+1:]...))
}

func (l *HunkLedger) indexOf(pathRef string, start, end int) int {
	for i, hunk := range l.byPath[pathRef] {
		if hunk.facts.RangeStart == start && hunk.facts.RangeEnd == end {
			return i
		}
	}
	return -1
}

func (l *HunkLedger) reset() {
	l.byPath = make(map[string][]openHunk)
}
