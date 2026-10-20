package capture

import (
	"context"
	"errors"
	"time"
)

// ObserverTool is the tool name of a hunk-observed event. The hook writes
// that event, not an agent tool.
const ObserverTool = "capture-hook"

// RecorderDeps are the collaborators of a Recorder. Every one is a port or
// a plain value, so the use case does no file system or clock access.
type RecorderDeps struct {
	Layout     Layout
	Clock      Clock
	Sink       EventSink
	Tail       EventTail
	Files      FileReader
	Salts      SaltSource
	Config     ConfigSource
	Edits      EditTextSource
	Decoder    PayloadDecoder
	Purger     RetentionPurger
	NewEventID IDSource
	// Home expands "~/" in the redaction globs. It may be empty.
	Home string
}

// Recorder is the hook use case: it turns one hook payload into the events
// of one day-file append. It does the work of architecture-notes.md
// decision 3, steps 4 to 12, and nothing else.
type Recorder struct {
	deps RecorderDeps
}

// NewRecorder builds a Recorder. A missing event id source gets RandomID.
func NewRecorder(deps RecorderDeps) *Recorder {
	if deps.NewEventID == nil {
		deps.NewEventID = RandomID
	}
	return &Recorder{deps: deps}
}

// Record handles one hook call. The error carries the failure cause to
// count (see CauseOf). A failure writes no event.
func (r *Recorder) Record(ctx context.Context, hookEvent string, payload []byte) error {
	obs, err := r.deps.Decoder.Decode(hookEvent, payload)
	if err != nil {
		return failed(CauseDecode, err)
	}
	recordErr := r.recordEvents(ctx, obs, payload)
	return errors.Join(recordErr, r.purgeOnStop(ctx, obs))
}

func (r *Recorder) recordEvents(ctx context.Context, obs Observation, payload []byte) error {
	call, err := r.newCall(ctx, obs, payload)
	if err != nil {
		return err
	}
	events, err := call.events()
	if err != nil {
		return failed(CauseWrite, err)
	}
	if err := r.deps.Sink.Append(ctx, events); err != nil {
		return failed(CauseWrite, err)
	}
	return nil
}

func (r *Recorder) newCall(ctx context.Context, obs Observation, payload []byte) (*hookCall, error) {
	cfg, err := r.deps.Config.Load()
	if err != nil {
		return nil, failed(CauseConfig, err)
	}
	salt, err := r.deps.Salts.Salt()
	if err != nil {
		return nil, failed(CauseSalt, err)
	}
	call, err := r.buildCall(ctx, cfg, salt)
	if err != nil {
		return nil, failed(CauseConfig, err)
	}
	call.obs, call.payload = obs, payload
	return call, nil
}

// purgeOnStop runs the retention purge at the end of a session turn. The
// purge checks its own stamp, so a call that is not due costs one stat.
func (r *Recorder) purgeOnStop(ctx context.Context, obs Observation) error {
	if obs.Kind != EventStop {
		return nil
	}
	cfg, err := r.deps.Config.Load()
	if err != nil {
		return failed(CauseConfig, err)
	}
	if err := r.deps.Purger.PurgeIfDue(ctx, r.deps.Clock.Now(), cfg.Retention); err != nil {
		return failed(CauseWrite, err)
	}
	return nil
}

// instantClock gives the same instant to every event of one call, so the
// events of a MultiEdit share one time and one day file.
type instantClock struct {
	at time.Time
}

func (c instantClock) Now() time.Time { return c.at }
