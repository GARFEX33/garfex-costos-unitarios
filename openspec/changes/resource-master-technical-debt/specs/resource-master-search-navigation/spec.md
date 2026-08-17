# Resource Master Search and Navigation Specification

## Purpose

Define bounded search hydration, deterministic pagination, and filter-preserving next/previous navigation for Resource Master.

## Requirements

### Requirement: Bounded set-based search hydration

The repository MUST hydrate a requested result page set-wise rather than issuing one detail read per result. Query work MUST remain bounded and independent of page size, while preserving search filters, lifecycle scope, deterministic ordering, and canonical resource reconstruction. A hydration inconsistency MUST fail the page rather than return silently partial resources.

#### Scenario: Larger pages do not create per-result queries

- GIVEN identical filters and requested pages of one, ten, and fifty results
- WHEN search executes
- THEN each page is hydrated through bounded set-based operations
- AND query count does not grow linearly with result count.

#### Scenario: Search preserves canonical results

- GIVEN matching resources with attributes and a class filter
- WHEN a page is loaded through the set-based path
- THEN every result has the same canonical values and identity as direct hydration
- AND no result outside the filters is returned.

### Requirement: Stable page contract and ordering

Search MUST return a page contract containing the requested filters, limit, offset, deterministic ordering, and enough boundary metadata to determine whether next or previous pages exist. Ordering MUST include stable tie-breakers so records do not move between adjacent pages when their primary display fields compare equally.

#### Scenario: Adjacent pages are reproducible

- GIVEN a fixed search scope and stable stored data
- WHEN the caller requests consecutive pages
- THEN the pages have no duplicate or skipped resource caused by equal sort values
- AND the original class and lifecycle filters remain attached to both pages.

#### Scenario: End boundary is explicit

- GIVEN a page has no next results
- WHEN the caller evaluates navigation metadata
- THEN `hasNext` is false
- AND the current page and selection remain unchanged if next is requested.

### Requirement: Filter-preserving next and previous navigation

The TUI/application flow MUST expose next and previous navigation only through the application search contract. Navigation MUST retain query text, class scope, lifecycle scope, ordering, and valid selection; it MUST not call PostgreSQL directly from the TUI.

#### Scenario: Next page retains context

- GIVEN a selected result page with active filters and a next page
- WHEN the user requests next
- THEN the next page is loaded with the same filters and ordering
- AND selection is reset or retained only according to the page contract.

#### Scenario: Previous at the first page is safe

- GIVEN the first page is displayed
- WHEN the user requests previous
- THEN no repository mutation or out-of-range query occurs
- AND the first page and filters remain visible.
