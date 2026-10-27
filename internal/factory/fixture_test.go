package factory

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/worktree"
)

// orderSpec is one test order's frontmatter knobs.
type orderSpec struct {
	id, status, created, finish string
	priority, attempts          int
	dependsOn                   []string
	scope                       []string
}

func (o orderSpec) text() string {
	o = o.withDefaults()
	var extra strings.Builder
	if len(o.dependsOn) > 0 {
		fmt.Fprintf(&extra, "depends-on: [%s]\n", strings.Join(o.dependsOn, ", "))
	}
	if len(o.scope) > 0 {
		fmt.Fprintf(&extra, "scope:\n  paths: [%s]\n", strings.Join(o.scope, ", "))
	}
	return fmt.Sprintf(`---
intake: 1
id: %s
type: feature
plan: deliver-feature
status: %s
priority: %d
created: %s
requested-by: oscar
source: manual
trust: internal
base-branch: main
finish: %s
autonomy: inherit
%sbudget:
  max-attempts: %d
  max-usd: 10
  max-hours: 4
---
# Deliver %s
`, o.id, o.status, o.priority, o.created, o.finish, extra.String(), o.attempts, o.id)
}

func (o orderSpec) withDefaults() orderSpec {
	if o.status == "" {
		o.status = "ready"
	}
	if o.created == "" {
		o.created = "2026-10-24"
	}
	if o.finish == "" {
		o.finish = "none"
	}
	if o.priority == 0 {
		o.priority = 2
	}
	if o.attempts == 0 {
		o.attempts = 2
	}
	return o
}

// testFactory is a real git repository, a scripted launcher, and a
// supervisor wired to both.
type testFactory struct {
	t          *testing.T
	root       string
	launcher   *fakeLauncher
	pulls      *fakePulls
	supervisor *Supervisor
	now        time.Time
	output     *bytes.Buffer
}

func newTestFactory(t *testing.T, orders ...orderSpec) *testFactory {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "test")
	writeTestFile(t, filepath.Join(root, "README.md"), "fixture\n")
	for _, order := range orders {
		writeTestFile(t, filepath.Join(root, IntakeDir, order.id+".md"), order.text())
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-qm", "orders")
	factory := &testFactory{t: t, root: root, launcher: newFakeLauncher(t), pulls: &fakePulls{},
		now: time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC), output: &bytes.Buffer{}}
	factory.supervisor = factory.newSupervisor()
	return factory
}

// newSupervisor is a fresh supervisor over the same repository and ledger —
// what a restart after a crash looks like.
func (f *testFactory) newSupervisor() *Supervisor {
	return New(Config{
		Layout: Layout{Root: f.root}, IntakeBranch: "main", MaxConcurrent: 2, Provider: "mock",
		Plans: []string{"deliver-feature"},
	}, Dependencies{
		Repository: worktree.New(f.root), Launcher: f.launcher, Pulls: f.pulls,
		Trees:  func(dir string) Tree { return worktree.New(dir) },
		Now:    func() time.Time { return f.now },
		Output: f.output,
	})
}

// outputLines is every non-empty line this factory's supervisor has printed
// so far. Call f.output.Reset() first to isolate one tick's output.
func (f *testFactory) outputLines() []string {
	var lines []string
	for _, line := range strings.Split(f.output.String(), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// outputCount counts the printed lines containing text.
func (f *testFactory) outputCount(text string) int {
	count := 0
	for _, line := range f.outputLines() {
		if strings.Contains(line, text) {
			count++
		}
	}
	return count
}

func (f *testFactory) tick() {
	f.t.Helper()
	if err := f.supervisor.Tick(); err != nil {
		f.t.Fatalf("tick: %v", err)
	}
}

// commitOrder writes and commits an order, as a human promoting it would.
func (f *testFactory) commitOrder(order orderSpec) {
	f.t.Helper()
	writeTestFile(f.t, filepath.Join(f.root, IntakeDir, order.id+".md"), order.text())
	runGit(f.t, f.root, "add", "-A")
	runGit(f.t, f.root, "commit", "-qm", "update "+order.id)
}

func (f *testFactory) orders() map[string]*OrderState {
	f.t.Helper()
	orders, err := f.supervisor.Orders()
	if err != nil {
		f.t.Fatalf("orders: %v", err)
	}
	return orders
}

func (f *testFactory) phase(id string) Phase {
	f.t.Helper()
	order, ok := f.orders()[id]
	if !ok {
		return ""
	}
	return order.Phase
}

func (f *testFactory) assertPhase(id string, want Phase) {
	f.t.Helper()
	if got := f.phase(id); got != want {
		f.t.Errorf("order %s phase = %q, want %q", id, got, want)
	}
}

func (f *testFactory) inbox() []InboxItem {
	f.t.Helper()
	committed, err := ReadIntake(worktree.New(f.root), "main", []string{"deliver-feature"})
	if err != nil {
		f.t.Fatalf("read intake: %v", err)
	}
	items, err := Inbox(f.orders(), committed, func(dir string) Tree { return worktree.New(dir) })
	if err != nil {
		f.t.Fatalf("inbox: %v", err)
	}
	return items
}

func (f *testFactory) approve(id, gate string) {
	f.t.Helper()
	if _, err := Approve(f.supervisor.ledger, f.inbox(), id, gate, "oscar"); err != nil {
		f.t.Fatalf("approve %s %s: %v", id, gate, err)
	}
}

// --- the scripted launcher ---

// run is what one scripted `loom run` invocation does: write this state and
// exit with this code. A held run stays running until released.
type run struct {
	code  int
	state func(id string) *orchestrator.RunState
	hold  bool
}

type fakeLauncher struct {
	t        *testing.T
	mu       sync.Mutex
	scripts  map[string][]run
	launches []LaunchRequest
	running  map[string]*fakeProcess
	nextPID  int
}

func newFakeLauncher(t *testing.T) *fakeLauncher {
	return &fakeLauncher{t: t, scripts: map[string][]run{}, running: map[string]*fakeProcess{}, nextPID: 1<<22 + 1000}
}

// script queues what the next launches of an order do. Unscripted launches
// complete every stage.
func (l *fakeLauncher) script(id string, runs ...run) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.scripts[id] = append(l.scripts[id], runs...)
}

func (l *fakeLauncher) Launch(request LaunchRequest) (Process, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.launches = append(l.launches, request)
	id := specID(request.Args)
	script := run{code: exitCompleted, state: completedState}
	if queued := l.scripts[id]; len(queued) > 0 {
		script, l.scripts[id] = queued[0], queued[1:]
	}
	l.nextPID++
	process := &fakeProcess{pid: l.nextPID, done: make(chan struct{}), dir: request.Dir, id: id, script: script, t: l.t}
	l.running[id] = process
	if !script.hold {
		process.finish()
	}
	return process, nil
}

// release lets a held run finish as scripted.
func (l *fakeLauncher) release(id string) {
	l.mu.Lock()
	process := l.running[id]
	l.mu.Unlock()
	process.finish()
}

func (l *fakeLauncher) launchesOf(id string) []LaunchRequest {
	l.mu.Lock()
	defer l.mu.Unlock()
	var found []LaunchRequest
	for _, request := range l.launches {
		if specID(request.Args) == id {
			found = append(found, request)
		}
	}
	return found
}

func specID(args []string) string {
	for index, arg := range args {
		if arg == "--spec" {
			return strings.TrimSuffix(filepath.Base(args[index+1]), ".md")
		}
	}
	return ""
}

type fakeProcess struct {
	t           *testing.T
	pid         int
	done        chan struct{}
	dir, id     string
	script      run
	once        sync.Once
	interrupted bool
}

func (p *fakeProcess) finish() {
	p.once.Do(func() {
		if p.script.state != nil {
			path := RunStatePath(p.dir, p.id)
			if err := orchestrator.NewStateStore(path).Save(p.script.state(p.id)); err != nil {
				p.t.Errorf("write scripted run state: %v", err)
			}
		}
		close(p.done)
	})
}

func (p *fakeProcess) PID() int              { return p.pid }
func (p *fakeProcess) Done() <-chan struct{} { return p.done }
func (p *fakeProcess) ExitCode() int         { return p.script.code }
func (p *fakeProcess) Interrupt() error {
	p.interrupted = true
	p.script = run{code: exitInterrupted, state: interruptedState}
	p.finish()
	return nil
}

type fakePulls struct {
	opened []Proposal
}

func (p *fakePulls) Open(_ string, proposal Proposal) (string, error) {
	p.opened = append(p.opened, proposal)
	return "https://github.com/example/repo/pull/1", nil
}

// --- scripted run states ---

func stateWith(records map[string]orchestrator.StageRecord) *orchestrator.RunState {
	state := orchestrator.NewRunState(orchestrator.DefaultDeliverFeaturePlanName, orchestrator.CreatedByExecutor)
	state.Stages = records
	return state
}

func completedState(string) *orchestrator.RunState {
	return stateWith(map[string]orchestrator.StageRecord{
		"analyst":   {Status: orchestrator.StageStatusCompleted, Sequence: 1, ArtifactSHA256: "a1"},
		"developer": {Status: orchestrator.StageStatusCompleted, Sequence: 2, ArtifactSHA256: "d1"},
	})
}

func waitingState(string) *orchestrator.RunState {
	return stateWith(map[string]orchestrator.StageRecord{
		"analyst":   {Status: orchestrator.StageStatusCompleted, Sequence: 1, ArtifactSHA256: "a1"},
		"developer": {Status: orchestrator.StageStatusWaitingApproval, Sequence: 2, Gate: "confirm-design"},
	})
}

func failedState(class orchestrator.FailureClass) func(string) *orchestrator.RunState {
	return func(string) *orchestrator.RunState {
		return stateWith(map[string]orchestrator.StageRecord{
			"analyst": {Status: orchestrator.StageStatusFailed, Sequence: 1, Error: "boom", FailureClass: class},
		})
	}
}

func interruptedState(string) *orchestrator.RunState {
	return stateWith(map[string]orchestrator.StageRecord{
		"analyst": {Status: orchestrator.StageStatusInterrupted, Sequence: 1, FailureClass: orchestrator.FailureTransient},
	})
}

func budgetState(string) *orchestrator.RunState {
	state := stateWith(map[string]orchestrator.StageRecord{
		"analyst": {Status: orchestrator.StageStatusCompleted, Sequence: 1, ArtifactSHA256: "a1",
			Usage: &orchestrator.Usage{CostUSD: 1}},
	})
	state.Spend = &orchestrator.Usage{CostUSD: 1}
	state.BudgetHalt = &orchestrator.BudgetHalt{Limit: orchestrator.BudgetLimitUSD, Allowed: 1, Consumed: 1, BeforeStage: "developer"}
	return state
}

// --- helpers ---

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasArg(args []string, want ...string) bool {
	joined := " " + strings.Join(args, " ") + " "
	return strings.Contains(joined, " "+strings.Join(want, " ")+" ")
}
