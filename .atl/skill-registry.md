# Skill Registry

**Delegator use only.** Any agent that launches sub-agents reads this registry to resolve compact rules, then injects them directly into sub-agent prompts. Sub-agents do NOT read this registry or individual SKILL.md files.

See `_shared/skill-resolver.md` for the full resolution protocol.

## User Skills

| Trigger | Skill | Path |
|---------|-------|------|
| When creating a pull request, opening a PR, or preparing changes for review. | branch-pr | `C:\Users\Jcarl\.config\opencode\skills\branch-pr\SKILL.md` |
| When writing Go tests, using teatest, or adding test coverage. | go-testing | `C:\Users\Jcarl\.config\opencode\skills\go-testing\SKILL.md` |
| When creating a GitHub issue, reporting a bug, or requesting a feature. | issue-creation | `C:\Users\Jcarl\.config\opencode\skills\issue-creation\SKILL.md` |
| When the user asks for judgment day, adversarial review, dual review, doble review, or equivalent. | judgment-day | `C:\Users\Jcarl\.config\opencode\skills\judgment-day\SKILL.md` |
| When the user asks to create a new skill, add agent instructions, or document patterns for AI. | skill-creator | `C:\Users\Jcarl\.config\opencode\skills\skill-creator\SKILL.md` |

## Compact Rules

Pre-digested rules per skill. Delegators copy matching blocks into sub-agent prompts as `## Project Standards (auto-resolved)`.

### branch-pr
- Every PR MUST link an issue carrying `status:approved`.
- Every PR MUST have exactly one `type:*` label and all automated checks must pass.
- Branch names MUST match `type/description`, using an allowed conventional type and lowercase `a-z0-9._-` description.
- Use conventional commits; never add `Co-Authored-By` trailers.
- PR bodies MUST include issue closure, one PR type, summary, changes table, test plan, and completed checklist.
- Verify the approved issue before creating the branch or PR.
- Run applicable checks before opening the PR; do not bypass repository enforcement.

### go-testing
- Prefer table-driven tests with named subtests for multiple inputs and outcomes.
- Test both success and error paths whenever a function can return an error.
- Test Bubble Tea state transitions through `Model.Update()` directly.
- Use `teatest.NewTestModel()` for complete interactive TUI flows.
- Use golden files for stable visual output and keep fixtures under `testdata/`.
- Isolate filesystem work with `t.TempDir()` and system commands behind interfaces/mocks.
- Mark real-command integration tests so they can be skipped with `-short`.
- Keep tests beside production files using Go's `*_test.go` convention.

### issue-creation
- Search existing issues before creating a new one.
- Blank issues are forbidden; use the bug-report or feature-request template.
- Complete every required field and pre-flight checkbox.
- New issues begin at `status:needs-review`; implementation must wait for `status:approved`.
- Route questions to Discussions rather than issues.
- Use bug templates for reproducible incorrect behavior and feature templates for new capabilities.
- Do not open a linked PR until a maintainer has approved the issue.

### judgment-day
- Resolve relevant compact rules from the skill registry before launching reviewers.
- Launch two independent blind judges in parallel with identical scope, criteria, and standards.
- Classify warnings as real only when normal intended use can trigger them; otherwise report them as theoretical INFO.
- Synthesize findings as confirmed, suspect, or contradictory; never auto-fix suspect findings.
- Ask for user confirmation before applying Round 1 confirmed fixes.
- After fixes, immediately re-run both judges; never commit or push before re-judgment.
- Approval requires zero confirmed CRITICALs and zero confirmed real WARNINGs.
- After two fix iterations with remaining issues, ask whether to continue; never escalate automatically.
- Do not declare completion until every judgment reaches APPROVED or ESCALATED.

### skill-creator
- Create a skill only for reusable, non-trivial guidance; reference existing documentation instead of duplicating it.
- Use lowercase hyphenated names and the standard `skills/{skill-name}/SKILL.md` layout.
- Frontmatter MUST include name, description with trigger, `Apache-2.0`, author, and semantic version string.
- Put templates and schemas in `assets/`; point `references/` only to local documentation.
- Lead with critical patterns, use decision tables where useful, and keep examples minimal.
- Include a copy-paste Commands section.
- Do not add a Keywords section, lengthy explanations, or troubleshooting sections.
- Register the completed skill in the applicable `AGENTS.md`.

## Project Conventions

| File | Path | Notes |
|------|------|-------|
| — | — | No project-level convention files detected; the project was empty at initialization. |

Read the convention files listed above for project-specific patterns and rules. No project-specific conventions were inferred.
