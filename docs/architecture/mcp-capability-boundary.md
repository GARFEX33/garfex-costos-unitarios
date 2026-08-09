# MCP Capability Boundary

MCP is an optional outer adapter for future AI and integration clients. It is never GARFEX domain core, never the authority for business data, and never a required route for deterministic clients.

## Boundary at a glance

| Concern | Decision |
| --- | --- |
| Deterministic clients | TUI, Web, and CLI call application services directly. MCP availability cannot change that route. |
| AI and integration clients | An AI host or runtime reaches a module-owned MCP adapter through its MCP client. The adapter calls the same application service. |
| Business truth | PostgreSQL remains the sole authoritative business-data source. |
| AI memory | Engram may retain scoped, already-authorized memory only. It is not a fallback, cache, join participant, synchronization source, or business-query source. |

```text
TUI / Web / CLI ───────────────> application service ──> repository port ──> PostgreSQL

AI host/runtime ─> MCP client == local stdio ==> module-owned MCP adapter ─> application service
```

Dependencies continue to point inward. The adapter owns transport and policy concerns; it does not introduce MCP types into the domain or application layer, expose repositories, or bypass the application service.

## Roadmap gates

P3 PASS unlocks P4 and the optional P3.5 pilot in parallel. P3.5 does not block P4 and does not unlock P13. P13 still requires every target service to be authoritative and the Runtime gate; every later module MCP adapter likewise requires that module's authoritative application service before it can be considered.

P3.5 is a future, local-stdio Materials read pilot after the Materials evidence gate. Its discovery surface contains exactly these tools:

- `materials.search.v1`
- `materials.get.v1`

It exposes zero MCP resources. Writes and broad or generic access are explicitly out of scope.

## Future pilot contract

`materials.search.v1` will accept validated search text with bounded pagination and return a deterministic, paginated, sanitized projection. `materials.get.v1` will accept one validated opaque material identifier and return one sanitized projection or a not-found outcome. Exact JSON schemas, numeric limits, identifier rules, page and byte bounds are implementation-time decisions; this document does not invent them.

The pilot must not expose:

- writes, SQL, repositories, database tools, schemas, filesystem access, credentials, or arbitrary commands;
- generic access, direct database access, or domain/application APIs that leak transport concerns; or
- a mandatory path for TUI, Web, CLI, or other ordinary clients.

Ordinary clients remain interoperable through application ports or future HTTP/gRPC adapters. MCP-compatible hosts may reuse the focused tools, but MCP is not a universal client boundary.

## Security and operational behavior

The future adapter follows this fail-closed path:

```text
authenticate/attest principal
  -> per-tool allowlist
  -> strict input validation
  -> rate and deadline guards
  -> application service
  -> PostgreSQL
  -> sanitize and bound response
  -> audit
  -> stable MCP outcome
```

A missing identity, policy, audit dependency, or authoritative service prevents startup. Malformed, unknown, unauthenticated, or unauthorized calls stop before the application service. They must not leak inputs, secrets, SQL, stack traces, or internal details.

Stable outcomes cover success, denied, invalid argument, unknown tool, not found, service unavailable/internal failure, deadline exceeded, and rate limited. Standard output is protocol-only; structured logs go to standard error and redact secrets and control characters.

## Evidence required before implementation

Implementation is separately gated on all of the following decisions and evidence:

1. Materials evidence and an authoritative application service.
2. MCP SDK selection and a principal/attestation transport.
3. Exact input/output schemas, numeric and response bounds, and validation behavior.
4. Security evidence for authorization, audit, stream safety, timeouts, rate limits, and fail-closed startup.
5. A separate decision on sqlc; it is not selected by this boundary.

Tests begin RED-first at the adapter boundary. Table tests must prove both tool successes; denial and malformed input; unknown tools; not found; service failure; timeout/cancellation; rate limits; audit success and failure; stdout purity; stderr/result leak prevention; two-tool/zero-resource discovery; response/page bounds; and that malformed or unknown calls never invoke the application service. Integration evidence uses a fake application port for policy/transport behavior and PostgreSQL-backed services for authoritative reads. Deterministic TUI, Web, and CLI routes must also prove non-regression when MCP is absent or fails.

## Current scope

This is an architecture boundary only. It does not add an MCP SDK, a service, schemas, sqlc, dependencies, migrations, runtime behavior, or a protocol endpoint.
