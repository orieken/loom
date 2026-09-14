// Package claude is the real Provider for the executor skeleton (roadmap
// M0.4 part 2): it invokes each stage's agent by spawning the `claude` CLI
// headless (`claude -p --output-format json`) as a subprocess. The prompt is
// built from the agent's shared/agents/<agent>.md definition plus the feature
// spec path; stdout carries a JSON result envelope whose `result` becomes the
// stage's artifact and whose `usage` becomes the stage's cost (roadmap L3.8).
// Timeouts arrive
// via ctx (the executor applies the stage's timeout). exec.CommandContext
// kills the subprocess when ctx ends — but only the process it started, so
// WaitDelay bounds the wait for output pipes an agent's own children may
// still hold open. Without it a stage could outlive its deadline by however
// long a grandchild chose to run. If the
// claude binary is absent the stage FAILS with a remediation message — it
// never silently falls back to the mock provider.
package claude

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/telemetry"
)

// DefaultBinaryName is the CLI the provider spawns.
const DefaultBinaryName = "claude"

// Config wires the provider to its environment. All fields are explicit so
// tests can point BinaryName at a scripted fake and AgentsDir at fixtures.
type Config struct {
	// BinaryName is the claude CLI to spawn; empty means DefaultBinaryName.
	BinaryName string
	// AgentsDir holds the agent definitions (shared/agents or a .claude/agents symlink).
	AgentsDir string
	// Logger receives structured logs; nil means JSON to os.Stderr. Logs
	// never go to stdout (that belongs to the user).
	Logger *slog.Logger
}

// Provider spawns the claude CLI once per stage.
type Provider struct {
	binaryName string
	agentsDir  string
	logger     *slog.Logger
}

// New builds a Provider from config, defaulting the binary name and logger.
func New(config Config) *Provider {
	binaryName := config.BinaryName
	if binaryName == "" {
		binaryName = DefaultBinaryName
	}
	logger := config.Logger
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stderr, nil))
	}
	return &Provider{binaryName: binaryName, agentsDir: config.AgentsDir, logger: logger}
}

// Invoke runs one stage's agent headless and writes its output to the
// stage's artifact path inside the workspace.
func (p *Provider) Invoke(ctx context.Context, stage orchestrator.Stage, input orchestrator.StageInput) (orchestrator.StageOutput, error) {
	binaryPath, err := exec.LookPath(p.binaryName)
	if err != nil {
		return orchestrator.StageOutput{}, fmt.Errorf(
			"claude binary %q not found on PATH — install it (npm install -g @anthropic-ai/claude-code) or run with --provider mock for a dry run: %w",
			p.binaryName, err)
	}
	prompt, allowed, err := p.buildPrompt(stage, input)
	if err != nil {
		return orchestrator.StageOutput{}, err
	}
	output, err := p.runSubprocess(ctx, binaryPath, prompt, allowed, stage, input)
	output.DeclaredWriteAccess = writesFiles(allowed)
	return output, err
}

// buildPrompt composes the stage prompt: the full agent definition, then the
// delivery task naming the spec and workspace so the agent reads them itself.
func (p *Provider) buildPrompt(stage orchestrator.Stage, input orchestrator.StageInput) (string, []string, error) {
	definitionPath := filepath.Join(p.agentsDir, stage.Agent+".md")
	definition, err := os.ReadFile(definitionPath)
	if err != nil {
		return "", nil, fmt.Errorf("agent definition for stage %q not found at %s: %w", stage.ID, definitionPath, err)
	}
	allowed := allowedToolsFor(definition)
	prompt := fmt.Sprintf(
		"%s\n\n---\n\nAct exactly as the agent defined above.\nFeature spec: %s\nWorkspace directory (read prior stage artifacts here): %s\nProduce your complete markdown artifact on stdout and nothing else.\n",
		definition, input.SpecPath, input.WorkspaceDir)
	if stage.StateKind == "" {
		return prompt, allowed, nil
	}
	instruction, err := typedInstruction(stage, input, allowed)
	if err != nil {
		return "", nil, err
	}
	return prompt + instruction, allowed, nil
}

// subprocessWaitDelay bounds how long Wait will keep waiting for output
// after the process is gone or its context has ended. Long enough for a
// well-behaved child to flush its last write, short enough that a deadline
// stays a deadline.
const subprocessWaitDelay = 2 * time.Second

// runSubprocess executes `claude -p --output-format json <prompt>` and
// parses the result envelope. exec.CommandContext kills the subprocess when
// ctx ends — that is how stage timeouts and SIGINT reach it, and it reaches
// only the direct child.
//
// stdout is captured in memory for every stage now, not only typed ones.
// The envelope has to be parsed before the agent's own output can be
// separated from the accounting around it, so there is nothing left to
// stream straight to a file.
func (p *Provider) runSubprocess(ctx context.Context, binaryPath, prompt string, allowed []string, stage orchestrator.Stage, input orchestrator.StageInput) (orchestrator.StageOutput, error) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	// The prompt travels over stdin, not argv: agent definitions begin with
	// `---` YAML frontmatter, which the claude CLI's argument parser would
	// treat as an option — and stdin also sidesteps ARG_MAX for large
	// definitions. `claude -p` reads the prompt from stdin when piped.
	// The stage's declared tools travel as an explicit allowlist. Without
	// them `claude -p` denies every Write and Edit and the run fails
	// silently (roadmap L2.22).
	args := append([]string{"-p", "--output-format", "json"}, permissionArgs(allowed)...)
	command := exec.CommandContext(ctx, binaryPath, args...)
	command.Stdin = strings.NewReader(prompt)
	command.Stdout = stdout
	command.Stderr = stderr
	// TRACEPARENT rides the environment so a tool call the agent makes can
	// land under this stage (roadmap L3.8). Env is inherited through exec,
	// which is the only channel available: MCP carries no trace context and
	// loom does not spawn the MCP server — the claude CLI does. Best-effort
	// by construction, and nothing downstream depends on it arriving.
	command.Env = append(os.Environ(), telemetry.TraceParentEnv(ctx)...)
	// Stdout and Stderr are buffers, so exec gives the child a pipe and Wait
	// blocks until every writer closes it. Killing the child does not close
	// what its own children inherited: `claude -p` spawns MCP servers and
	// tools, and on a shell whose /bin/sh forks rather than execs (dash on
	// Debian and Ubuntu) even a one-line wrapper leaves a grandchild holding
	// the pipe. Without this the stage returns when that grandchild exits
	// rather than when its deadline passes, which is the timeout guarantee
	// the executor is built on.
	command.WaitDelay = subprocessWaitDelay

	started := time.Now()
	p.logger.Info("stage.started", "stage", stage.ID, "agent", stage.Agent, "binary", binaryPath,
		"allowedTools", strings.Join(allowed, ","))
	runErr := command.Run()
	p.logger.Info("stage.finished", "stage", stage.ID, "durationMs", time.Since(started).Milliseconds(), "success", runErr == nil)
	return p.finish(ctx, runErr, stage, input, stdout, stderr)
}

func (p *Provider) finish(ctx context.Context, waitErr error, stage orchestrator.Stage, input orchestrator.StageInput, stdout, stderr *bytes.Buffer) (orchestrator.StageOutput, error) {
	if ctx.Err() != nil {
		return orchestrator.StageOutput{}, fmt.Errorf("stage %q subprocess terminated: %w", stage.ID, ctx.Err())
	}
	// ErrWaitDelay means the process itself finished but something it spawned
	// still held the output pipe open. The envelope is already captured, so a
	// lingering grandchild is not this stage's failure — and if the output was
	// genuinely cut short, parseEnvelope below is what says so.
	if errors.Is(waitErr, exec.ErrWaitDelay) {
		waitErr = nil
	}
	if waitErr != nil {
		return orchestrator.StageOutput{}, fmt.Errorf("stage %q agent exited with error: %w — stderr: %s",
			stage.ID, waitErr, truncate(stderr.String(), 2000))
	}
	result, err := parseEnvelope(stdout.Bytes())
	if err != nil {
		return orchestrator.StageOutput{}, fmt.Errorf("stage %q: %w", stage.ID, err)
	}
	p.warnIfUnreported(stage, result)
	return p.outputFor(stage, input, result)
}

// outputFor turns the envelope's result text into the stage's artifact: a
// validated JSON payload for a typed stage, a markdown file for the rest.
// Usage rides along either way — it is a property of the invocation, not of
// what the invocation happened to produce.
func (p *Provider) outputFor(stage orchestrator.Stage, input orchestrator.StageInput, result *envelope) (orchestrator.StageOutput, error) {
	if stage.StateKind != "" {
		payload, err := extractJSON([]byte(result.Result))
		if err != nil {
			return orchestrator.StageOutput{Usage: result.usage()}, fmt.Errorf("stage %q: %w", stage.ID, err)
		}
		return orchestrator.StageOutput{Payload: payload, Usage: result.usage()}, nil
	}
	artifactPath := filepath.Join(input.WorkspaceDir, stage.ID+".md")
	if err := os.WriteFile(artifactPath, []byte(result.Result), 0o644); err != nil {
		return orchestrator.StageOutput{Usage: result.usage()}, fmt.Errorf("stage %q: write artifact: %w", stage.ID, err)
	}
	return orchestrator.StageOutput{ArtifactPath: artifactPath, Usage: result.usage()}, nil
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "…(truncated)"
}

// warnIfUnreported flags a successful invocation that reported no usage at
// all (roadmap L3.15).
//
// It is a warning rather than a failure because a provider is entitled to
// report nothing, and failing a stage over an accounting detail would be
// worse than the silence. But it must not pass unremarked: a wrong field
// name in the envelope decoder does not error, it decodes to zero, so
// unreported and mis-decoded look identical downstream — and everything
// built on those numbers, including any future budget ceiling, would then
// be protecting nothing.
func (p *Provider) warnIfUnreported(stage orchestrator.Stage, result *envelope) {
	if result.Reported() {
		return
	}
	p.logger.Warn("stage.usage.unreported",
		"stage", stage.ID,
		"detail", "the CLI returned no token counts and no cost; treat this run's usage as unmeasured, "+
			"not as zero — and check that the result envelope's shape has not changed")
}
