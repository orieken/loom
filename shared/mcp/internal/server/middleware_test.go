package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
)

// step is one scripted run of a tool.
type step func(ctx context.Context) (*domain.ToolResult, error)

func succeeds(context.Context) (*domain.ToolResult, error) { return domain.NewTextResult("ok"), nil }

func fails(kind domain.ErrorKind) step {
	return func(context.Context) (*domain.ToolResult, error) {
		return domain.NewToolErrorResult(domain.NewToolError(kind, "scripted "+string(kind))), nil
	}
}

func transportFails(context.Context) (*domain.ToolResult, error) {
	return nil, errors.New("pipe closed")
}

// scriptedTool runs its steps in order, repeating the last, and counts runs.
type scriptedTool struct {
	mutex sync.Mutex
	steps []step
	runs  int
}

func (s *scriptedTool) Name() string                  { return "scripted" }
func (s *scriptedTool) Description() string           { return "runs a script" }
func (s *scriptedTool) InputSchema() json.RawMessage  { return json.RawMessage(`{"type":"object"}`) }
func (s *scriptedTool) OutputSchema() json.RawMessage { return nil }
func (s *scriptedTool) Execute(ctx context.Context, _ domain.ToolRequest) (*domain.ToolResult, error) {
	s.mutex.Lock()
	next := s.steps[min(s.runs, len(s.steps)-1)]
	s.runs++
	s.mutex.Unlock()
	return next(ctx)
}

func (s *scriptedTool) runCount() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.runs
}

// fastPolicy keeps the production thresholds and shrinks the waits.
var fastPolicy = resiliencePolicy{tripAfter: 5, openFor: time.Hour, maxAttempts: 3, initialDelay: time.Millisecond}

func resilientScript(policy resiliencePolicy, retry domain.RetryClass, steps ...step) (*resilientTool, *scriptedTool) {
	tool := &scriptedTool{steps: steps}
	registration := domain.ToolRegistration{Tool: tool, Retry: retry}
	return newResilientTool(registration, policy, logging.NewLogger(&bytes.Buffer{})), tool
}

func callTimes(r *resilientTool, times int) callOutcome {
	var last callOutcome
	for range times {
		last = r.call(context.Background(), domain.ToolRequest{})
	}
	return last
}

// The L2.6 done-when: five failures in a row open the breaker, and the next
// call returns at once, without running the tool.
func TestFiveConsecutiveFailuresOpenTheBreakerAndTheNextCallReturnsImmediately(t *testing.T) {
	resilient, tool := resilientScript(fastPolicy, domain.RetryIdempotent, fails(domain.ErrorInternal))
	callTimes(resilient, 5)
	if tool.runCount() != 5 {
		t.Fatalf("the tool ran %d times for 5 calls", tool.runCount())
	}

	started := time.Now()
	outcome := resilient.call(context.Background(), domain.ToolRequest{})
	if elapsed := time.Since(started); elapsed > 50*time.Millisecond {
		t.Errorf("an open breaker took %v to answer", elapsed)
	}
	if tool.runCount() != 5 {
		t.Errorf("the open breaker ran the tool (%d runs)", tool.runCount())
	}
	assertBreakerOpenResult(t, outcome)
}

// assertBreakerOpenResult checks an open breaker's answer: a retryable
// transient failure that names the breaker.
func assertBreakerOpenResult(t *testing.T, outcome callOutcome) {
	t.Helper()
	failure := outcome.result.Error
	if failure == nil || failure.Kind != domain.ErrorTransient || !failure.Retryable || !strings.Contains(failure.Message, "circuit breaker opened after 5") {
		t.Errorf("open-breaker result = %+v, want a retryable transient failure naming the breaker", failure)
	}
	if outcome.breaker.After != "open" {
		t.Errorf("breaker state = %q, want open", outcome.breaker.After)
	}
}

func TestFourFailuresDoNotOpenTheBreaker(t *testing.T) {
	resilient, tool := resilientScript(fastPolicy, domain.RetryIdempotent, fails(domain.ErrorInternal))
	callTimes(resilient, 4)
	if outcome := callTimes(resilient, 1); outcome.breaker.After != "open" || tool.runCount() != 5 {
		t.Errorf("after 5 calls: state %q, runs %d — the fifth call must run and open it", outcome.breaker.After, tool.runCount())
	}
}

// A caller's mistake is not the tool's health: one confused model must not
// switch a tool off for everyone.
func TestCallerMistakesAndCancellationsNeverOpenTheBreaker(t *testing.T) {
	for _, kind := range []domain.ErrorKind{domain.ErrorValidation, domain.ErrorNotFound, domain.ErrorPermission, domain.ErrorCancelled} {
		t.Run(string(kind), func(t *testing.T) {
			resilient, tool := resilientScript(fastPolicy, domain.RetryIdempotent, fails(kind))
			outcome := callTimes(resilient, 10)
			if tool.runCount() != 10 || outcome.breaker.After != "closed" {
				t.Errorf("runs %d, state %q: %s failures counted against the tool", tool.runCount(), outcome.breaker.After, kind)
			}
			if outcome.result.Error == nil || outcome.result.Error.Kind != kind {
				t.Errorf("the tool's own failure was replaced: %+v", outcome.result.Error)
			}
		})
	}
}

func TestASuccessResetsTheConsecutiveCount(t *testing.T) {
	resilient, _ := resilientScript(fastPolicy, domain.RetryIdempotent,
		fails(domain.ErrorInternal), fails(domain.ErrorInternal), fails(domain.ErrorInternal), fails(domain.ErrorInternal),
		succeeds, fails(domain.ErrorInternal))
	if outcome := callTimes(resilient, 9); outcome.breaker.After != "closed" {
		t.Errorf("state %q after 4 failures, a success, 4 failures — the success must reset the count", outcome.breaker.After)
	}
}

// An open breaker lets one trial call through after openFor: a success closes
// it, a failure opens it again.
func TestAnOpenBreakerRecoversThroughOneTrialCall(t *testing.T) {
	policy := fastPolicy
	policy.openFor = 20 * time.Millisecond
	steps := []step{fails(domain.ErrorInternal), fails(domain.ErrorInternal), fails(domain.ErrorInternal),
		fails(domain.ErrorInternal), fails(domain.ErrorInternal), fails(domain.ErrorInternal), succeeds}
	resilient, tool := resilientScript(policy, domain.RetryIdempotent, steps...)
	callTimes(resilient, 5)

	time.Sleep(30 * time.Millisecond)
	if outcome := callTimes(resilient, 1); outcome.breaker.Before != "half-open" || outcome.breaker.After != "open" {
		t.Errorf("failed trial: %+v, want half-open then open", outcome.breaker)
	}
	time.Sleep(30 * time.Millisecond)
	if outcome := callTimes(resilient, 1); outcome.breaker.After != "closed" || outcome.result.IsError {
		t.Errorf("successful trial: %+v, want closed with the tool's result", outcome.breaker)
	}
	if tool.runCount() != 7 {
		t.Errorf("runs = %d, want 5 + two trials", tool.runCount())
	}
}

func TestATransientFailureIsRetriedWithBackoffUntilItSucceeds(t *testing.T) {
	resilient, tool := resilientScript(fastPolicy, domain.RetryIdempotent,
		fails(domain.ErrorTransient), fails(domain.ErrorTransient), succeeds)
	outcome := callTimes(resilient, 1)
	if outcome.result.IsError || outcome.attempts != 3 || tool.runCount() != 3 {
		t.Errorf("attempts %d, runs %d, result %+v — want success on the third attempt", outcome.attempts, tool.runCount(), outcome.result.Error)
	}
}

// Retries are bounded, and one call is one breaker sample however many
// attempts it took.
func TestRetriesStopAtMaxAttemptsAndCountAsOneFailure(t *testing.T) {
	resilient, tool := resilientScript(fastPolicy, domain.RetryIdempotent, fails(domain.ErrorTransient))
	// Bounded, so an unbounded retry fails this assertion instead of hanging
	// the suite until backoff's own 15-minute cap.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	outcome := resilient.call(ctx, domain.ToolRequest{})
	if outcome.attempts != 3 || tool.runCount() != 3 {
		t.Errorf("attempts %d, runs %d, want 3", outcome.attempts, tool.runCount())
	}
	if outcome.result.Error == nil || outcome.result.Error.Kind != domain.ErrorTransient {
		t.Errorf("the last attempt's failure was lost: %+v", outcome.result)
	}
	if failures := resilient.breaker.Counts().ConsecutiveFailures; failures != 1 {
		t.Errorf("breaker counted %d failures for one call", failures)
	}
}

func TestOnlyATransientFailureOfARepeatableToolIsRetried(t *testing.T) {
	cases := []struct {
		name  string
		retry domain.RetryClass
		step  step
	}{
		{"transient, not repeatable", domain.RetryNone, fails(domain.ErrorTransient)},
		{"internal", domain.RetryIdempotent, fails(domain.ErrorInternal)},
		{"validation", domain.RetryIdempotent, fails(domain.ErrorValidation)},
		{"cancelled", domain.RetryIdempotent, fails(domain.ErrorCancelled)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resilient, tool := resilientScript(fastPolicy, tc.retry, tc.step)
			if outcome := callTimes(resilient, 1); outcome.attempts != 1 || tool.runCount() != 1 {
				t.Errorf("attempts %d, runs %d, want 1", outcome.attempts, tool.runCount())
			}
		})
	}
}

// An error Execute returns, rather than reports, goes back to the transport
// as before — unretried, and counted against the tool.
func TestATransportErrorIsReturnedUnretriedAndCounted(t *testing.T) {
	resilient, tool := resilientScript(fastPolicy, domain.RetryIdempotent, transportFails)
	outcome := callTimes(resilient, 1)
	if outcome.err == nil || outcome.err.Error() != "pipe closed" || outcome.result != nil || tool.runCount() != 1 {
		t.Errorf("err %v, result %+v, runs %d", outcome.err, outcome.result, tool.runCount())
	}
	if resilient.breaker.Counts().ConsecutiveFailures != 1 {
		t.Error("a transport error was not counted against the tool")
	}
}

// A caller that gives up while a retry waits stops the retrying at once, and
// the abandoned call is not held against the tool.
func TestACallerThatStopsDuringBackoffEndsTheRetriesUncounted(t *testing.T) {
	// Long enough that only the cancellation can end the wait, and short of
	// backoff's own 15-minute elapsed cap, which would end it first and make
	// this test pass without the cancellation mattering.
	policy := fastPolicy
	policy.initialDelay = 10 * time.Second
	resilient, tool := resilientScript(policy, domain.RetryIdempotent, fails(domain.ErrorTransient))
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(20*time.Millisecond, cancel)

	started := time.Now()
	outcome := resilient.call(ctx, domain.ToolRequest{})
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("the retry kept waiting %v after its caller left", elapsed)
	}
	if tool.runCount() != 1 || outcome.result == nil || outcome.result.Error.Kind != domain.ErrorTransient {
		t.Errorf("runs %d, result %+v — want the one attempt's failure", tool.runCount(), outcome.result)
	}
	if counts := resilient.breaker.Counts(); counts.ConsecutiveFailures != 0 {
		t.Errorf("an abandoned call was counted: %+v", counts)
	}
}

// Each attempt gets the registration's own Timeout, so a retry is not born
// with its predecessor's spent deadline.
func TestEveryAttemptRunsUnderItsOwnTimeout(t *testing.T) {
	var deadlines []time.Time
	var mutex sync.Mutex
	timesOut := func(ctx context.Context) (*domain.ToolResult, error) {
		deadline, _ := ctx.Deadline()
		mutex.Lock()
		deadlines = append(deadlines, deadline)
		mutex.Unlock()
		<-ctx.Done()
		return domain.NewToolErrorResult(domain.NewToolError(domain.ErrorTransient, ctx.Err().Error())), nil
	}
	tool := &scriptedTool{steps: []step{timesOut}}
	registration := domain.ToolRegistration{Tool: tool, Retry: domain.RetryIdempotent, Timeout: 20 * time.Millisecond}
	// Bounded, so a regression that shares one deadline across attempts, or
	// retries without end, fails the count below instead of hanging.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	newResilientTool(registration, fastPolicy, logging.NewLogger(&bytes.Buffer{})).call(ctx, domain.ToolRequest{})

	if len(deadlines) != 3 {
		t.Fatalf("%d attempts, want 3", len(deadlines))
	}
	for i := 1; i < len(deadlines); i++ {
		if !deadlines[i].After(deadlines[i-1].Add(15 * time.Millisecond)) {
			t.Errorf("attempt %d deadline %v is not a fresh budget after %v", i+1, deadlines[i], deadlines[i-1])
		}
	}
}

// Malformed calls are refused before the breaker: they say nothing about the
// tool, so six of them leave it closed and the next valid call runs.
func TestMalformedCallsNeverReachTheBreaker(t *testing.T) {
	tool := &scriptedTool{steps: []step{succeeds}}
	handler := New(logging.NewLogger(&bytes.Buffer{}))
	handler.resilience = fastPolicy
	strict := strictSchemaTool{scriptedTool: tool}
	call := handler.mcpToolHandler(domain.ToolRegistration{Tool: strict, Retry: domain.RetryIdempotent})
	for range 6 {
		request := mcp.CallToolRequest{}
		request.Params.Arguments = map[string]any{"unexpected": true}
		if result, _ := call(context.Background(), request); result == nil || !result.IsError {
			t.Fatal("a malformed call was accepted")
		}
	}
	if result, err := call(context.Background(), mcp.CallToolRequest{}); err != nil || result.IsError || tool.runCount() != 1 {
		t.Errorf("valid call after malformed ones: err %v, result %+v, runs %d", err, result, tool.runCount())
	}
}

// strictSchemaTool is a scriptedTool whose schema accepts no arguments.
type strictSchemaTool struct{ *scriptedTool }

func (strictSchemaTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","additionalProperties":false}`)
}

func TestAnUnknownErrorKindIsRecordedAsOther(t *testing.T) {
	invented := domain.NewToolErrorResult(domain.NewToolError("quota_exceeded", "x"))
	if got := toolResult(callOutcome{result: invented}).ErrorKind; got != "other" {
		t.Errorf("kind = %q, want other", got)
	}
	known := domain.NewToolErrorResult(domain.NewToolError(domain.ErrorNotFound, "x"))
	if got := toolResult(callOutcome{result: known}).ErrorKind; got != "not_found" {
		t.Errorf("kind = %q, want not_found", got)
	}
	if got := toolResult(callOutcome{result: domain.NewTextResult("ok")}).ErrorKind; got != "" {
		t.Errorf("a success recorded kind %q", got)
	}
}

// The state changes are logged as they happen, whichever call causes them.
func TestBreakerStateChangesAreLogged(t *testing.T) {
	var logs bytes.Buffer
	tool := &scriptedTool{steps: []step{fails(domain.ErrorInternal)}}
	resilient := newResilientTool(domain.ToolRegistration{Tool: tool}, fastPolicy, logging.NewLogger(&logs))
	callTimes(resilient, 5)
	if !strings.Contains(logs.String(), "tool.breaker.state_changed") || !strings.Contains(logs.String(), `"to":"open"`) {
		t.Errorf("no state change logged:\n%s", logs.String())
	}
}
