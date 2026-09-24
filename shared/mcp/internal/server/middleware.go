package server

// Resilience for every tool call (roadmap L2.6). Guardrail #5 forbids a
// hand-rolled retry loop and requires a circuit breaker or exponential
// backoff; until this file neither existed, and the only retry in the
// framework was prose telling an LLM to count to three.
//
// Each registration gets its own breaker, and retries run inside it, so one
// logical call — however many attempts it took — is one sample. No tool
// implements either: both are applied here, in the adapter layer, which keeps
// the third-party libraries out of internal/domain (M0.3).

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/sony/gobreaker/v2"

	"github.com/orieken/loom/internal/telemetry"
	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
)

// resiliencePolicy is how hard a tool is retried and how soon it is cut off.
type resiliencePolicy struct {
	// tripAfter consecutive unhealthy calls open the breaker.
	tripAfter uint32
	// openFor is how long an open breaker refuses calls before letting one
	// trial call through.
	openFor time.Duration
	// maxAttempts bounds one call, first attempt included.
	maxAttempts uint
	// initialDelay is the first backoff; each later one roughly doubles.
	initialDelay time.Duration
}

// defaultResiliencePolicy: five failures in a row is the roadmap's threshold.
// Three attempts, each under the registration's own Timeout, bounds a walk
// that keeps timing out to about three budgets — the caller's context can
// stop it sooner.
var defaultResiliencePolicy = resiliencePolicy{
	tripAfter:    5,
	openFor:      30 * time.Second,
	maxAttempts:  3,
	initialDelay: 200 * time.Millisecond,
}

// errUnhealthy tells the breaker a call failed in a way that says the tool
// itself is in trouble: a transient or internal failure. A caller's mistake —
// bad arguments, a missing path, a refused one — is not the tool's health,
// and counting it would let one confused model switch a tool off for all.
var errUnhealthy = errors.New("tool reported a transient or internal failure")

// errCallerStopped is a call its caller cancelled: neither a success nor a
// failure of the tool, so the breaker does not count it.
var errCallerStopped = errors.New("caller cancelled the call")

// transportError is an error Execute returned rather than reported: it goes
// back to the transport as it did before, unretried.
type transportError struct{ err error }

func (e *transportError) Error() string { return e.err.Error() }
func (e *transportError) Unwrap() error { return e.err }

// resilientTool runs one registration's calls through its breaker and retry.
type resilientTool struct {
	registration domain.ToolRegistration
	policy       resiliencePolicy
	breaker      *gobreaker.CircuitBreaker[*domain.ToolResult]
}

func newResilientTool(registration domain.ToolRegistration, policy resiliencePolicy, logger *logging.Logger) *resilientTool {
	return &resilientTool{
		registration: registration,
		policy:       policy,
		breaker: gobreaker.NewCircuitBreaker[*domain.ToolResult](gobreaker.Settings{
			Name:        registration.Tool.Name(),
			MaxRequests: 1,
			Timeout:     policy.openFor,
			ReadyToTrip: func(counts gobreaker.Counts) bool { return counts.ConsecutiveFailures >= policy.tripAfter },
			IsExcluded:  isCallerStop,
			OnStateChange: func(name string, from, to gobreaker.State) {
				logger.Warn("tool.breaker.state_changed", "tool", name, "from", from.String(), "to", to.String())
			},
		}),
	}
}

// callOutcome is one call's result and what it took to get it.
type callOutcome struct {
	result   *domain.ToolResult
	err      error
	attempts int
	breaker  telemetry.BreakerStates
}

// call runs request through the breaker. An open breaker returns at once,
// without running the tool.
func (r *resilientTool) call(ctx context.Context, request domain.ToolRequest) callOutcome {
	outcome := callOutcome{breaker: telemetry.BreakerStates{Before: r.breaker.State().String()}}
	result, err := r.breaker.Execute(func() (*domain.ToolResult, error) {
		return r.retrying(ctx, request, &outcome.attempts)
	})
	outcome.breaker.After = r.breaker.State().String()
	outcome.result, outcome.err = r.settle(result, err)
	return outcome
}

// retrying runs the tool until it succeeds, fails in a way retrying cannot
// fix, or runs out of attempts, backing off exponentially between attempts.
func (r *resilientTool) retrying(ctx context.Context, request domain.ToolRequest, attempts *int) (*domain.ToolResult, error) {
	schedule := backoff.NewExponentialBackOff()
	schedule.InitialInterval = r.policy.initialDelay
	return backoff.Retry(ctx, func() (*domain.ToolResult, error) {
		*attempts++
		result, err := r.attempt(ctx, request)
		return result, r.verdict(result, err)
	}, backoff.WithBackOff(schedule), backoff.WithMaxTries(r.policy.maxAttempts))
}

// attempt is one run of the tool under the registration's Timeout (L2.2).
func (r *resilientTool) attempt(ctx context.Context, request domain.ToolRequest) (*domain.ToolResult, error) {
	ctx, cancel := withDeadline(ctx, r.registration.Timeout)
	defer cancel()
	return r.registration.Tool.Execute(ctx, request)
}

// verdict turns an attempt into what the retry and the breaker act on: nil
// for a call that says nothing against the tool, errUnhealthy for one that
// does — retryable only when transient and the tool is safe to repeat.
func (r *resilientTool) verdict(result *domain.ToolResult, err error) error {
	if err != nil {
		return backoff.Permanent(&transportError{err: err})
	}
	switch failureKind(result) {
	case domain.ErrorTransient:
		return r.transientVerdict()
	case domain.ErrorInternal:
		return backoff.Permanent(errUnhealthy)
	case domain.ErrorCancelled:
		return backoff.Permanent(errCallerStopped)
	}
	return nil
}

func (r *resilientTool) transientVerdict() error {
	if r.registration.Retry == domain.RetryIdempotent {
		return errUnhealthy
	}
	return backoff.Permanent(errUnhealthy)
}

// failureKind is a result's failure kind, or "" when it succeeded.
func failureKind(result *domain.ToolResult) domain.ErrorKind {
	if result == nil || !result.IsError || result.Error == nil {
		return ""
	}
	return result.Error.Kind
}

// settle turns the breaker's answer back into what the transport returns.
func (r *resilientTool) settle(result *domain.ToolResult, err error) (*domain.ToolResult, error) {
	var transport *transportError
	switch {
	case errors.As(err, &transport):
		return nil, transport.err
	case errors.Is(err, gobreaker.ErrOpenState), errors.Is(err, gobreaker.ErrTooManyRequests):
		return r.breakerOpenResult(), nil
	case result == nil:
		return nil, err
	}
	return result, nil
}

// breakerOpenResult is a transient failure: the tool may recover, and the
// message says when the next call will be let through to find out.
func (r *resilientTool) breakerOpenResult() *domain.ToolResult {
	message := fmt.Sprintf("%s is unavailable: its circuit breaker opened after %d consecutive failures; retry after %s",
		r.registration.Tool.Name(), r.policy.tripAfter, r.policy.openFor)
	return domain.NewToolErrorResult(domain.NewToolError(domain.ErrorTransient, message))
}

// isCallerStop keeps a call its caller stopped out of the breaker's counts,
// whether the tool reported the cancellation or the retry's wait saw it.
func isCallerStop(err error) bool {
	return errors.Is(err, errCallerStopped) || errors.Is(err, context.Canceled)
}
