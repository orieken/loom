package capture

import (
	"context"
	"time"
)

// Clock gives the current instant. The detector never calls it: every event
// carries its own UTC time.
type Clock interface {
	Now() time.Time
}

// EventSink appends the events of one hook call. An implementation writes
// all lines of one call with one Write.
type EventSink interface {
	Append(ctx context.Context, events []SessionEvent) error
}

// FileReader reads at most maxBytes of a file. An implementation checks the
// size of the open handle first and refuses a file above the cap.
type FileReader interface {
	ReadCapped(ctx context.Context, path string, maxBytes int64) ([]byte, error)
}

// SaltSource gives the Capture salt. A missing or malformed salt is an
// error. An implementation never creates a salt.
type SaltSource interface {
	Salt() (Salt, error)
}

// PayloadDecoder turns one untrusted hook payload into an Observation.
// It is the anti-corruption boundary for the tool that sends the payload.
type PayloadDecoder interface {
	Decode(hookEvent string, payload []byte) (Observation, error)
}
