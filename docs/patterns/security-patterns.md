# Security Patterns

`security-reviewer.md` explicitly names Adam Shostack as an influence ("You think like Adam Shostack —
threat modeling — practice defense in depth") and has a fully worked STRIDE framework with Saturday/
Sunday-specific examples baked directly into its contract. This file extracts that as a standalone
reference so the reasoning isn't locked inside one agent's prompt.

## STRIDE Threat Modeling

**Context**: A systematic checklist for finding security gaps by threat category, applied to every
feature that touches auth, API endpoints, user input, secrets, tokens, sessions, or any trust boundary
crossing (`security-reviewer.md`'s own invocation trigger).

**Structure**: Six categories, each with a question to ask and concrete examples already catalogued for
this stack:

| Threat | Question | Example gap |
|---|---|---|
| **S**poofing | Can an attacker impersonate a legitimate user or service? | JWT not verified, session fixation, missing `httpOnly` on auth cookies |
| **T**ampering | Can an attacker modify data in transit or at rest? | Missing input validation, no Zod schema on API responses |
| **R**epudiation | Can a user deny performing an action without detection? | Missing audit trail, no OTel span on sensitive operations |
| **I**nformation Disclosure | Can sensitive data leak to unauthorized parties? | PII in OTel traces, secrets in logs, user enumeration |
| **D**enial of Service | Can an attacker exhaust resources? | No rate limiting, no timeout on external calls, missing `CircuitBreaker` |
| **E**levation of Privilege | Can a user gain permissions they shouldn't have? | Missing authorization checks, IDOR, role not verified server-side |

**Trade-offs**: Working through all six categories for every feature is real overhead. The alternative —
reviewing security "when something feels risky" — is exactly what lets an entire threat category go
unconsidered because nobody thought to ask about it that day.

## Secure by Default

**Context**: The safe behavior should require no extra effort; dangerous behavior should require
explicit opt-in.

**Example**: An API client that sends auth headers by default is more secure than one that requires the
caller to remember to add them — the insecure state (unauthenticated request) should be the one that
takes deliberate action to reach, not the default.

## Defense in Depth

**Context**: No single control is sufficient — a real security posture is made of layers, and any one
missing layer is a gap even if the others hold.

**Structure**: Input validation + output encoding + authorization check + audit log is four layers, per
`security-reviewer.md`'s own example. Missing any one of the four is a finding, even though the other
three might prevent the same specific attack today.

**Related**: The Adapter Layer's role in `clean-architecture-layers.md` (translating and validating at
the boundary) is one of these layers in practice, not the whole defense.

## Least Privilege

**Context**: Every component should have the minimum permissions needed to do its job — API clients only
request scopes they use, functions only receive data they need, telemetry only emits fields required for
observability.

**Structure**: This is the same reasoning behind Cursor subagents' `readonly: true/false` flag (see
`docs/ARCHITECTURE.md`'s Cursor capability notes) and behind `shared/agents/*.md`'s own `tools:`
allowlist — an agent that can only `Read`/`Glob`/`Grep` (no `Write`/`Edit`/`Bash`) has fewer permissions
than it would need to cause damage even if its reasoning went wrong.

**Trade-offs**: Requesting narrower permissions up front means occasionally having to go back and widen
scope when a legitimate new need appears. That friction is much cheaper than the alternative: a component
that could always do more than it needed to, discovered only after something goes wrong.

**One axis, and there are two** — see the next pattern. The paragraph above treats `Write`/`Edit`/`Bash`
as a single "can cause damage" grouping. That is right about damage and silent about disclosure.

## Least Privilege on Egress

**Context**: Least privilege on *data* is not least privilege on *egress*. They are different questions
with different controls, and answering one is routinely mistaken for answering both.

A read-only role answers exactly one question — can this component **write**? It says nothing about
whether it can read everything, and nothing at all about whether it can carry what it read somewhere
else. An agent with no `Write` and no `Edit` that can still reach the network is not constrained in the
way "read-only" suggests to whoever reads the label.

**Structure**: two independent axes, asked separately.

| Axis | The question | Narrowed by |
|---|---|---|
| **Data** | What can it read, and what can it change? | Keyed reads over broad queries; no `Write`/`Edit` where none is needed |
| **Egress** | What can it send, and where? | An allowlist on anything that makes an outbound request; encoding at the output boundary |

The second axis has a trap the first does not: **egress does not require a tool that looks like egress.**
Rendered output is a path out. A markdown image URL in a report renders as a fetch to whoever controls
that URL, with no network tool called and nothing in a tool allowlist to notice. Encode at the boundary
rather than trusting that only named tools can send.

**In this framework**, the egress surface is `Bash`, and it is the whole surface — no agent declares
`WebFetch` or `WebSearch`. `Bash` is both axes at once: it can write any file and reach any host, so an
agent holding it is unconstrained on both regardless of what the rest of its `tools:` line says. That is
the reason `agent-frontmatter-contract.md` stopped claiming `tools` "enforces capability boundaries"
(see the alignment audit's `A6`): it describes declared capability, and `Bash` is a declaration that
covers everything.

**Trade-offs**: an egress allowlist is real work and is wrong at first — legitimate destinations get
blocked and someone has to widen it. The honest comparison is not allowlist-versus-nothing but
allowlist-versus-discovering-the-path-afterwards, and only one of those is recoverable.

**Enforcement, stated plainly**: loom cannot police egress inside a project it does not run in. What it
*can* hold is that its own read-only counter agents declare neither a write tool nor an egress tool —
asserted by `TestCounterAgentsDeclareNoWriteOrEgress`. Everything else in this pattern is judgment, and
is marked as such rather than implied to be checked.

## Paved Road / Golden Path

**Context**: When a security flaw is found (e.g. custom crypto, manual auth checks), the fix isn't a
patch to the specific flaw — it's mandating the pre-approved standard library or framework path (the
"Golden Path") instead of letting a hand-rolled alternative continue to exist alongside it.

**Structure**: `security-reviewer.md`'s own framing: "Security is a Design Property... If you find a
vulnerability, it is an architectural issue — not a bug to patch. Ask: what design decision made this
possible? Fix the decision, not just the symptom." Concretely, this repo's own Resilience Primitives
(see `stability-patterns.md`) are a Golden Path: `CircuitBreaker`/`ExponentialBackoffStrategy` are the
approved path, and a hand-rolled `for`/`sleep` retry loop is exactly the kind of ad hoc alternative this
principle exists to eliminate, not just flag once.

**Trade-offs**: Mandating the Golden Path removes a team's flexibility to reach for a quick one-off
fix. That's deliberate — a one-off fix for one instance of a problem class leaves every other instance of
the same class unfixed; forcing the standard path fixes the whole class at once.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
