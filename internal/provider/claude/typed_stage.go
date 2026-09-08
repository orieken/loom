package claude

// A typed stage (roadmap L2.9) returns a state document instead of
// markdown. The agent definition is not edited to say so — those files are
// shared with the markdown pipeline, which must keep working — so the
// schema and the output instruction are appended here, at invocation time.

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/state"
)

// typedInstruction appends the schema and the output contract for a typed
// stage. The schema is inlined rather than referenced by path so the run
// does not depend on the framework being installed in the target project.
func typedInstruction(stage orchestrator.Stage, input orchestrator.StageInput, allowed []string) (string, error) {
	schema, ok := state.SchemaForKind(state.Kind(stage.StateKind))
	if !ok {
		return "", fmt.Errorf("stage %q declares state kind %q, which has no schema", stage.ID, stage.StateKind)
	}
	var instruction strings.Builder
	instruction.WriteString("\n---\n\nOUTPUT CONTRACT (this overrides any output-format instruction above).\n")
	instruction.WriteString(fileClause(allowed))
	instruction.Write(schema)
	instruction.WriteString(upstreamSection(input))
	return instruction.String(), nil
}

// fileClause states what the stage does to the working tree before it
// answers. A stage holding no edit tool is told not to write, which is what
// the contract has always said; a stage holding one is told that the JSON
// describes work it has actually done, because the JSON is a report and a
// report of unwritten code is the failure mode this clause exists to
// prevent (see writesFiles).
func fileClause(allowed []string) string {
	if !writesFiles(allowed) {
		return "Return a single JSON object conforming to this schema, and nothing else.\n" +
			"Do not write files. Do not add commentary before or after the JSON.\n\n"
	}
	return "Make the file changes this stage is responsible for, using the tools you have been given.\n" +
		"Then return a single JSON object conforming to this schema as your final output.\n" +
		"The JSON REPORTS that work; it does not replace it. Every path you list as created or modified\n" +
		"must be a file you actually wrote this session — verify with git status before answering.\n" +
		"Do not add commentary before or after the JSON.\n\n"
}

// upstreamSection hands the agent the projected fields of each stage it
// reads — the data it is allowed to see, rather than documents to reparse.
// One labelled block per upstream: which stage a fact came from is part of
// what the consumer needs to know.
func upstreamSection(input orchestrator.StageInput) string {
	if len(input.UpstreamState) == 0 {
		return ""
	}
	var section strings.Builder
	section.WriteString("\n\nInput from earlier stages. These fields are your source of truth — do not go looking for their markdown.\n")
	for _, upstream := range sortedUpstreams(input.UpstreamState) {
		fmt.Fprintf(&section, "\nFrom %s:\n\n%s\n", upstream, input.UpstreamState[upstream])
	}
	return section.String()
}

// sortedUpstreams keeps the prompt deterministic: the same run must build
// the same prompt, and Go map order is not.
func sortedUpstreams(upstreams map[string][]byte) []string {
	names := make([]string, 0, len(upstreams))
	for name := range upstreams {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// extractJSON pulls the state document out of an agent's response. Raw JSON
// is accepted, and so is the last fenced block that holds an object.
//
// This used to require the response be *entirely* one fenced block, and the
// third real end-to-end run died on it (roadmap L3.21): qa-engineer returned
// a complete, valid, schema-conformant state document preceded by the single
// sentence "Now producing the final QA state JSON." The run halted and the
// attempt still billed $0.69. The prompt above does say not to add
// commentary, and the model disregarded it, so the parser cannot treat that
// instruction as a guarantee — models narrate as a reflex, and failing a run
// over one is reporting a habit as a modelling error.
//
// The last block, specifically, keeps the property the old strictness
// existed for: not adopting a schema example the agent quoted back. A quote
// precedes the real answer; it does not follow it.
func extractJSON(response []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(response)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("agent returned nothing")
	}
	if trimmed[0] == '{' {
		return trimmed, nil
	}
	return lastFencedObject(string(trimmed))
}

// lastFencedObject returns the final fenced block whose body looks like a
// JSON object. Blocks that hold something else — a shell command, a diff —
// are passed over rather than failing the response.
func lastFencedObject(text string) ([]byte, error) {
	blocks := fencedBlocks(text)
	for index := len(blocks) - 1; index >= 0; index-- {
		body := strings.TrimSpace(blocks[index])
		if strings.HasPrefix(body, "{") {
			return []byte(body), nil
		}
	}
	return nil, unexpectedResponse(text)
}

func fencedBlocks(text string) []string {
	scan := &fenceScan{}
	for _, line := range strings.Split(text, "\n") {
		scan.consume(line)
	}
	return scan.blocks
}

// fenceScan collects fenced block bodies as lines arrive. An unterminated
// final block is discarded: a truncated response is not a state document.
type fenceScan struct {
	blocks   []string
	current  []string
	isInside bool
}

func (scan *fenceScan) consume(line string) {
	if strings.HasPrefix(strings.TrimSpace(line), "```") {
		scan.toggle()
		return
	}
	if scan.isInside {
		scan.current = append(scan.current, line)
	}
}

func (scan *fenceScan) toggle() {
	if scan.isInside {
		scan.blocks = append(scan.blocks, strings.Join(scan.current, "\n"))
		scan.current = nil
	}
	scan.isInside = !scan.isInside
}

func unexpectedResponse(text string) error {
	return fmt.Errorf("agent did not return a JSON state document — got: %s", truncate(text, 800))
}
