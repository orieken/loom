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
	// DefaultUnavailableWait is how long an order waits out a provider that
	// cannot take calls — a usage or session limit — before trying again.
	DefaultUnavailableWait = 30 * time.Minute
	DefaultProvider        = "claude"
	// DefaultRemote is the remote a pass fetches before reading intake/,
	// when Config.Remote names none of its own.
	DefaultRemote = "origin"
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
	// UnavailableWait is the wait after a provider reported it cannot take
	// calls. It spends none of the order's attempts.
	UnavailableWait time.Duration
	// Remote is fetched once at the start of every pass, before intake/ is
	// read (`loom factory run`'s fetch-before-read). Empty defaults to
	// DefaultRemote.
	Remote string
	// NoFetch reads IntakeBranch from the local checkout only, with no
	// fetch — the behavior every pass had before fetch-before-read.
	NoFetch bool
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

// withDefaults fills in config's zero values with their defaults.
func (config Config) withDefaults() Config {
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = DefaultMaxConcurrent
	}
	if config.Provider == "" {
		config.Provider = DefaultProvider
	}
	if config.Remote == "" {
		config.Remote = DefaultRemote
	}
	if config.UnavailableWait <= 0 {
		config.UnavailableWait = DefaultUnavailableWait
	}
	if config.RetryMax < config.RetryBase {
		config.RetryMax = config.RetryBase
	}
	return config
}

// New returns a supervisor. Zero config values take their defaults.
func New(config Config, deps Dependencies) *Supervisor {
	config = config.withDefaults()
	if deps.Now == nil {
		deps.Now = func() time.Time { return time.Now().UTC() }
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
	committed, err := s.readIntake()
	if err != nil {
		return err
	}
	if err := s.startWork(committed); err != nil {
		return err
	}
	return s.reportTickState()
}

// readIntake fetches the ref this pass reads intake/ at, then reads the
// committed orders there. It names the fetch-then-read sequence Tick needs
// as one step.
func (s *Supervisor) readIntake() (Intake, error) {
	ref, err := s.fetchIntakeRef()
	if err != nil {
		return Intake{}, err
	}
	return ReadIntake(s.deps.Repository, ref, s.config.Plans)
}

// fetchIntakeRef decides which ref this pass reads intake/ at. With
// --no-fetch, or no remote configured by Config.Remote's name, it is the
// bare local branch, exactly as every pass read before fetch-before-read —
// no fetch is attempted and nothing is printed. Otherwise it fetches the
// remote once and returns its remote-tracking ref. A fetch that fails (no
// network, bad credentials) is logged on one line; the pass goes on from
// whatever that ref last resolved to.
func (s *Supervisor) fetchIntakeRef() (string, error) {
	if s.config.NoFetch {
		return s.config.IntakeBranch, nil
	}
	remotes, err := s.deps.Repository.Remotes()
	if err != nil {
		return "", err
	}
	if !hasRemote(remotes, s.config.Remote) {
		return s.config.IntakeBranch, nil
	}
	if err := s.deps.Repository.Fetch(s.config.Remote, s.config.IntakeBranch); err != nil {
		s.say("fetch %s %s failed, continuing from the last fetch: %v", s.config.Remote, s.config.IntakeBranch, err)
	}
	ref := s.config.Remote + "/" + s.config.IntakeBranch
	if _, err := s.deps.Repository.ResolveCommit(ref); err != nil {
		return s.config.IntakeBranch, nil
	}
	return ref, nil
}

func hasRemote(remotes []string, name string) bool {
	for _, remote := range remotes {
		if remote == name {
			return true
		}
	}
	return false
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
