package factory

import (
	"context"
	"fmt"
	"io"
	"time"
)

// Defaults for a supervisor's knobs.
const (
	DefaultMaxConcurrent = 2
	DefaultInterval      = 30 * time.Second
	DefaultRetryBase     = time.Minute
	DefaultRetryMax      = 30 * time.Minute
	DefaultProvider      = "claude"
)

// Config is how one supervisor runs.
type Config struct {
	Layout Layout
	// IntakeBranch is where committed orders are read from (ADR-010, 2B).
	IntakeBranch  string
	MaxConcurrent int
	// Provider and RunArgs are passed to every `loom run`.
	Provider string
	RunArgs  []string
	// Plans are the plan names an order may name.
	Plans     []string
	RetryBase time.Duration
	RetryMax  time.Duration
}

// Dependencies are everything the supervisor reaches outside itself.
type Dependencies struct {
	Repository Repository
	Launcher   Launcher
	Trees      TreeOpener
	Pulls      PullRequests
	Output     io.Writer
	Now        func() time.Time
}

// Supervisor claims, launches, settles and finishes orders. One tick does
// one pass; Run ticks until its context ends.
type Supervisor struct {
	config    Config
	deps      Dependencies
	ledger    *Ledger
	processes map[string]Process
	// ended is signalled whenever a run this supervisor started exits.
	ended chan struct{}
	// draining is set while the supervisor shuts down: a run it interrupts
	// then is stopped and queued for resumption, spending no attempt.
	draining bool
	// lastReported is the idle/waiting line last printed, so announce only
	// repeats it on a state change (zero value: busy, i.e. never reported).
	lastReported tickState
}

// New returns a supervisor. Zero config values take their defaults.
func New(config Config, deps Dependencies) *Supervisor {
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = DefaultMaxConcurrent
	}
	if config.Provider == "" {
		config.Provider = DefaultProvider
	}
	if deps.Now == nil {
		deps.Now = func() time.Time { return time.Now().UTC() }
	}
	if config.RetryMax < config.RetryBase {
		config.RetryMax = config.RetryBase
	}
	if deps.Output == nil {
		deps.Output = io.Discard
	}
	ledger := OpenLedger(config.Layout.LedgerPath())
	ledger.now = deps.Now
	return &Supervisor{config: config, deps: deps, ledger: ledger, processes: map[string]Process{},
		ended: make(chan struct{}, 1)}
}

// Tick settles every run that has ended, moves finished runs along the
// finish line, and starts what the free slots allow.
func (s *Supervisor) Tick() error {
	if err := s.config.Layout.Prepare(); err != nil {
		return err
	}
	if err := s.settleEnded(); err != nil {
		return err
	}
	if err := s.advanceFinishLine(); err != nil {
		return err
	}
	committed, err := ReadIntake(s.deps.Repository, s.config.IntakeBranch, s.config.Plans)
	if err != nil {
		return err
	}
	if err := s.startWork(committed); err != nil {
		return err
	}
	return s.reportTickState()
}

// Run ticks every interval, and as soon as a run it started ends, until ctx
// is done. It then interrupts the runs it started, waits for each to
// checkpoint, and records them as rescheduled.
func (s *Supervisor) Run(ctx context.Context, interval time.Duration) error {
	for {
		if err := s.Tick(); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return s.Drain(true)
		case <-time.After(interval):
		case <-s.ended:
		}
	}
}

// Once does a single pass for a scheduler such as cron: one tick, then a
// wait for the runs it started to halt, then their finish-line steps. It
// claims nothing more after the tick, so one invocation is bounded.
func (s *Supervisor) Once() error {
	if err := s.Tick(); err != nil {
		return err
	}
	if err := s.Drain(false); err != nil {
		return err
	}
	return s.advanceFinishLine()
}

// Drain waits for every run this supervisor started and settles each. With
// interrupt, it first asks each to checkpoint.
func (s *Supervisor) Drain(interrupt bool) error {
	s.draining = interrupt
	defer func() { s.draining = false }()
	for id, process := range s.processes {
		if interrupt {
			if err := process.Interrupt(); err != nil {
				s.say("could not interrupt %s: %v", id, err)
			}
		}
		<-process.Done()
	}
	return s.settleEnded()
}

// watch signals ended once the process exits, so Run wakes at once
// rather than at the next interval.
func (s *Supervisor) watch(process Process) {
	go func() {
		<-process.Done()
		select {
		case s.ended <- struct{}{}:
		default:
		}
	}()
}

// Orders is the ledger folded into each order's state.
func (s *Supervisor) Orders() (map[string]*OrderState, error) {
	entries, err := s.ledger.Entries()
	if err != nil {
		return nil, err
	}
	return Replay(entries), nil
}

func (s *Supervisor) record(entry Entry) error {
	if err := s.ledger.Append(entry); err != nil {
		return err
	}
	s.say("%s %s%s", entry.Order, entry.Event, detail(entry))
	return nil
}

func detail(entry Entry) string {
	switch {
	case entry.Gate != "":
		return " at " + entry.Gate
	case entry.Reason != "":
		return ": " + entry.Reason
	}
	return ""
}

func (s *Supervisor) say(format string, args ...any) {
	fmt.Fprintf(s.deps.Output, format+"\n", args...)
}
