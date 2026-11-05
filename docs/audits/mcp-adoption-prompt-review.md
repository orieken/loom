# MCP adoption prompt review

This review compares the existing installation, bridge, update, and extraction prompts with the current repository.
It is a static documentation review. No downstream project installation, MCP client integration, or live API operation was performed.

## Findings and disposition

| Finding | Evidence | Disposition |
|---|---|---|
| The bridge prompt instructs teams to copy deleted Go templates. | `docs/prompts/done/install-framework-with-mcp-bridge.md` references `shared/mcp-patterns/go/`; `shared/mcp-patterns/README.md` explains its removal. | Marked the old prompt historical and added a current adoption prompt. |
| The bridge prompt describes `search_ki` as LLM-based retrieval. | `shared/mcp/README.md` describes deterministic lexical retrieval; the register API describes BM25 full-text search. | The new prompt requires checking the selected version and treats retrieval as deterministic. |
| The bridge prompt assumes a Saturday-shaped project and a full framework installation. | Its discovery and execution phases prescribe `./project/` paths and framework installation before bridging. | The new prompt discovers actual paths and selects standalone serving, Go embedding, or a necessary language port. |
| The bridge prompt references a missing bootstrap prompt and a downstream repository as always relevant. | `docs/prompts/bootstrap-project.md` is absent; the downstream example is not required for the supported registry API. | The replacement uses local authoritative references and does not require a Saturday clone. |
| The update prompt still directs bridge updates toward copied templates and delegates execution to an older prompt. | Pattern B and Phase B in `docs/prompts/update-installed-framework.md`. | Added a compatibility notice directing MCP updates to the current adoption guide and preserving local modifications. The older execution sequence remains historical reference, not a newly verified update procedure. |
| The extraction handoff can be mistaken for current distribution guidance. | `docs/prompts/done/add-mcp-patterns-directory.md` describes introducing copies later removed. | Added a historical notice with current guidance links. |
| Old prompts prescribe commits as workflow steps. | Discovery, planning, and execution phases contain commit commands or mandatory commit wording. | Added explicit approval clarification to bridge and update prompts; the new adoption prompt never treats a plan as commit authorization. |

## Current integration choices

- Run `loom mcp serve` for a separate stdio server.
- In Go, adapt the transport-free registry returned by `register.Frameworks` to the host server's SDK.
  `register.FrameworksAt` allows an explicit workspace root; the default confines paths to the working directory.
- If a native language port is necessary, use the porting guide and current compiled implementation as references.
  Ports need their own tests and update owner.

The supported API is documented in `shared/mcp/register/register.go` and demonstrated by `examples/embedding/`.
The new [adoption prompt](../prompts/adopt-framework-mcp-tools.md) asks adopters to verify versions, discovery,
actual invocation, invalid input, path boundaries, index prerequisites, and protocol logging.

## Remaining work

- Trial the new prompt against a real downstream project before claiming adoption success.
- Fully modernize the framework update prompt's non-MCP installation and platform-generation instructions in a separate review.
  The compatibility notice requires checking current CLI help rather than assuming the legacy commands are current.
- Reconcile porting-guide behavior with any selected release during an actual port. This review did not validate each guide's implementation details.
- Historical handoffs remain archived for provenance; they are not executable adoption recipes.
