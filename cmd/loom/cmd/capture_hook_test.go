package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
	"github.com/spf13/cobra"
)

// exitProbe records the exit code and the moment of the exit.
type exitProbe struct {
	mu    sync.Mutex
	codes []int
	at    time.Time
	done  chan struct{}
	once  sync.Once
}

func newExitProbe() *exitProbe { return &exitProbe{done: make(chan struct{})} }

func (p *exitProbe) exit(code int) {
	p.mu.Lock()
	p.codes = append(p.codes, code)
	p.at = time.Now()
	p.mu.Unlock()
	p.once.Do(func() { close(p.done) })
}

func (p *exitProbe) exitedWithin(limit time.Duration) bool {
	select {
	case <-p.done:
		return true
	case <-time.After(limit):
		return false
	}
}

// causeLog is a FailureCounter that remembers the causes and can block.
type causeLog struct {
	mu      sync.Mutex
	causes  []capture.FailureCause
	blocked chan struct{} // when set, Record blocks until it is closed
}

func (c *causeLog) Record(cause capture.FailureCause) {
	c.mu.Lock()
	c.causes = append(c.causes, cause)
	blocked := c.blocked
	c.mu.Unlock()
	if blocked != nil {
		<-blocked
	}
}

func (c *causeLog) seen() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out strings.Builder
	for _, cause := range c.causes {
		out.WriteByte(byte(cause))
	}
	return out.String()
}

type fixedStall bool

func (s fixedStall) IsStalled(time.Time) bool { return bool(s) }

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Now() }

// newParts builds fake parts. record is the use case, counter the failure log.
func newParts(record func(context.Context, string, []byte) error, counter capture.FailureCounter, stall bool) hookParts {
	return hookParts{counter: counter, stall: fixedStall(stall), clock: fixedClock{}, record: record}
}

func fastEnv(stdin io.Reader, probe *exitProbe) hookEnv {
	return hookEnv{stdin: stdin, exit: probe.exit, stopAfter: 80 * time.Millisecond, counterWait: 30 * time.Millisecond}
}

// neverClosingReader blocks every Read until released, like a client that
// keeps stdin open.
type neverClosingReader struct{ release chan struct{} }

func (r neverClosingReader) Read([]byte) (int, error) {
	<-r.release
	return 0, io.EOF
}

func TestHookHardStopExitsZeroWhenTheSinkAndTheCounterBothBlock(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	counter := &causeLog{blocked: release}
	probe := newExitProbe()
	record := func(_ context.Context, _ string, _ []byte) error {
		<-release // the sink blocks, whatever the context says
		return nil
	}
	env := liveHookEnv(strings.NewReader("{}"))
	env.exit = probe.exit
	start := time.Now()

	run := startHookRun(env, newParts(record, counter, false), "PostToolUse")
	defer run.disarm()
	go run.execute()

	limit := capture.HookStopAfter + capture.CounterWriteWait
	if !probe.exitedWithin(limit + 300*time.Millisecond) {
		t.Fatal("the hook did not exit although the sink and the counter both block")
	}
	elapsed := probe.at.Sub(start)
	if elapsed < capture.HookStopAfter || elapsed > limit+startupSlack {
		t.Fatalf("exited after %v, want between %v and %v", elapsed, capture.HookStopAfter, limit+startupSlack)
	}
	if probe.codes[0] != 0 || counter.seen() != "t" {
		t.Fatalf("exit codes %v, counted %q; want exit 0 and one stall byte", probe.codes, counter.seen())
	}
}

func TestHookWatchdogIsArmedBeforeStdinIsRead(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	probe := newExitProbe()
	counter := &causeLog{}
	record := func(context.Context, string, []byte) error { return nil }

	run := startHookRun(fastEnv(neverClosingReader{release}, probe), newParts(record, counter, false), "Stop")
	defer run.disarm()
	go run.execute()

	if !probe.exitedWithin(time.Second) {
		t.Fatal("a stdin that never closes must not hang the hook")
	}
	if counter.seen() != "t" {
		t.Fatalf("counted %q, want the stall byte", counter.seen())
	}
}

func TestHookExitWaitsForTheCounterWriteWhenItIsFast(t *testing.T) {
	probe := newExitProbe()
	counter := &causeLog{}
	release := make(chan struct{})
	defer close(release)
	record := func(context.Context, string, []byte) error { <-release; return nil }

	run := startHookRun(fastEnv(strings.NewReader("{}"), probe), newParts(record, counter, false), "Stop")
	defer run.disarm()
	go run.execute()

	if !probe.exitedWithin(time.Second) || counter.seen() != "t" {
		t.Fatalf("counted %q: the fast counter write must land before the exit", counter.seen())
	}
}

func TestHookDisarmCancelsTheWatchdog(t *testing.T) {
	probe := newExitProbe()
	record := func(context.Context, string, []byte) error { return nil }

	run := startHookRun(fastEnv(strings.NewReader("{}"), probe), newParts(record, &causeLog{}, false), "Stop")
	run.execute()
	run.disarm()

	if probe.exitedWithin(300 * time.Millisecond) {
		t.Fatal("a finished hook must not be stopped by its own watchdog")
	}
}

func TestHookRunTable(t *testing.T) {
	bigPayload := strings.Repeat("x", capture.MaxStdinBytes+1)
	tests := []struct {
		name        string
		stdin       io.Reader
		stall       bool
		record      func(context.Context, string, []byte) error
		wantCauses  string
		wantRecords int
	}{
		{name: "success counts nothing", stdin: strings.NewReader("{}"),
			record: func(context.Context, string, []byte) error { return nil }, wantRecords: 1},
		{name: "a failure counts its cause", stdin: strings.NewReader("{}"),
			record: func(context.Context, string, []byte) error {
				return &capture.RecordError{Cause: capture.CauseSalt, Err: errors.New("x")}
			},
			wantCauses: "s", wantRecords: 1},
		{name: "an error with no cause counts as append", stdin: strings.NewReader("{}"),
			record:     func(context.Context, string, []byte) error { return errors.New("plain") },
			wantCauses: "a", wantRecords: 1},
		{name: "stdin over the cap counts a decode failure", stdin: strings.NewReader(bigPayload),
			record: func(context.Context, string, []byte) error { return nil }, wantCauses: "d"},
		{name: "a stdin read error counts a decode failure", stdin: errReader{},
			record: func(context.Context, string, []byte) error { return nil }, wantCauses: "d"},
		{name: "a panic is counted and swallowed", stdin: strings.NewReader("{}"),
			record:     func(context.Context, string, []byte) error { panic("boom") },
			wantCauses: "p", wantRecords: 1},
		{name: "an open breaker does no work", stdin: strings.NewReader("{}"), stall: true,
			record: func(context.Context, string, []byte) error { return nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := &causeLog{}
			records := 0
			record := func(ctx context.Context, event string, payload []byte) error {
				records++
				return tt.record(ctx, event, payload)
			}
			stdin := &trackedReader{Reader: tt.stdin}
			env := liveHookEnv(stdin)
			env.exit = func(int) { t.Error("the hook must not be stopped") }
			run := startHookRun(env, newParts(record, counter, tt.stall), "PostToolUse")
			defer run.disarm()

			run.execute()

			if counter.seen() != tt.wantCauses || records != tt.wantRecords {
				t.Fatalf("counted %q with %d records; want %q with %d", counter.seen(), records, tt.wantCauses, tt.wantRecords)
			}
			if tt.stall && stdin.reads != 0 {
				t.Fatal("an open breaker must not even read stdin")
			}
		})
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("broken pipe") }

type trackedReader struct {
	io.Reader
	reads int
}

func (r *trackedReader) Read(p []byte) (int, error) {
	r.reads++
	return r.Reader.Read(p)
}

func TestHookPassesTheEventAndThePayloadToTheUseCase(t *testing.T) {
	var gotEvent string
	var gotPayload []byte
	record := func(_ context.Context, event string, payload []byte) error {
		gotEvent, gotPayload = event, payload
		return nil
	}
	env := liveHookEnv(strings.NewReader(`{"a":1}`))
	env.exit = func(int) {}

	run := startHookRun(env, newParts(record, &causeLog{}, false), "UserPromptSubmit")
	defer run.disarm()
	run.execute()

	if gotEvent != "UserPromptSubmit" || !bytes.Equal(gotPayload, []byte(`{"a":1}`)) {
		t.Fatalf("event %q, payload %q", gotEvent, gotPayload)
	}
}

func TestHookGivesTheUseCaseADeadline(t *testing.T) {
	var deadline time.Time
	var hasDeadline bool
	record := func(ctx context.Context, _ string, _ []byte) error {
		deadline, hasDeadline = ctx.Deadline()
		return nil
	}
	env := liveHookEnv(strings.NewReader("{}"))
	env.exit = func(int) {}

	run := startHookRun(env, newParts(record, &causeLog{}, false), "Stop")
	defer run.disarm()
	run.execute()

	if !hasDeadline || time.Until(deadline) > capture.HookStopAfter {
		t.Fatalf("deadline = %v (set %v), want one within %v", deadline, hasDeadline, capture.HookStopAfter)
	}
}

func TestEventNameTakesTheFirstArgument(t *testing.T) {
	if eventName(nil) != "" || eventName([]string{"Stop", "extra"}) != "Stop" {
		t.Fatal("the event is the first argument, and a missing one is empty")
	}
}

func TestRunCaptureHookWithoutARootDoesNothingAndReturnsNil(t *testing.T) {
	command := &cobra.Command{}
	command.Flags().String(captureRootFlag, "", "")
	command.SetIn(strings.NewReader("{}"))

	if err := runCaptureHook(command, []string{"Stop"}); err != nil {
		t.Fatalf("err = %v, want nil: a hook never fails", err)
	}
}

func TestRunCaptureHookNeverReturnsAnError(t *testing.T) {
	root := t.TempDir() // no .claude/memory: every step fails
	command := &cobra.Command{}
	command.Flags().String(captureRootFlag, root, "")
	command.SetIn(strings.NewReader("not json"))

	if err := runCaptureHook(command, []string{"PostToolUse"}); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

func TestCaptureCommandsAreHiddenAndSilent(t *testing.T) {
	if !captureCmd.Hidden || !captureHookCmd.Hidden {
		t.Fatal("capture commands must stay out of --help")
	}
	if !captureHookCmd.SilenceErrors || !captureHookCmd.SilenceUsage || !captureHookCmd.FParseErrWhitelist.UnknownFlags {
		t.Fatal("the hook command must silence cobra and accept unknown flags")
	}
	if flag := captureHookCmd.Flags().Lookup(captureRootFlag); flag == nil {
		t.Fatal("the hook command needs the --root flag")
	}
}

func TestHookWatchdogStopsTheProcessEvenWhenThePartsNeverArrive(t *testing.T) {
	probe := newExitProbe()

	run := armHookRun(fastEnv(strings.NewReader("{}"), probe), "Stop")
	defer run.disarm()

	if !probe.exitedWithin(time.Second) || probe.codes[0] != 0 {
		t.Fatalf("exit codes %v: an armed watchdog must exit 0 although the parts are not ready", probe.codes)
	}
}

func TestHookExecuteWithoutPartsDoesNothing(t *testing.T) {
	probe := newExitProbe()
	run := armHookRun(fastEnv(strings.NewReader("{}"), probe), "Stop")
	defer run.disarm()

	run.execute()

	if probe.exitedWithin(10 * time.Millisecond) {
		t.Fatal("execute without parts must return at once and leave the exit to the watchdog")
	}
}

func TestSwallowFlagErrorCountsADecodeFailureAndReturnsNil(t *testing.T) {
	root := t.TempDir()
	project := &captureProject{t: t, root: root, layout: capture.NewLayout(root)}
	project.write(".claude/memory/placeholder", "", 0o600)
	command := &cobra.Command{}
	command.Flags().String(captureRootFlag, root, "")

	err := swallowFlagError(command, errors.New("flag needs an argument"))

	if err != nil || project.failures() != "d" {
		t.Fatalf("err = %v, counter = %q, want nil and one decode failure", err, project.failures())
	}
}
