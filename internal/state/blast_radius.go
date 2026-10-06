package state

// BlastRadius is what a change reaches, recorded in the context manifest so the
// stages that follow pin files by real dependency rather than by guess
// (ADR-010, Phase 8).
//
// The agent supplies only the question: which paths change and how far to look.
// The answer is computed by the executor from the repo graph and written over
// whatever the agent put there — the same rule the token budget follows
// (roadmap L3.25). A model transcribing a tool's output can mis-transcribe it,
// and a fabricated "nothing depends on this" is the one error this section
// exists to prevent.

import "fmt"

// MaxBlastRadiusEntries caps the recorded list, so a manifest stays small
// whatever the repository's size. Total always says how many there really are.
const MaxBlastRadiusEntries = 50

// BlastImpacted is one directory the change reaches.
type BlastImpacted struct {
	Path       string `json:"path"`
	Distance   int    `json:"distance"`
	Confidence string `json:"confidence" jsonschema:"enum=exact,enum=approximate"`
}

// BlastRadius is the typed blast-radius section of a context manifest.
type BlastRadius struct {
	Seeds []string `json:"seeds" jsonschema:"required,minItems=1,description=The repo-relative paths the change touches"`
	Depth int      `json:"depth,omitempty" jsonschema:"minimum=1,maximum=10,description=How many imports to follow (default 3)"`

	// Everything below is computed by loom from the repo graph; anything the
	// agent supplies is replaced.
	GraphSHA   string          `json:"graphSha,omitempty" jsonschema:"description=Computed by loom. Do not supply"`
	Fresh      bool            `json:"fresh,omitempty" jsonschema:"description=Computed by loom. Do not supply"`
	Total      int             `json:"total,omitempty" jsonschema:"description=Computed by loom. Do not supply"`
	Impacted   []BlastImpacted `json:"impacted,omitempty" jsonschema:"description=Computed by loom. Do not supply"`
	Truncated  bool            `json:"truncated,omitempty" jsonschema:"description=Computed by loom. Do not supply"`
	Unresolved string          `json:"unresolved,omitempty" jsonschema:"description=Computed by loom. Do not supply"`
}

// BlastResolution is a resolver's answer for one set of seeds.
type BlastResolution struct {
	GraphSHA string
	Fresh    bool
	Impacted []BlastImpacted
}

// BlastRadiusResolver computes the directories that transitively depend on the
// seeds. It is injected, never called directly, so this package stays free of
// the repo graph and the filesystem (architecture-guardrails.md #1).
type BlastRadiusResolver func(seeds []string, depth int) (BlastResolution, error)

// BlastResolvable is a state document whose blast radius the executor must
// compute rather than accept from an agent.
type BlastResolvable interface {
	ResolveBlastRadius(resolve BlastRadiusResolver)
}

// validate checks the part the agent controls.
func (b *BlastRadius) validate() error {
	if b == nil {
		return nil
	}
	if len(b.Seeds) == 0 {
		return &ValidationError{Field: "blastRadius.seeds", Reason: "must name at least one path, or omit blastRadius"}
	}
	for index, seed := range b.Seeds {
		if seed == "" {
			return &ValidationError{Field: fmt.Sprintf("blastRadius.seeds[%d]", index), Reason: "is empty"}
		}
	}
	if b.Depth < 0 || b.Depth > 10 {
		return &ValidationError{Field: "blastRadius.depth", Reason: fmt.Sprintf("is %d, want 1 to 10 or omitted", b.Depth)}
	}
	return nil
}

// ResolveBlastRadius replaces everything the agent supplied beyond the seeds
// and depth with the resolver's answer. A resolver that fails, or none at all,
// leaves the section naming why it is empty: an unresolved radius must never
// read as an empty one.
func (c *ContextState) ResolveBlastRadius(resolve BlastRadiusResolver) {
	radius := c.BlastRadius
	if radius == nil {
		return
	}
	*radius = BlastRadius{Seeds: radius.Seeds, Depth: radius.Depth}
	if resolve == nil {
		radius.Unresolved = "no repo graph was available to the executor; run `loom index`"
		return
	}
	resolution, err := resolve(radius.Seeds, radius.Depth)
	if err != nil {
		radius.Unresolved = err.Error()
		return
	}
	radius.GraphSHA, radius.Fresh, radius.Total = resolution.GraphSHA, resolution.Fresh, len(resolution.Impacted)
	radius.Impacted, radius.Truncated = capBlast(resolution.Impacted)
}

func capBlast(impacted []BlastImpacted) ([]BlastImpacted, bool) {
	if len(impacted) <= MaxBlastRadiusEntries {
		return impacted, false
	}
	return impacted[:MaxBlastRadiusEntries], true
}

// blastLines renders the section: the question, then the computed answer.
func (b *BlastRadius) blastLines() []string {
	lines := []string{fmt.Sprintf("- **Change touches**: %s", joinCode(b.Seeds))}
	if b.Unresolved != "" {
		return append(lines, "- **Not resolved**: "+b.Unresolved)
	}
	lines = append(lines, fmt.Sprintf("- Computed by loom from the repo graph at `%s` (%s); %d director%s depend on it.",
		shortSHA(b.GraphSHA), freshWord(b.Fresh), b.Total, plural(b.Total)))
	for _, item := range b.Impacted {
		lines = append(lines, fmt.Sprintf("  - `%s` — %d import%s away, %s", item.Path, item.Distance, pluralS(item.Distance), item.Confidence))
	}
	if b.Truncated {
		lines = append(lines, fmt.Sprintf("  - … and %d more (the list is capped)", b.Total-len(b.Impacted)))
	}
	return lines
}

func freshWord(fresh bool) string {
	if fresh {
		return "fresh"
	}
	return "STALE — it predates the working tree; run `loom index`"
}

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	if sha == "" {
		return "unknown"
	}
	return sha
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func joinCode(values []string) string {
	out := ""
	for index, value := range values {
		if index > 0 {
			out += ", "
		}
		out += "`" + value + "`"
	}
	return out
}
