# Agent Runtime Harness

The P13.0 harness is a documentation-only safety contract for future guided runtimes. It does not add an agent, provider, SDK, storage backend, tool, product Skill, endpoint, or write path.

> **Principle:** The agent interprets, proposes, and coordinates. Go calculates, validates, and executes. PostgreSQL preserves business truth.

## Control before model execution

The deterministic control plane decides whether execution is permitted before a model is invoked. It owns input normalization and versioning, capability resolution, policy, limits, confirmations, immutable resolution evidence, and audit decisions. The model execution plane owns prompts, context assembly, eligible Skills, model interaction, and typed tool invocation; it cannot override a control decision.

```text
product TUI / supervised dev adapter
  -> coordinator -> CapabilityResolver -> OperationPolicy
  -> model/runtime adapter -> typed Go application service -> PostgreSQL
  -> RunEnvelope -> trace | audit | structured logs | evaluation records
```

`CapabilityResolver` resolves only known, typed tool contracts:

```text
effective_tools = runtime_capabilities
                ∩ agent_capabilities
                ∩ task_policy
                ∩ skill_requirements
```

Required capabilities are checked first. A missing capability in any input **never grants** access and returns `INSUFFICIENT_CAPABILITY`; unknown, duplicate, conflicting, malformed, or unauthorized declarations fail closed before model or tool invocation. Resolution is deterministic for identical normalized/versioned inputs and produces immutable evidence.

`OperationPolicy` is the sole authority that authorizes, denies, or requires confirmation for an operation. `CapabilityResolver` provides evidence, not authority. Prompts, models, Skills, sensors, MCP, and Engram cannot add tools, waive confirmation, replace policy, or authorize work.

## Stable outcomes and derived risk

Pre-execution failures are stable and typed: `INSUFFICIENT_CAPABILITY`, `POLICY_DENIED`, `CONFIRMATION_REQUIRED`, `LIMIT_EXCEEDED`, `SKILL_INVALID`, `SKILL_INTEGRITY_MISMATCH`, `CONTEXT_UNAVAILABLE`, and `DEPENDENCY_UNAVAILABLE`. Invocation outcomes remain distinct: `TOOL_INVALID_ARGUMENT`, `TOOL_DENIED`, `TOOL_TIMEOUT`, `TOOL_FAILURE`, `MODEL_FAILURE`, and `RUN_CANCELLED`.

Risk is deterministically derived from trusted task, capability, operation, context, and policy inputs. It may tighten controls, but it is never authority. A prompt, model, or Skill cannot declare or lower effective risk.

## Product Skill supply chain

`.agents/skills/` is developer-only and must never be runtime-discoverable. Product runtime discovery is limited to `agent-skills/`. Future CI must prove this structural exclusion.

Product Skills use progressive disclosure: validate metadata first, load `SKILL.md` only when eligible, then load `references/` or `assets/` only when needed. V1 permits those content forms and rejects `scripts/` and executable content, including executable Markdown/MDX, `README.sh`, and build manifests.

The registry derives canonical identity and version from Git provenance plus a content-tree hash. A `SkillDescriptor` may describe identity, provenance, digest, format, purpose, requirements, and locations; it must not carry tools, policy, risk overrides, or other authority-bearing metadata. Eligibility requires canonical structure and root, normalized identity/version, permitted content, declared requirements, verified integrity, and no authority metadata. A Skill may require guidance or capabilities, but it never installs a tool or grants a capability.

Fixed canonical Git-root selection is required. Relative/absolute escapes, alternate repositories, integrity mismatches, and staged, `commit -a`, dirty, or empty-index ambiguity deny eligibility fail closed.

## Run evidence and privacy

Every run uses an immutable `RunEnvelope` containing run, session, and context IDs; actor and purpose; task, policy, and resolution decisions; model, prompt-template, Skill-hash, tool-contract, and capability-set versions; timestamps and limits; confirmation events; ordered trace spans; typed outcome/failure attribution; and an audit reference.

Trace explains execution. Audit proves authorized security or business events. Structured logs diagnose at their owning boundary. Evaluation records measure gates. None is business truth. Each sink requires classification, pre-emission redaction, access control, retention, deletion, and correlation rules; secrets, raw prompts/context, and sensitive values are deny-by-default.

## Evaluation and feedback governance

Evaluation progresses from deterministic domain fixtures (Materials, Documents, CFDI, and APU), to resolver/policy/limit/confirmation cases, tool-contract and forbidden-action cases, privacy/redaction and regression checks, and only then probabilistic agent/Skill scenarios after authoritative capabilities exist. Forbidden actions and database corruption have zero tolerance and block release.

Feedback is evidence only. Repeated failures may create reviewable proposals, controls, or evaluation cases; they must not self-modify production prompts, Skills, policy, or authority.

## Adapters and authority boundaries

Bubble Tea remains the product UX. ADK is an experimental outer adapter; ADK Web/API may run only as supervised development tooling through a future `garfex dev` workflow with development attestation and no product endpoint.

Same-host coordination uses typed routing and handoffs. A2A is allowed only across a verified network, process, or organizational boundary; it is not a same-host convenience protocol.

PostgreSQL-backed application services preserve business truth. MCP remains a separately governed optional outer adapter; unavailable MCP dependencies return `DEPENDENCY_UNAVAILABLE` and never permit a database bypass. Engram stores planning and delivery artifacts only and cannot authorize or persist runtime business operations.

## Roadmap and gates

| Phase | Gate and permitted scope |
| --- | --- |
| P13.0 Harness Foundation | This document and readback only. |
| P13.1 Materials agent | Requires P3, an authoritative Materials read service, the Runtime gate, and G4 provider decision; it remains read-only. |
| P13.2 | Proposal Skills with human-in-the-loop confirmation only. |
| Controlled writes | Require a later operation-specific gate plus atomic PostgreSQL and audit evidence. |

P3.5 Materials MCP remains a separate read-only pilot: it neither blocks P4 nor unlocks P13. Future implementation must remain independently reversible in stacked-to-main slices of at most 400 authored changed lines.

Open decisions remain deferred to later phases: provider/SDK, backends, retention durations, schemas, numeric thresholds, and operation-specific write rules. No present behavior is implied by this contract.
