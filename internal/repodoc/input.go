package repodoc

import (
	"encoding/json"
	"fmt"

	"github.com/orieken/loom/internal/contextindex"
	"github.com/orieken/loom/internal/state"
)

// Names of the input blocks the agent receives. They are the labels in the
// prompt ("From fact-sheet:"), so they are part of the agent's contract.
const (
	inputFactSheet      = "fact-sheet"
	inputChildSummaries = "child-summaries"
	inputAllowedTags    = "allowed-tags"
	inputFeedback       = "revision-feedback"
	inputCandidate      = "candidate"
)

// ChildSummary is what a parent is shown about one child directory.
type ChildSummary struct {
	Path          string   `json:"path"`
	Status        string   `json:"status"`
	Purpose       string   `json:"purpose,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	Layer         string   `json:"layer,omitempty"`
	OpenQuestions []string `json:"openQuestions,omitempty"`
}

const (
	summaryDocumented   = "documented"
	summaryUndocumented = "undocumented"
)

// summaries gathers child summaries, preferring what this run produced over
// what the index recorded earlier. A child with neither is listed as
// undocumented, so the parent's agent knows it is working without it.
type summaries struct {
	thisRun  map[string]state.RepoDocState
	recorded contextindex.Index
}

func (s summaries) forChildren(children []string) []ChildSummary {
	out := make([]ChildSummary, 0, len(children))
	for _, child := range children {
		out = append(out, s.one(child))
	}
	return out
}

func (s summaries) one(child string) ChildSummary {
	if doc, ok := s.thisRun[child]; ok {
		return ChildSummary{Path: child, Status: summaryDocumented, Purpose: doc.Purpose, Tags: doc.Tags,
			Layer: string(doc.Layer), OpenQuestions: doc.OpenQuestions}
	}
	if entry, ok := s.recorded.Entries[child]; ok && entry.InputHash != "" {
		return ChildSummary{Path: child, Status: summaryDocumented, Purpose: entry.Purpose, Tags: entry.Tags,
			Layer: entry.Layer, OpenQuestions: entry.OpenQuestions}
	}
	return ChildSummary{Path: child, Status: summaryUndocumented}
}

// buildInput assembles what one invocation is given: the fact sheet, the
// children's summaries and the tags it may use. Never raw source.
func buildInput(task Task, known summaries, vocabulary Vocabulary, feedback *Feedback) (map[string][]byte, error) {
	sheet, err := json.MarshalIndent(task.Sheet, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode fact sheet %s: %w", task.Path, err)
	}
	children, err := json.MarshalIndent(known.forChildren(task.Sheet.Children), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode child summaries %s: %w", task.Path, err)
	}
	tags, err := json.MarshalIndent(vocabulary.Tags(), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode tags: %w", err)
	}
	input := map[string][]byte{inputFactSheet: sheet, inputChildSummaries: children, inputAllowedTags: tags}
	if feedback == nil {
		return input, nil
	}
	revision, err := json.MarshalIndent(feedback, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode revision feedback %s: %w", task.Path, err)
	}
	input[inputFeedback] = revision
	return input, nil
}

// Feedback is what a documenter is shown on a revision round: the attempt it
// made and every problem found with it, typed so each can be acted on.
type Feedback struct {
	Round    int                 `json:"round"`
	Previous *state.RepoDocState `json:"previous,omitempty"`
	Findings []Violation         `json:"findings"`
}
