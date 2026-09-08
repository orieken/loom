# Goal Description

Establish a comprehensive strategy to minimize LLM token usage and reduce context churn within the Loom Context Engineering Framework. This guide outlines the implementation paths for six key token optimization strategies, allowing the team to integrate them systematically once the current audits are complete.

## User Review Required

> [!IMPORTANT]
> **Prioritization**: This is a sweeping set of optimizations. Review the proposed changes below and decide which ones should be prioritized for Phase 1. (Recommendation: Start with Prompt Caching and AST-Based Context Pruning).

## Open Questions

> [!WARNING]
> **Tooling Choices**: For AST parsing and Symbol Mapping, do we want to rely on existing binaries (e.g., `ctags`, `tree-sitter` CLI) executed via the framework, or build native Go parsers directly into the `loom` CLI? native Go is faster and more portable but requires more initial development effort.

## Proposed Changes

The optimizations are broken down into independent modules that can be integrated into the framework incrementally.

### 1. Native Prompt Caching Enablement
*Target: System Prompts & Orchestrator*
- **Action**: Refactor the prompt builder in `internal/orchestrator` to strictly separate static and dynamic context.
- **Implementation**: Place `AGENTS.md`, `DOMAIN_DICTIONARY.md`, static KIs, and the `CODEMAP` at the absolute beginning of the prompt. Ensure these blocks are immutable across turns to trigger Anthropic/Gemini prompt caching.

### 2. AST-Based Context Pruning (Slice Extraction)
*Target: `context-engineer` Skill / Context Manifest*
- **Action**: Build an AST extraction tool (e.g., `loom extract --file <file> --symbol <func>`).
- **Implementation**: When an agent requests context for a specific function, use `tree-sitter` (or native Go AST) to extract only the method signature, its struct, and its body. Inject this slice into the `context-manifest.md` instead of the full 2,000-line file.

### 3. Context Compression (Minification)
*Target: File Reader / MCP Server*
- **Action**: Introduce a `--minify` flag to the tool that reads files into the context window.
- **Implementation**: Run files through a fast regex or tokenizer to strip block comments, inline comments, empty lines, and standard library imports before presenting them to the LLM.

### 4. Semantic Symbol Mapping (Codemap Expansion)
*Target: `loom codemap` CLI*
- **Action**: Extend the `loom codemap` tool to generate a `symbols.json` map.
- **Implementation**: Parse the repository to map every class, function, and interface to its file and line number. Agents will query this symbol map to locate definitions instantly, bypassing the need for exploratory `grep` loops.

### 5. Specialized Agent Contexts (Rule Pruning)
*Target: Agent Configuration / `shared/rules`*
- **Action**: Split the monolithic `AGENTS.md` (which contains rules for C#, Go, Java, Infrastructure) into specialized rule blocks.
- **Implementation**: Add logic to the orchestrator to dynamically assemble the system prompt based on the files touched in the feature spec. If only `.go` files are touched, omit Java and C# rules entirely.

### 6. Pipeline State Summarization
*Target: `loom run` Orchestrator*
- **Action**: Introduce a "memory compression" stage between pipeline handoffs.
- **Implementation**: Use a fast, inexpensive model (e.g., Haiku or Flash) to summarize the verbose artifacts from the previous stage (like `analysis.md`) into concise bullet points before passing them to the next agent in the sequence.

## Verification Plan

### Automated Tests
- Implement token-counting telemetry in `internal/telemetry` to baseline current token usage per run.
- Write unit tests for the AST extractor to ensure it successfully captures complete method bodies and signatures without truncating.

### Manual Verification
- Execute an end-to-end `loom run` using the standard test fixture before and after enabling Prompt Caching to verify cost/token reductions.
- Verify that minified files still compile and retain enough context for the LLM to understand the logic.
