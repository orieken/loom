package factory

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/orieken/loom/internal/intake"
	"github.com/orieken/loom/internal/orchestrator"
)

// Event is one kind of ledger line. The ledger is append-only: a later line
// supersedes an earlier one, and nothing is ever rewritten.
type Event string

// What the ledger records.
const (
	// EventClaimed: the supervisor took an order, at a recorded commit.
	EventClaimed Event = "claimed"
	// EventLaunched: a `loom run` process started for the order.
	EventLaunched Event = "launched"
	// EventWaiting: the run halted at a gate and waits on a human.
	EventWaiting Event = "waiting"
	// EventCompleted: every stage of the run settled.
	EventCompleted Event = "completed"
	// EventRetryScheduled: a transient failure will be resumed after a delay.
	EventRetryScheduled Event = "retry-scheduled"
	// EventParked: a terminal state a human clears by editing the order.
	EventParked Event = "parked"
	// EventStopped: `loom factory stop` interrupted the run.
	EventStopped Event = "stopped"
	// EventStopRequested: a human asked for the run to be interrupted.
	EventStopRequested Event = "stop-requested"
	// EventResumeRequested: a human asked for a stopped run to continue.
	EventResumeRequested Event = "resume-requested"
	// EventRerunRequested: a human chose to re-run an order edited since
	// its claim, rather than keep the version that ran (ADR-010, 1C).
	EventRerunRequested Event = "rerun-requested"
	// EventApproved: a human approved an inbox item. For a run gate the
	// approval itself is still recorded by `loom run --resume --approve`;
	// this line is the request, bound to what the human was shown.
	EventApproved Event = "approved"
	// EventApprovalDropped: what an approval was bound to changed before
	// the supervisor could act on it.
	EventApprovalDropped Event = "approval-dropped"
	// EventCommitted: the run's changes were committed on its branch.
	EventCommitted Event = "committed"
	// EventPushed: the branch was pushed.
	EventPushed Event = "pushed"
	// EventProposed: a pull request proposal is waiting in the inbox.
	EventProposed Event = "pr-proposed"
	// EventPROpened: a human-approved pull request was opened.
	EventPROpened Event = "pr-opened"
	// EventFinishFailed: a finish-line step failed and will be retried.
	EventFinishFailed Event = "finish-failed"
	// EventShipped: the order is done; its dependents may start.
	EventShipped Event = "shipped"
)

// Entry is one ledger line. Fields are set per event; most are empty.
type Entry struct {
	At      time.Time `json:"at"`
	Event   Event     `json:"event"`
	Order   string    `json:"order"`
	Attempt int       `json:"attempt,omitempty"`
	Claim   *Claim    `json:"claim,omitempty"`
	// Launch facts.
	PID       int    `json:"pid,omitempty"`
	Approving string `json:"approving,omitempty"`
	// Outcome facts.
	ExitCode  *int       `json:"exitCode,omitempty"`
	Gate      string     `json:"gate,omitempty"`
	Reason    string     `json:"reason,omitempty"`
	NotBefore *time.Time `json:"notBefore,omitempty"`
	Spend     *Spend     `json:"spend,omitempty"`
	// Approval facts.
	Digest     string `json:"digest,omitempty"`
	ApprovedBy string `json:"approvedBy,omitempty"`
	// Finish-line facts.
	Commit   string    `json:"commit,omitempty"`
	Proposal *Proposal `json:"proposal,omitempty"`
	URL      string    `json:"url,omitempty"`
}

// Claim is what the supervisor took, recorded so the run can be relaunched
// and explained without reading the intake file again.
type Claim struct {
	SourceCommit string        `json:"sourceCommit"`
	OrderPath    string        `json:"orderPath"`
	OrderSHA256  string        `json:"orderSha256"`
	Plan         string        `json:"plan"`
	Type         string        `json:"type"`
	Title        string        `json:"title,omitempty"`
	Priority     int           `json:"priority"`
	Created      string        `json:"created"`
	BaseBranch   string        `json:"baseBranch"`
	Finish       string        `json:"finish"`
	Budget       intake.Budget `json:"budget"`
	ScopePaths   []string      `json:"scopePaths,omitempty"`
	Branch       string        `json:"branch"`
	Worktree     string        `json:"worktree"`
}

// Spend is what a run consumed, recorded when a budget parks it (L3.70).
type Spend struct {
	TotalUSD float64                   `json:"totalUsd"`
	PerStage []orchestrator.StageSpend `json:"perStage,omitempty"`
}

// Proposal is a pull request waiting for a human to open it (L3.69).
type Proposal struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Base  string `json:"base"`
	Head  string `json:"head"`
}

// Ledger is the append-only file of entries.
type Ledger struct {
	path string
	now  func() time.Time
}

// OpenLedger returns the ledger at path. The file is created on first append.
func OpenLedger(path string) *Ledger {
	return &Ledger{path: path, now: func() time.Time { return time.Now().UTC() }}
}

// Append writes one entry as one line, stamped with the time, and syncs it.
// One write per line is what keeps a crash from interleaving two lines.
func (l *Ledger) Append(entry Entry) error {
	if entry.At.IsZero() {
		entry.At = l.now()
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode ledger entry: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return fmt.Errorf("create ledger directory: %w", err)
	}
	if err := dropTornLine(l.path); err != nil {
		return err
	}
	return appendLine(l.path, append(line, '\n'))
}

func appendLine(path string, line []byte) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open ledger: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(line); err != nil {
		return fmt.Errorf("append to ledger: %w", err)
	}
	return file.Sync()
}

// ErrCorruptLedger reports a line that does not decode, other than a final
// line a crash cut short.
var ErrCorruptLedger = errors.New("corrupt factory ledger")

// Entries reads every entry in order. A final line without its newline was
// being written when the process died; it is dropped, because the action it
// described had not been recorded as done. Any other bad line is an error.
func (l *Ledger) Entries() ([]Entry, error) {
	raw, err := os.ReadFile(l.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read ledger: %w", err)
	}
	if !bytes.HasSuffix(raw, []byte("\n")) {
		raw = raw[:bytes.LastIndexByte(raw, '\n')+1]
	}
	return decodeEntries(raw, l.path)
}

func decodeEntries(raw []byte, path string) ([]Entry, error) {
	var entries []Entry
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for line := 1; scanner.Scan(); line++ {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, fmt.Errorf("%w: %s line %d: %v", ErrCorruptLedger, path, line, err)
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

// dropTornLine truncates a final line a crash left without its newline, so
// the next entry starts on a line of its own instead of being glued to it.
func dropTornLine(path string) error {
	if complete, err := endsWithNewline(path); complete || err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read ledger: %w", err)
	}
	if err := os.Truncate(path, int64(bytes.LastIndexByte(raw, '\n')+1)); err != nil {
		return fmt.Errorf("drop a torn ledger line: %w", err)
	}
	return nil
}

// endsWithNewline reads only the last byte: the common case, a ledger ending
// cleanly, must not cost a read of the whole file on every append.
func endsWithNewline(path string) (bool, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("open ledger: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() == 0 {
		return err == nil, err
	}
	last := make([]byte, 1)
	if _, err := file.ReadAt(last, info.Size()-1); err != nil {
		return false, fmt.Errorf("read ledger: %w", err)
	}
	return last[0] == '\n', nil
}
