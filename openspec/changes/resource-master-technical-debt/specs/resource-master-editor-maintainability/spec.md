# Resource Master Editor Maintainability Specification

## Purpose

Define a behavior-preserving editor responsibility extraction and correct the documentation and comments that describe Resource Master boundaries and semantics.

## Requirements

### Requirement: Behavior-preserving editor extraction

Editor responsibilities MAY be split into cohesive units, but extraction MUST preserve prompts, state transitions, field ordering, canonical-value handling, create/edit/duplicate behavior, cancellation, persistence calls, and user-visible error mapping. This capability MUST NOT introduce lifecycle, pagination, or catalog behavior changes.

#### Scenario: Successful workflows remain equivalent

- GIVEN characterized create, edit, and duplicate interactions
- WHEN responsibilities are extracted
- THEN each workflow produces the same prompts, values, ordering, and persistence operation
- AND the resulting detail presentation is unchanged.

#### Scenario: Failure and cancellation workflows remain equivalent

- GIVEN validation failure, persistence failure, duplicate identity, or cancellation
- WHEN the extracted editor handles the interaction
- THEN it returns the same recovery state and user-visible error semantics
- AND it does not discard a recoverable draft unexpectedly.

### Requirement: Characterization evidence protects the refactor

The editor extraction MUST be supported by focused characterization coverage for success and failure paths before and after the change. Tests MUST prove behavior parity rather than merely exercising that new files compile.

#### Scenario: Characterization detects behavioral drift

- GIVEN a changed prompt, transition, ordering, canonical value, or repository call
- WHEN the editor characterization suite runs
- THEN the mismatch is reported as a failure
- AND the extraction is not considered complete.

### Requirement: Documentation and comments state actual contracts

Resource Master documentation and comments MUST describe the TUI-to-application-to-domain-port-to-PostgreSQL direction, derived identity, active versus historical-read semantics, explicit deactivation/reactivation, bounded search hydration, and pagination. Stale delete terminology and claims that place validation authority only in the TUI MUST be corrected. Supplier Master and future master catalogs MUST remain out of scope.

#### Scenario: Documentation matches settled behavior

- GIVEN the implemented Resource Master contracts are reviewed
- WHEN documentation and comments are checked against actual symbols and flows
- THEN terminology, identity, lifecycle, search, and architecture statements are accurate
- AND no TUI-to-PostgreSQL shortcut or Supplier Master design is documented.
