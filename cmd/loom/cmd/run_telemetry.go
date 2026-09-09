package cmd

// Telemetry wiring for `loom run` (roadmap L3.8).
//
// The default is asymmetric on purpose: every run writes its own trace file
// into its workspace, and nothing is sent over the network unless
// OTEL_EXPORTER_OTLP_ENDPOINT says where. Local export leaves nothing, so
// the reason to make network export opt-in does not apply to it — and a
// trace that only exists when someone predicted they would want it cannot
// answer "what did this run cost" about a run that already finished.

import (
	"context"
	"strings"
	"time"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/telemetry"
	"github.com/spf13/cobra"
)

// flushTimeout bounds shutdown so a wedged collector cannot hang a run that
// has otherwise finished its work.
const flushTimeout = 5 * time.Second

// reportRunUsage prints what the run cost, from run state rather than from
// the trace — the figure has to be available whether or not anyone
// configured a collector, and it is the same number either way.
func reportRunUsage(cmd *cobra.Command, store *orchestrator.StateStore) {
	state, err := store.Load()
	if err != nil || state == nil {
		return
	}
	total := state.TotalUsage()
	if total == (orchestrator.Usage{}) {
		return
	}
	cmd.Printf("Usage: %d in / %d out tokens (%d cache read, %d cache write) — $%.4f\n",
		total.InputTokens, total.OutputTokens, total.CacheReadTokens, total.CacheCreationTokens, total.CostUSD)
	reportPostureViolations(cmd, state)
}

// reportPostureErrorOnce warns that the tree cannot be fingerprinted, one
// time. The check runs per stage, so an unfingerprintable tree — a project
// with no git repository, most often — would otherwise print the same line
// fifteen times and bury everything else.
func reportPostureErrorOnce(cmd *cobra.Command) func(error) {
	var reported bool
	return func(err error) {
		if reported {
			return
		}
		reported = true
		cmd.PrintErrf("warning: cannot check what stages change (%v) — "+
			"posture checking is off for this run\n", err)
	}
}

// reportPostureViolations names any stage that changed source it never
// declared it would change (roadmap L3.30). Buried in the timeline it would
// go unread, which is how run 4's edits reached the shipped tree without
// anyone noticing they had bypassed review.
func reportPostureViolations(cmd *cobra.Command, state *orchestrator.RunState) {
	violations := state.PostureViolations()
	if len(violations) == 0 {
		return
	}
	cmd.PrintErrf("warning: %s changed the working tree without declaring a tool that writes files — "+
		"review those edits, nothing else did\n", strings.Join(violations, ", "))
}

// startTelemetry opens a tracing session for this run and returns the
// shutdown to defer. A nil session yields a no-op shutdown, so callers do
// not branch on whether telemetry is on.
func startTelemetry(cmd *cobra.Command, executor *orchestrator.Executor, workspaceDir string) (func(), error) {
	session, err := telemetry.Start(telemetry.Options{
		Version:   version,
		TraceFile: traceFileFor(workspaceDir),
	})
	if err != nil {
		return nil, err
	}
	executor.WithTracer(session.Tracer())
	return func() { shutdownTelemetry(cmd, session) }, nil
}

// traceFileFor resolves where OTLP/JSON goes: nowhere when telemetry is
// off, the flag's path when given, the run's own workspace otherwise.
func traceFileFor(workspaceDir string) string {
	if runArgs.noTelemetry {
		return ""
	}
	if runArgs.otelFile != "" {
		return runArgs.otelFile
	}
	return telemetry.TraceFileFor(workspaceDir)
}

// shutdownTelemetry flushes on every exit path, the gate halt included — a
// run that stops to ask a human still recorded the stages it completed
// first. A flush failure is reported and never masks the run's own result.
func shutdownTelemetry(cmd *cobra.Command, session *telemetry.Session) {
	ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()
	if err := session.Shutdown(ctx); err != nil {
		cmd.PrintErrf("telemetry: flush failed: %v\n", err)
	}
}
