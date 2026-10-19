package capture

// HunkFacts are the line range of an edited hunk and the salted hashes of
// its lines before and after the change. The hook computes them. A zero
// value means no hunk.
type HunkFacts struct {
	RangeStart int
	RangeEnd   int
	PreHash    string
	PostHash   string
}

// Observation is the neutral, in-memory result of decoding one hook payload.
// It may hold prompt text, so it has no JSON tags, no encoder and no
// String method. Nothing serializes it: the Assembler copies only
// properties into a SessionEvent (architecture-notes.md decision 4).
type Observation struct {
	Kind      EventKind
	Tool      string
	SessionID string
	FilePath  string
	Prompt    string
	ArgKeys   []string
	ArgTypes  []string
	Hunk      HunkFacts
}
