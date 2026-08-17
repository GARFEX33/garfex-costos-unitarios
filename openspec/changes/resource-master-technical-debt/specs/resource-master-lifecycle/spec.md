# Resource Master Lifecycle Specification

## Purpose

Define explicit resource deactivation, inactive discovery, reactivation, and consistent active-state behavior across application, repository, and TUI.

## Requirements

### Requirement: Consistent active-state semantics

The system MUST represent resource lifecycle state explicitly in repository results and user-facing detail. Active-only search MUST remain the default for existing callers. Lifecycle operations MUST use `Deactivate` terminology; `Delete` MUST NOT imply physical removal or remain the authoritative operation name.

#### Scenario: Default search remains active-only

- GIVEN active and inactive resources satisfy the same search text
- WHEN a caller searches without a lifecycle scope
- THEN only active resources are returned
- AND each returned resource exposes its active state.

#### Scenario: Deactivation is visible and non-destructive

- GIVEN an active resource
- WHEN the user requests `Deactivate`
- THEN the resource becomes inactive without physical deletion
- AND subsequent detail and lifecycle views show the inactive state.

### Requirement: Explicit idempotent lifecycle transitions

The application MUST expose separate `Deactivate` and `Reactivate` capabilities and MUST execute only the requested transition. Repeating an already-applied transition MUST produce a deterministic no-op or explicit domain result, MUST NOT perform the opposite transition, and MUST preserve the resource identity.

#### Scenario: Repeated deactivation does not reactivate

- GIVEN an inactive resource
- WHEN `Deactivate` is requested again
- THEN the result is deterministic
- AND the resource remains inactive with no contradictory write.

#### Scenario: Reactivation changes only lifecycle state

- GIVEN an inactive resource whose identity key remains reserved
- WHEN `Reactivate` is requested and current catalog eligibility is valid
- THEN the same resource becomes active
- AND no replacement resource is created.

### Requirement: Inactive discovery and safe reactivation

The system MUST provide an explicit inactive-discovery mode, preserve active-only defaults, and show lifecycle state in results and detail. Reactivation MUST reject unavailable catalog references or identity conflicts and MUST leave the resource inactive when rejected.

#### Scenario: User discovers and reactivates history

- GIVEN an inactive resource outside the default search scope
- WHEN the user enables inactive discovery, selects it, and requests reactivation
- THEN the resource is shown as inactive before the action
- AND it becomes active only after eligibility and identity checks succeed.

#### Scenario: Reactivation failure preserves inactive state

- GIVEN an inactive resource whose current catalog dependency is unavailable or whose identity conflicts
- WHEN reactivation is requested
- THEN an actionable error is returned
- AND the resource remains inactive and its stored identity is unchanged.
