package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
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
	clock := localfs.SystemClock{}
	failures := jsonlstore.NewFailureLog(layout, clock)
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
// an error and exit 1, and a hook must do neither. The watchdog is the first
// act, so that nothing after it, not even a working-directory lookup, can
// hold the process past the stop time.
func runCaptureHook(cmd *cobra.Command, args []string) error {
	run := armHookRun(liveHookEnv(cmd.InOrStdin()), eventName(args))
	defer run.disarm()
	defer func() { _ = recover() }()
	if parts, isReady := livePartsOfFlags(cmd); isReady {
		run.supply(parts)
		run.execute()
	}
	return nil
}

// livePartsOfFlags wires the adapters for the root of the --root flag. A
// missing root gives no parts: there is nowhere to record anything.
func livePartsOfFlags(cmd *cobra.Command) (hookParts, bool) {
	root, _ := cmd.Flags().GetString(captureRootFlag)
	if root == "" {
		return hookParts{}, false
	}
	// A relative root would make every payload path look outside the project.
	if absolute, err := filepath.Abs(root); err == nil {
		root = absolute
	}
	return liveHookParts(root), true
}

// swallowFlagError makes a flag error silent. It logs the error as a flag
// failure in the project of the --root flag, or of the working directory when
// the flag has no value, and it lets the command end with exit 0.
func swallowFlagError(cmd *cobra.Command, _ error) error {
	root, _ := cmd.Flags().GetString(captureRootFlag)
	if root == "" {
		root = "."
	}
	jsonlstore.NewFailureLog(capture.NewLayout(root), localfs.SystemClock{}).Record(capture.CauseFlag)
	return nil
}

func eventName(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

// hookRun is one run of the hook with its watchdog. The parts arrive after
// the watchdog is armed, so the field is read and written through a pointer.
type hookRun struct {
	env   hookEnv
	parts atomic.Pointer[hookParts]
	event string
	timer *time.Timer
}

// startHookRun arms the watchdog and supplies the parts in one step.
func startHookRun(env hookEnv, parts hookParts, event string) *hookRun {
	run := armHookRun(env, event)
	run.supply(parts)
	return run
}

// armHookRun arms the watchdog as its first act, before any read, so a
// stdin that never closes cannot hang the process (performance-report.md
// mandate B).
func armHookRun(env hookEnv, event string) *hookRun {
	run := &hookRun{env: env, event: event}
	run.timer = time.AfterFunc(env.stopAfter, run.stopHard)
	return run
}

func (h *hookRun) supply(parts hookParts) { h.parts.Store(&parts) }

func (h *hookRun) disarm() { h.timer.Stop() }

// stopHard logs the stall in a separate goroutine and waits for it at most
// counterWait. It then exits 0 whether or not the write finished, because a
// blocked disk must not block the exit (mandate A). When the parts are not
// ready, there is no counter to write to, and the exit comes at once.
func (h *hookRun) stopHard() {
	if parts := h.parts.Load(); parts != nil {
		h.countStall(parts.counter)
	}
	h.env.exit(0)
}

func (h *hookRun) countStall(counter capture.FailureCounter) {
	written := make(chan struct{})
	go func() {
		counter.Record(capture.CauseStall)
		close(written)
	}()
	select {
	case <-written:
	case <-time.After(h.env.counterWait):
	}
}

// execute does the work of the hook. Every failure is logged and nothing
// is printed.
func (h *hookRun) execute() {
	parts := h.parts.Load()
	if parts == nil {
		return
	}
	defer countPanic(parts.counter)
	if parts.stall.IsStalled(parts.clock.Now()) {
		return
	}
	payload, err := h.readPayload()
	if err != nil {
		parts.counter.Record(capture.CauseDecode)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), h.env.stopAfter)
	defer cancel()
	if err := parts.record(ctx, h.event, payload); err != nil {
		parts.counter.Record(capture.CauseOf(err))
	}
}

func (h *hookRun) readPayload() ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(h.env.stdin, capture.MaxStdinBytes+1))
	if err == nil && len(payload) > capture.MaxStdinBytes {
		return nil, errPayloadTooLarge
	}
	return payload, err
}

func countPanic(counter capture.FailureCounter) {
	if recover() != nil {
		counter.Record(capture.CausePanic)
	}
}
