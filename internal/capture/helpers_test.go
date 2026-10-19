package capture

import (
	"fmt"
	"testing"
	"time"
)

// testSaltText is a printable salt, so a test can search output for it.
const testSaltText = "0123456789abcdef0123456789abcdef"

var baseTime = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func newTestSalt(t *testing.T) Salt {
	t.Helper()
	salt, err := NewSalt([]byte(testSaltText))
	if err != nil {
		t.Fatalf("NewSalt: %v", err)
	}
	return salt
}

func newTestRedactor(t *testing.T, extra ...string) Redactor {
	t.Helper()
	redactor, err := NewRedactor(extra, "/home/user")
	if err != nil {
		t.Fatalf("NewRedactor: %v", err)
	}
	return redactor
}

func counterIDs() IDSource {
	next := 0
	return func() (string, error) {
		next++
		return fmt.Sprintf("id-%d", next), nil
	}
}

func newTestAssembler(t *testing.T, root string) *Assembler {
	t.Helper()
	return NewAssembler(AssemblerDeps{
		Hasher:     NewHasher(newTestSalt(t)),
		Paths:      NewPathPolicy(root, newTestRedactor(t)),
		Clock:      fixedClock{now: baseTime},
		NewEventID: counterIDs(),
	})
}

func at(offset time.Duration) time.Time { return baseTime.Add(offset) }
