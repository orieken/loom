package claude_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/claude"
	"github.com/orieken/loom/internal/telemetry"
)

// Leading YAML frontmatter mirrors real shared/agents/*.md files — it is why
// the prompt must travel over stdin, never argv (the claude CLI would parse
// a leading `---` as an option).
const testAgentDefinition = "---\nname: analyst\n---\n# analyst agent\nYou analyze feature specs."

// writeFakeClaude writes an executable shell script standing in for the
// claude CLI, so subprocess plumbing (arg construction, stdout capture,
// timeout kill, exit codes) is tested without the real binary.
func writeFakeClaude(t *testing.T, dir, script string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake-binary tests use POSIX shell scripts")
	}
	path := filepath.Join(dir, "fake-claude")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}
	return path
}

// envelopeScript builds a fake CLI that prints a `--output-format json`
// result envelope carrying the given agent output, with realistic usage
// numbers. Every test that expects a successful invocation goes through
// this, because a bare `echo` is no longer a valid CLI response.
func envelopeScript(result string) string {
	return `cat <<'ENVELOPE'
{"type":"result","subtype":"success","is_error":false,
 "result":` + strconv.Quote(result) + `,
 "total_cost_usd":0.0425,
 "usage":{"input_tokens":1200,"output_tokens":340,
          "cache_read_input_tokens":800,"cache_creation_input_tokens":64},
 "modelUsage":{"claude-opus-5":{"inputTokens":1200}}}
ENVELOPE`
}

func newTestProvider(t *testing.T, binaryPath string) (*claude.Provider, orchestrator.Stage, orchestrator.StageInput) {
	t.Helper()
	agentsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(agentsDir, "analyst.md"), []byte(testAgentDefinition), 0o644); err != nil {
		t.Fatalf("write agent definition: %v", err)
	}
	provider := claude.New(claude.Config{
		BinaryName: binaryPath,
		AgentsDir:  agentsDir,
		Logger:     slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	workspace := t.TempDir()
	stage := orchestrator.Stage{ID: "analyst", Agent: "analyst", Timeout: 5 * time.Second}
	input := orchestrator.StageInput{SpecPath: "/specs/user-auth.md", WorkspaceDir: workspace}
	return provider, stage, input
}

func TestInvokeCapturesStdoutToArtifact(t *testing.T) {
	binary := writeFakeClaude(t, t.TempDir(), envelopeScript("# analysis output"))
	provider, stage, input := newTestProvider(t, binary)

	output, err := provider.Invoke(context.Background(), stage, input)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	wantPath := filepath.Join(input.WorkspaceDir, "analyst.md")
	if output.ArtifactPath != wantPath {
		t.Errorf("artifact path = %q, want %q", output.ArtifactPath, wantPath)
	}
	content, err := os.ReadFile(output.ArtifactPath)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if strings.TrimSpace(string(content)) != "# analysis output" {
		t.Errorf("artifact content = %q, want the fake agent's stdout", content)
	}
}

func TestInvokeSendsPromptOverStdinNotArgv(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	stdinFile := filepath.Join(dir, "stdin.txt")
	// The fake dumps argv and stdin separately so the test can assert both.
	binary := writeFakeClaude(t, dir, `printf '%s\n' "$@" > `+argsFile+`
cat > `+stdinFile+`
`+envelopeScript("# analysis output"))
	provider, stage, input := newTestProvider(t, binary)

	if _, err := provider.Invoke(context.Background(), stage, input); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	assertPromptPlumbing(t, argsFile, stdinFile, input)
}

func assertPromptPlumbing(t *testing.T, argsFile, stdinFile string, input orchestrator.StageInput) {
	t.Helper()
	args := strings.Fields(readFile(t, argsFile))
	// The property is that the prompt is NOT in argv, not that argv is a
	// fixed list — pinning the list made adding the permission flags look
	// like a regression (roadmap L2.22).
	for _, want := range []string{"-p", "--output-format", "json"} {
		if !slices.Contains(args, want) {
			t.Errorf("argv %v is missing %q — the JSON envelope is where token counts come from", args, want)
		}
	}
	if strings.Contains(strings.Join(args, " "), "You are a") {
		t.Errorf("argv = %v carries the prompt; it must travel over stdin (agent frontmatter starts with ---)", args)
	}
	prompt := readFile(t, stdinFile)
	for _, want := range []string{testAgentDefinition, input.SpecPath, input.WorkspaceDir} {
		if !strings.Contains(prompt, want) {
			t.Errorf("stdin prompt missing %q", want)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

func TestInvokeFailsClearlyWhenBinaryMissing(t *testing.T) {
	provider, stage, input := newTestProvider(t, "definitely-not-a-real-binary-xyz")

	_, err := provider.Invoke(context.Background(), stage, input)
	if err == nil {
		t.Fatal("Invoke succeeded with a missing binary; must fail, never fall back to mock")
	}
	for _, want := range []string{"not found", "npm install -g @anthropic-ai/claude-code"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing-binary error %q lacks remediation text %q", err, want)
		}
	}
}

func TestInvokeFailsWhenAgentDefinitionMissing(t *testing.T) {
	binary := writeFakeClaude(t, t.TempDir(), envelopeScript("ok"))
	provider, stage, input := newTestProvider(t, binary)
	stage.Agent = "no-such-agent"

	_, err := provider.Invoke(context.Background(), stage, input)
	if err == nil || !strings.Contains(err.Error(), "no-such-agent.md") {
		t.Fatalf("Invoke error = %v, want missing agent definition naming the path", err)
	}
}

// The script backgrounds its sleep and waits on it, so /bin/sh forks a
// grandchild on every platform. Without that, macOS's sh exec's the sleep
// and the direct child IS the sleep, which hides the bug this test exists
// for: exec.CommandContext kills the direct child only, and a grandchild
// that inherited the output pipe keeps Wait blocked until it exits. Ubuntu's
// dash forks either way, which is why this failed only in CI.
func TestInvokeKillsSubprocessOnTimeout(t *testing.T) {
	binary := writeFakeClaude(t, t.TempDir(), "sleep 30 &\nwait")
	provider, stage, input := newTestProvider(t, binary)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := provider.Invoke(ctx, stage, input)
	if err == nil {
		t.Fatal("Invoke succeeded despite the deadline")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Invoke error = %v, want context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("subprocess was not killed on deadline: took %v", elapsed)
	}
}

// WaitDelay bounds the wait for output, and that must not turn a successful
// run into a failure. An agent that leaves anything running — an MCP server,
// a watcher, a stray tool — keeps the output pipe open after it exits, so
// Wait returns ErrWaitDelay even though the envelope arrived and the agent
// succeeded. A lingering grandchild is not this stage's failure.
func TestInvokeSucceedsWhenTheAgentLeavesAChildRunning(t *testing.T) {
	binary := writeFakeClaude(t, t.TempDir(), "sleep 30 &\n"+envelopeScript("# analysis output"))
	provider, stage, input := newTestProvider(t, binary)

	output, err := provider.Invoke(context.Background(), stage, input)
	if err != nil {
		t.Fatalf("Invoke() = %v, want success despite a lingering child", err)
	}
	content, readErr := os.ReadFile(output.ArtifactPath)
	if readErr != nil {
		t.Fatalf("read artifact: %v", readErr)
	}
	if strings.TrimSpace(string(content)) != "# analysis output" {
		t.Errorf("artifact content = %q, want the envelope result", content)
	}
}

func TestInvokeReportsNonzeroExitWithStderr(t *testing.T) {
	binary := writeFakeClaude(t, t.TempDir(), `echo "agent blew up" >&2; exit 3`)
	provider, stage, input := newTestProvider(t, binary)

	_, err := provider.Invoke(context.Background(), stage, input)
	if err == nil {
		t.Fatal("Invoke succeeded despite nonzero exit")
	}
	if !strings.Contains(err.Error(), "agent blew up") {
		t.Errorf("error %q does not surface the subprocess stderr", err)
	}
}

// The envelope is where token counts and cost come from, so a stage must
// carry them out. These numbers are reported, never computed here.
func TestInvokeReportsUsageFromTheEnvelope(t *testing.T) {
	binary := writeFakeClaude(t, t.TempDir(), envelopeScript("# analysis output"))
	provider, stage, input := newTestProvider(t, binary)

	output, err := provider.Invoke(context.Background(), stage, input)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if output.Usage == nil {
		t.Fatal("stage reported no usage — the envelope carried it")
	}
	want := orchestrator.Usage{
		Model: "claude-opus-5", InputTokens: 1200, OutputTokens: 340,
		CacheReadTokens: 800, CacheCreationTokens: 64, CostUSD: 0.0425,
	}
	if *output.Usage != want {
		t.Errorf("usage = %+v, want %+v", *output.Usage, want)
	}
}

// A malformed envelope must fail the stage. Falling back to treating raw
// stdout as the artifact would make a broken run look like a successful one
// that happened to cost nothing — reintroducing exactly the unmeasured
// number this whole item removes.
func TestInvokeFailsOnAMalformedEnvelopeRatherThanFallingBack(t *testing.T) {
	binary := writeFakeClaude(t, t.TempDir(), `echo "# not an envelope, just markdown"`)
	provider, stage, input := newTestProvider(t, binary)

	_, err := provider.Invoke(context.Background(), stage, input)
	if err == nil {
		t.Fatal("Invoke succeeded on non-envelope output; it must fail the stage")
	}
	// The raw output has to survive into the error, or the failure is
	// undiagnosable.
	if !strings.Contains(err.Error(), "not an envelope") {
		t.Errorf("error %q does not preserve the raw output", err)
	}
	if _, statErr := os.Stat(filepath.Join(input.WorkspaceDir, "analyst.md")); statErr == nil {
		t.Error("an artifact was written despite the envelope failing to parse")
	}
}

// An envelope the CLI itself marks as an error is a failed stage, even
// though the process exited zero.
func TestInvokeFailsWhenTheEnvelopeReportsAnError(t *testing.T) {
	binary := writeFakeClaude(t, t.TempDir(),
		`echo '{"type":"result","subtype":"error_during_execution","is_error":true,"result":"context limit reached"}'`)
	provider, stage, input := newTestProvider(t, binary)

	_, err := provider.Invoke(context.Background(), stage, input)
	if err == nil {
		t.Fatal("Invoke succeeded on an is_error envelope")
	}
	for _, want := range []string{"error_during_execution", "context limit reached"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// Fields loom does not read must not break a run: the CLI adds them over
// time, and a strict decode would turn an upgrade into a failed pipeline.
func TestInvokeIgnoresUnknownEnvelopeFields(t *testing.T) {
	binary := writeFakeClaude(t, t.TempDir(),
		`echo '{"type":"result","is_error":false,"result":"# out","total_cost_usd":0.01,`+
			`"usage":{"input_tokens":5,"output_tokens":7},"some_future_field":{"nested":true},"num_turns":3}'`)
	provider, stage, input := newTestProvider(t, binary)

	output, err := provider.Invoke(context.Background(), stage, input)
	if err != nil {
		t.Fatalf("Invoke rejected an envelope carrying an unknown field: %v", err)
	}
	if output.Usage.InputTokens != 5 || output.Usage.CostUSD != 0.01 {
		t.Errorf("usage = %+v, want the fields loom does read", *output.Usage)
	}
}

// With several models in modelUsage no single name is true, so none is
// reported. An attribute that names an arbitrary member of a set is worse
// than an absent one.
func TestInvokeReportsNoModelWhenSeveralServedTheRequest(t *testing.T) {
	binary := writeFakeClaude(t, t.TempDir(),
		`echo '{"type":"result","is_error":false,"result":"# out","usage":{"input_tokens":1},`+
			`"modelUsage":{"claude-opus-5":{},"claude-haiku-4-5":{}}}'`)
	provider, stage, input := newTestProvider(t, binary)

	output, err := provider.Invoke(context.Background(), stage, input)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if output.Usage.Model != "" {
		t.Errorf("model = %q, want empty when several models served the request", output.Usage.Model)
	}
}

// The subprocess must carry TRACEPARENT so a tool call the agent makes can
// land under the stage that caused it. Env is the only channel: MCP carries
// no trace context, and loom does not spawn the MCP server.
func TestInvokeExportsTraceParentToTheSubprocess(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.txt")
	binary := writeFakeClaude(t, dir, `printenv > `+envFile+`
`+envelopeScript("# analysis output"))
	provider, stage, input := newTestProvider(t, binary)

	session, err := telemetry.Start(telemetry.Options{
		Version: "test", TraceFile: filepath.Join(dir, "traces.jsonl"),
	})
	if err != nil {
		t.Fatalf("telemetry.Start: %v", err)
	}
	ctx, span := session.Tracer().StartStage(context.Background(), orchestrator.StageSpan{ID: "analyst"})
	if _, err := provider.Invoke(ctx, stage, input); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	span.End(orchestrator.SpanOutcome{Status: orchestrator.StageStatusCompleted})
	_ = session.Shutdown(context.Background())

	environment := readFile(t, envFile)
	if !strings.Contains(environment, telemetry.TraceParentEnvVar+"=00-") {
		t.Errorf("subprocess environment carries no %s:\n%s", telemetry.TraceParentEnvVar, environment)
	}
	// The rest of the environment must survive — replacing it rather than
	// appending would strip PATH and HOME from every agent.
	if !strings.Contains(environment, "PATH=") {
		t.Error("subprocess lost the inherited environment")
	}
}

// Untraced runs must not set the variable at all. An empty TRACEPARENT is
// worse than an absent one: a child would try to parse it.
func TestInvokeSetsNoTraceParentWhenUntraced(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.txt")
	binary := writeFakeClaude(t, dir, `printenv > `+envFile+`
`+envelopeScript("# analysis output"))
	provider, stage, input := newTestProvider(t, binary)

	if _, err := provider.Invoke(context.Background(), stage, input); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	for _, line := range strings.Split(readFile(t, envFile), "\n") {
		if strings.HasPrefix(line, telemetry.TraceParentEnvVar+"=") {
			t.Errorf("untraced run exported %q", line)
		}
	}
}
