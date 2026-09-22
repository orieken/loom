package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/state"
)

func typedStage() orchestrator.Stage {
	return orchestrator.Stage{ID: "analyst", Agent: "analyst", StateKind: string(state.KindAnalysis)}
}

func TestTypedInstructionInlinesTheSchemaAndForbidsCommentary(t *testing.T) {
	instruction, err := typedInstruction(typedStage(), orchestrator.StageInput{}, defaultAllowedTools())
	if err != nil {
		t.Fatalf("typedInstruction: %v", err)
	}

	for _, want := range []string{"acceptanceCriteria", "$schema", "nothing else", "Do not write files"} {
		if !strings.Contains(instruction, want) {
			t.Errorf("instruction missing %q", want)
		}
	}
}

func TestTypedInstructionHandsOverTheProjectedUpstreamState(t *testing.T) {
	input := orchestrator.StageInput{UpstreamState: map[string][]byte{
		"analyst": []byte(`{"feature":"user-auth"}`),
	}}
	stage := orchestrator.Stage{ID: "architect", Agent: "architect", StateKind: string(state.KindArchitecture)}

	instruction, err := typedInstruction(stage, input, defaultAllowedTools())
	if err != nil {
		t.Fatalf("typedInstruction: %v", err)
	}

	if !strings.Contains(instruction, `{"feature":"user-auth"}`) {
		t.Error("instruction does not carry the projected upstream state")
	}
	if !strings.Contains(instruction, "do not go looking for their markdown") {
		t.Error("instruction should tell the agent the projection is its source of truth")
	}
	if !strings.Contains(instruction, "From analyst:") {
		t.Error("each upstream block should say which stage it came from")
	}
}

func TestTypedInstructionRejectsAnUnknownKind(t *testing.T) {
	stage := orchestrator.Stage{ID: "devops-engineer", Agent: "devops-engineer", StateKind: "devops"}
	if _, err := typedInstruction(stage, orchestrator.StageInput{}, defaultAllowedTools()); err == nil {
		t.Error("accepted a state kind with no schema")
	}
}

func TestExtractJSONAcceptsRawAndSingleFencedResponses(t *testing.T) {
	cases := map[string]string{
		"raw":                   `{"feature":"user-auth"}`,
		"leading whitespace":    "\n\n  {\"feature\":\"user-auth\"}\n",
		"json fence":            "```json\n{\"feature\":\"user-auth\"}\n```",
		"bare fence":            "```\n{\"feature\":\"user-auth\"}\n```",
		"fence with whitespace": "\n```json\n{\"feature\":\"user-auth\"}\n```\n",
	}
	for name, response := range cases {
		t.Run(name, func(t *testing.T) {
			payload, err := extractJSON([]byte(response))
			if err != nil {
				t.Fatalf("extractJSON: %v", err)
			}
			var decoded map[string]string
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatalf("extracted payload does not parse: %v (%s)", err, payload)
			}
			if decoded["feature"] != "user-auth" {
				t.Errorf("extracted %v", decoded)
			}
		})
	}
}

// Commentary around the document is accepted (roadmap L3.21). These cases
// were rejected until the third real end-to-end run halted on the first of
// them, having already billed for the attempt. The prompt forbids the
// preamble; the model wrote one anyway, so the parser does not rely on it.
func TestExtractJSONAcceptsADocumentSurroundedByCommentary(t *testing.T) {
	cases := map[string]struct{ response, want string }{
		// Verbatim shape of the response that halted the third real run.
		"the run-3 preamble": {
			"Now producing the final QA state JSON.\n```json\n{\"feature\":\"user-auth\"}\n```", "user-auth"},
		"trailing commentary": {
			"```json\n{\"feature\":\"user-auth\"}\n```\nLet me know if you want changes.", "user-auth"},
		"commentary both sides": {
			"Here it is:\n```json\n{\"feature\":\"user-auth\"}\n```\nDone.", "user-auth"},
		// A quoted schema example precedes the answer, so the last block is
		// the answer. This is why it is the last and not the first.
		"quoted example then answer": {
			"The schema looks like:\n```json\n{\"feature\":\"EXAMPLE\"}\n```\nMine:\n```json\n{\"feature\":\"user-auth\"}\n```", "user-auth"},
		// A non-JSON block after the answer is passed over, not fatal.
		"answer then a shell block": {
			"```json\n{\"feature\":\"user-auth\"}\n```\nRun:\n```bash\npnpm test\n```", "user-auth"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			payload, err := extractJSON([]byte(testCase.response))
			if err != nil {
				t.Fatalf("extractJSON: %v", err)
			}
			var decoded map[string]string
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatalf("extracted payload does not parse: %v (%s)", err, payload)
			}
			if decoded["feature"] != testCase.want {
				t.Errorf("extracted %q, want %q", decoded["feature"], testCase.want)
			}
		})
	}
}

// What must still fail. A response with no JSON object in it is a stage that
// did not do its job, and calling that success would hide the failure.
func TestExtractJSONRejectsAResponseWithNoStateDocument(t *testing.T) {
	cases := map[string]string{
		"empty":              "   ",
		"prose only":         "I analyzed the feature and it looks good.",
		"markdown artifact":  "# Feature Analysis\n\n## Summary\nIt does a thing.",
		"fence without json": "```bash\npnpm install\n```",
		"unterminated fence": "Here you go:\n```json\n{\"feature\":\"x\"}",
	}
	for name, response := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := extractJSON([]byte(response)); err == nil {
				t.Errorf("extractJSON accepted %q", response)
			}
		})
	}
}

func TestExtractJSONErrorShowsWhatCameBack(t *testing.T) {
	_, err := extractJSON([]byte("I could not complete this task."))
	if err == nil || !strings.Contains(err.Error(), "could not complete") {
		t.Errorf("error = %v, want it to quote the response", err)
	}
}

func TestUntypedStagePromptIsUnchanged(t *testing.T) {
	dir := t.TempDir()
	writeAgentDefinition(t, dir, "developer")
	provider := New(Config{AgentsDir: dir})

	prompt, _, err := provider.buildPrompt(orchestrator.Stage{ID: "developer", Agent: "developer"}, orchestrator.StageInput{})
	if err != nil {
		t.Fatalf("buildPrompt: %v", err)
	}
	if strings.Contains(prompt, "OUTPUT CONTRACT") {
		t.Error("an untyped stage was given the typed output contract")
	}
	if !strings.Contains(prompt, "markdown artifact on stdout") {
		t.Error("untyped stages should still be asked for markdown")
	}
}

func TestTypedStagePromptCarriesTheContract(t *testing.T) {
	dir := t.TempDir()
	writeAgentDefinition(t, dir, "analyst")
	provider := New(Config{AgentsDir: dir})

	prompt, _, err := provider.buildPrompt(typedStage(), orchestrator.StageInput{})
	if err != nil {
		t.Fatalf("buildPrompt: %v", err)
	}
	if !strings.Contains(prompt, "OUTPUT CONTRACT") || !strings.Contains(prompt, "acceptanceCriteria") {
		t.Error("typed stage prompt is missing the output contract or the schema")
	}
	// The agent definition itself is untouched — it is shared with the
	// markdown pipeline, which must keep working.
	if !strings.HasPrefix(prompt, agentDefinitionMarker) {
		t.Error("the agent definition should still lead the prompt, unmodified")
	}
}

const agentDefinitionMarker = "# Agent: scripted for tests"

func writeAgentDefinition(t *testing.T, dir, agent string) {
	t.Helper()
	body := agentDefinitionMarker + "\n\nDo the thing this agent does.\n"
	if err := os.WriteFile(filepath.Join(dir, agent+".md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write agent definition: %v", err)
	}
}

// TestAWriteCapableTypedStageIsNotForbiddenToWrite is the regression guard
// for run 4's second blocker.
//
// The typed contract said "Do not write files" to every typed stage. The
// developer holds Write/Edit/MultiEdit by declaration and is the one stage
// whose entire purpose is changing the working tree, so the contract
// cancelled the job. Experiment B's developer returned a complete,
// schema-valid ImplementationState naming four modified files against an
// empty git diff, and reported success.
func TestAWriteCapableTypedStageIsNotForbiddenToWrite(t *testing.T) {
	stage := orchestrator.Stage{ID: "developer", Agent: "developer",
		StateKind: string(state.KindImplementation)}
	allowed := []string{"Read", "Write", "Edit", "MultiEdit", "Bash", "Glob", "Grep"}

	instruction, err := typedInstruction(stage, orchestrator.StageInput{}, allowed)
	if err != nil {
		t.Fatalf("typedInstruction: %v", err)
	}
	if strings.Contains(instruction, "Do not write files") {
		t.Error("a stage holding Write/Edit was told not to write files — " +
			"the tools are granted and then forbidden by prompt")
	}
	if !strings.Contains(instruction, "Make the file changes") {
		t.Error("a write-capable stage should be told to make its file changes")
	}
}

// TestAReadOnlyTypedStageIsStillForbiddenToWrite keeps the other half: the
// clause is conditioned, not deleted.
func TestAReadOnlyTypedStageIsStillForbiddenToWrite(t *testing.T) {
	instruction, err := typedInstruction(typedStage(), orchestrator.StageInput{}, defaultAllowedTools())
	if err != nil {
		t.Fatalf("typedInstruction: %v", err)
	}
	if !strings.Contains(instruction, "Do not write files") {
		t.Error("a read-only stage should still be told not to write files")
	}
}

// TestEveryWriteCapableAgentInThePlanGetsAWritableContract closes the gap at
// the level the defect actually lived: not one stage, but the pairing of the
// plan's typed stages with the tools their shipped definitions declare.
func TestEveryWriteCapableAgentInThePlanGetsAWritableContract(t *testing.T) {
	for _, tool := range []string{"Write", "Edit", "MultiEdit", "NotebookEdit"} {
		if !writesFiles([]string{"Read", tool}) {
			t.Errorf("%s should count as a write tool", tool)
		}
	}
	for _, posture := range [][]string{
		{"Read", "Glob", "Grep"},
		{"Read", "Glob", "Grep", "Bash"},
	} {
		if writesFiles(posture) {
			t.Errorf("%v should not count as write-capable — Bash runs checks, it does not author code", posture)
		}
	}
}

// TestAnUnparseableResponseIsKeptWhole is the half of roadmap L2.26 the
// run-4 audit recorded as only partly done. The error quotes the first 800
// characters, which made it readable and made it lossy: in that run the
// quote cut off before `knownGaps`. The quote is fine; losing the rest is
// not.
func TestAnUnparseableResponseIsKeptWhole(t *testing.T) {
	dir := t.TempDir()
	stage := orchestrator.Stage{ID: "security-reviewer", Agent: "security-reviewer", StateKind: string(state.KindSecurity)}
	input := orchestrator.StageInput{WorkspaceDir: dir}
	response := "I was unable to produce the state document.\n" +
		strings.Repeat("reasoning about the threat model. ", 100) +
		"knownGaps: the tail nobody could read"

	_, err := (&Provider{}).outputFor(stage, input, &envelope{Result: response})
	if err == nil {
		t.Fatal("outputFor accepted a response that is not a state document")
	}

	kept := filepath.Join(dir, state.TypedStateDir, "security-reviewer.rejected.txt")
	if !strings.Contains(err.Error(), kept) {
		t.Errorf("error does not name where the response was kept:\n%v", err)
	}
	body, readErr := os.ReadFile(kept)
	if readErr != nil {
		t.Fatalf("response was not kept: %v", readErr)
	}
	if string(body) != response {
		t.Errorf("kept response is %d bytes, want the whole %d", len(body), len(response))
	}
	if !strings.Contains(string(body), "knownGaps") {
		t.Error("the tail was lost — the run-4 'partial' finding, unfixed")
	}
}
