package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/orieken/loom/internal/capture"
	"github.com/orieken/loom/internal/capture/claudehook"
	"github.com/orieken/loom/internal/capture/jsonlstore"
	"github.com/orieken/loom/internal/capture/localfs"
	"github.com/spf13/cobra"
)

var errPayloadTooLarge = errors.New("capture: hook payload exceeds the stdin cap")

// hookEnv holds what a test replaces: the input, the exit and the times.
type hookEnv struct {
	stdin       io.Reader
	exit        func(code int)
	stopAfter   time.Duration
	counterWait time.Duration
}

// hookParts are the collaborators of the hook command, behind ports.
type hookParts struct {
	counter capture.FailureCounter
	stall   capture.StallMarker
	clock   capture.Clock
	record  func(ctx context.Context, event string, payload []byte) error
}

func liveHookEnv(stdin io.Reader) hookEnv {
	return hookEnv{
		stdin:       stdin,
		exit:        os.Exit,
		stopAfter:   capture.HookStopAfter,
		counterWait: capture.CounterWriteWait,
	}
}

// liveHookParts wires the adapters. It does no file access: every adapter
// opens its file when it is used.
func liveHookParts(root string) hookParts {
	layout := capture.NewLayout(root)
	failures := jsonlstore.NewFailureLog(layout)
	clock := localfs.SystemClock{}
	home, _ := os.UserHomeDir()
	recorder := capture.NewRecorder(capture.RecorderDeps{
		Layout:  layout,
		Clock:   clock,
		Sink:    jsonlstore.NewDaySink(layout),
		Tail:    jsonlstore.NewTail(layout),
		Files:   localfs.CappedReader{},
		Salts:   localfs.NewSaltFile(layout.SaltPath()),
		Config:  localfs.NewConfigFile(layout.ConfigPath()),
		Edits:   claudehook.EditTexts{},
		Decoder: claudehook.Decoder{},
		Purger:  jsonlstore.NewPurger(layout),
		Home:    home,
	})
	return hookParts{counter: failures, stall: failures, clock: clock, record: recorder.Record}
}

// runCaptureHook never returns an error and never prints: cobra would print
// an error and exit 1, and a hook must do neither.
func runCaptureHook(cmd *cobra.Command, args []string) error {
	root, _ := cmd.Flags().GetString(captureRootFlag)
	if root == "" {
		return nil
	}
	// A relative root would make every payload path look outside the project.
	if absolute, err := filepath.Abs(root); err == nil {
		root = absolute
	}
	run := startHookRun(liveHookEnv(cmd.InOrStdin()), liveHookParts(root), eventName(args))
	defer run.disarm()
	run.execute()
	return nil
}

func eventName(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

// hookRun is one run of the hook with its watchdog.
type hookRun struct {
	env   hookEnv
	parts hookParts
	event string
	timer *time.Timer
}

// startHookRun arms the watchdog as its first act, before any read, so a
// stdin that never closes cannot hang the process (performance-report.md
// mandate B).
func startHookRun(env hookEnv, parts hookParts, event string) *hookRun {
	run := &hookRun{env: env, parts: parts, event: event}
	run.timer = time.AfterFunc(env.stopAfter, run.stopHard)
	return run
}

func (h *hookRun) disarm() { h.timer.Stop() }

// stopHard counts the stall in a separate goroutine and waits for it at most
// counterWait. It then exits 0 whether or not the write finished, because a
// blocked disk must not block the exit (mandate A).
func (h *hookRun) stopHard() {
	written := make(chan struct{})
	go func() {
		h.parts.counter.Record(capture.CauseTimeout)
		close(written)
	}()
	select {
	case <-written:
	case <-time.After(h.env.counterWait):
	}
	h.env.exit(0)
}

// execute does the work of the hook. Every failure is counted and nothing
// is printed.
func (h *hookRun) execute() {
	defer h.countPanic()
	if h.parts.stall.IsStalled(h.parts.clock.Now()) {
		return
	}
	payload, err := h.readPayload()
	if err != nil {
		h.parts.counter.Record(capture.CauseDecode)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), h.env.stopAfter)
	defer cancel()
	if err := h.parts.record(ctx, h.event, payload); err != nil {
		h.parts.counter.Record(capture.CauseOf(err))
	}
}

func (h *hookRun) readPayload() ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(h.env.stdin, capture.MaxStdinBytes+1))
	if err == nil && len(payload) > capture.MaxStdinBytes {
		return nil, errPayloadTooLarge
	}
	return payload, err
}

func (h *hookRun) countPanic() {
	if recover() != nil {
		h.parts.counter.Record(capture.CausePanic)
	}
}
