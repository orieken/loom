package capture

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// IDSource gives the identifier of a new event.
type IDSource func() (string, error)

const eventIDBytes = 8

// RandomID returns a 64-bit random identifier as hex, for tracing.
func RandomID() (string, error) {
	raw := make([]byte, eventIDBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("capture: event id: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// AssemblerDeps are the collaborators of an Assembler.
type AssemblerDeps struct {
	Hasher     *Hasher
	Paths      PathPolicy
	Clock      Clock
	NewEventID IDSource
}

// Assembler turns an Observation into a SessionEvent. It is the one place
// where untrusted text is reduced to properties: it hashes the session id
// and the path, applies the path rules and replaces the prompt with a class
// label. The prompt text is never copied.
type Assembler struct {
	deps       AssemblerDeps
	classifier PromptClassifier
}

// NewAssembler builds an Assembler.
func NewAssembler(deps AssemblerDeps) *Assembler {
	return &Assembler{deps: deps}
}

// Assemble builds the event of one observation. The time comes from the
// injected clock, never from the payload.
func (a *Assembler) Assemble(obs Observation) (SessionEvent, error) {
	id, err := a.deps.NewEventID()
	if err != nil {
		return SessionEvent{}, err
	}
	event := NewSessionEvent(id, a.deps.Clock.Now(), obs.Kind).
		WithTool(SafeToolName(obs.Tool)).
		WithArgShape(safeKeys(obs.ArgKeys), obs.ArgTypes)
	event = a.withSession(event, obs.SessionID)
	event = a.withFile(event, obs)
	return a.withPromptClass(event, obs), nil
}

func (a *Assembler) withSession(event SessionEvent, sessionID string) SessionEvent {
	if sessionID == "" {
		return event
	}
	return event.WithSessionRef(a.deps.Hasher.Hash(HashSession, sessionID))
}

// withFile stores the path hash always, the readable path and the line range
// only for a path that is not redacted.
func (a *Assembler) withFile(event SessionEvent, obs Observation) SessionEvent {
	if obs.FilePath == "" {
		return event
	}
	decision := a.deps.Paths.Decide(obs.FilePath)
	event = event.WithPath(a.deps.Hasher.Hash(HashPath, decision.HashInput), decision.Display)
	if decision.IsRedacted {
		return event
	}
	return event.WithHunk(obs.Hunk)
}

func (a *Assembler) withPromptClass(event SessionEvent, obs Observation) SessionEvent {
	if obs.Kind != EventPrompt {
		return event
	}
	return event.WithPromptClass(a.classifier.Classify(obs.Prompt))
}

func safeKeys(keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	safe := make([]string, len(keys))
	for i, key := range keys {
		safe[i] = key
		if !IsSafeKeyName(key) {
			safe[i] = UnsafeNamePlaceholder
		}
	}
	return safe
}
