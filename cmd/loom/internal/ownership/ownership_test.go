package ownership_test

import (
	"testing"

	"github.com/orieken/loom/cmd/loom/internal/ownership"
)

func TestDecideWritesWhenNothingOccupiesThePath(t *testing.T) {
	ledger := ownership.Ledger{}
	if decision := ledger.Decide(".claude/agents/analyst.md", ownership.State{}); decision != ownership.Write {
		t.Fatalf("empty destination: got %v, want Write", decision)
	}
}

// The case L3.26 exists for: a team's own agent sitting where loom is about
// to install, which no prior install recorded.
func TestDecideSkipsAFileLoomNeverInstalled(t *testing.T) {
	ledger := ownership.Ledger{".claude/agents/analyst.md": ownership.Digest([]byte("ours"))}
	state := ownership.State{Exists: true, Digest: ownership.Digest([]byte("theirs"))}

	decision := ledger.Decide(".claude/agents/our-team-reviewer.md", state)

	if decision != ownership.Skip {
		t.Fatalf("foreign file: got %v, want Skip", decision)
	}
	if ownership.Reason(decision) == "" {
		t.Fatal("a skip must carry a reason a human can act on")
	}
}

func TestDecideUpdatesAPathLoomOwnsAndNobodyEdited(t *testing.T) {
	content := []byte("agent definition")
	ledger := ownership.Ledger{".claude/agents/analyst.md": ownership.Digest(content)}
	state := ownership.State{Exists: true, Digest: ownership.Digest(content)}

	if decision := ledger.Decide(".claude/agents/analyst.md", state); decision != ownership.Update {
		t.Fatalf("unmodified owned file: got %v, want Update", decision)
	}
}

func TestDecideSkipsAnOwnedPathTheProjectHasEdited(t *testing.T) {
	ledger := ownership.Ledger{".claude/agents/analyst.md": ownership.Digest([]byte("as installed"))}
	state := ownership.State{Exists: true, Digest: ownership.Digest([]byte("locally customised"))}

	decision := ledger.Decide(".claude/agents/analyst.md", state)

	if decision != ownership.SkipModified {
		t.Fatalf("locally edited file: got %v, want SkipModified", decision)
	}
	if !ownership.IsSkip(decision) {
		t.Fatal("SkipModified must count as a skip")
	}
}

// An unreadable destination reports an empty digest. It must not compare
// equal to a recorded digest, or an unreadable file would be treated as
// unmodified and silently overwritten.
func TestDecideTreatsAnUnreadableDestinationAsModified(t *testing.T) {
	ledger := ownership.Ledger{"ARCHITECTURE_RULES.md": ownership.Digest([]byte("content"))}
	state := ownership.State{Exists: true, Digest: ""}

	if decision := ledger.Decide("ARCHITECTURE_RULES.md", state); decision != ownership.SkipModified {
		t.Fatalf("unreadable destination: got %v, want SkipModified", decision)
	}
}

func TestDigestDistinguishesContent(t *testing.T) {
	if ownership.Digest([]byte("a")) == ownership.Digest([]byte("b")) {
		t.Fatal("different content must not share a digest")
	}
}
