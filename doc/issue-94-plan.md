# Issue #94 Plan: Add a GitHub Summary renderer in Go

- Repository: `morshoto/kgh`
- Issue: `#94`
- URL: <https://github.com/morshoto/kgh/issues/94>
- State: `OPEN`
- Labels: `Feedback: Feature Request`, `Type: New Feature`
- Assignee: `morshoto`
- Milestone: `Release for v0.0.10`
- Planning branch: `feat/94-github-summary-renderer`

## Summary

Add a small reporting unit that renders `internal/execution.Result` into stable Markdown for GitHub Actions job summaries. The renderer should cover milestone 10 reporting fields without expanding the execution model unless a required field truly cannot be derived from the current result shape.

The current codebase already has the right source-of-truth object in `internal/execution.Result`, and the GitHub workflow already writes directly to `$GITHUB_STEP_SUMMARY` in `.github/workflows/commit-trigger.yml`. The missing piece is a first-class renderer and a controlled integration path from the `github run` command.

## Goal

- Produce a concise, deterministic Markdown summary for GitHub Actions based on `execution.Result`.
- Cover the issue-required fields:
  - target name
  - notebook path
  - kernel id
  - Kaggle run status
  - submit status
  - public score
  - useful identifiers or references
- Represent missing or skipped values explicitly as `not run`, `not submitted`, `pending`, or `unavailable`.

## Non-goals

- Redesign the JSON CLI output contract from `kgh run` or `kgh github run`.
- Add new execution fields if the current `execution.Result` already exposes enough information.
- Build a generic rich reporting framework for non-GitHub consumers.
- Change the workflow trigger semantics or Kaggle lifecycle behavior.

## Current Code Seams

- `internal/execution.Result` already contains the execution spec, bundle result, push result, poll result, submission result, and score result needed for the summary.
- `cmd/kgh/main.go` centralizes execution through `executeRequest`, which is the natural place to hook summary generation without duplicating execution logic.
- `internal/github` currently resolves GitHub trigger metadata only. It is a plausible home for GitHub-specific summary writing if the write-to-file concern should stay GitHub-scoped.
- `.github/workflows/commit-trigger.yml` already appends ad hoc text to `$GITHUB_STEP_SUMMARY`, so the new renderer should complement or replace part of that handwritten output.

## Proposed Approach

1. Add a renderer with a narrow API, for example:

```go
package reporting

func RenderGitHubSummary(result execution.Result) string
```

or, if the write concern should remain GitHub-specific:

```go
package github

type SummaryWriter struct {
	Getenv func(string) string
	Append func(path string, body []byte) error
}
```

2. Keep rendering logic pure and deterministic.
   Derive display values from `execution.Result` only, and isolate any file-writing side effect behind a thin wrapper.

3. Integrate summary emission into the `github run` path only.
   `kgh github run` is the issue-specific execution path for GitHub Actions. That keeps local CLI behavior unchanged unless there is a deliberate follow-up requirement to expose the same summary elsewhere.

4. Preserve machine-readable stdout.
   Continue printing the JSON result to stdout. Write the Markdown summary separately to `$GITHUB_STEP_SUMMARY` when that environment variable is present.

5. Normalize status text.
   Define clear mapping rules for dry-run, skipped submission, missing score, pending score, failed or incomplete poll states, and absent kernel refs.

## Field Mapping

- Target name: `result.Execution.Target`
- Notebook path: `result.Bundle.NotebookPath`, else `result.Execution.NotebookPath`, else `unavailable`
- Kernel id: `result.Execution.KernelID` or `result.Push.KernelRef`, else `not run`
- Kaggle run status:
  - dry run: `not run`
  - live with poll result: derive from `result.Poll.Status` and terminal state
  - live without poll result but with push: `pending` or `unavailable`, depending on reachable state
- Submit status:
  - no submission step: `not submitted`
  - attempted but not submitted: use `result.Submission.Message` or `not submitted`
  - submitted: use `result.Submission.Status` when present
- Public score:
  - `result.Score.PublicScore` when available
  - `pending` when `result.Score.State == "pending"`
  - `unavailable` or `not submitted` for the remaining explicit states
- Useful identifiers or references:
  - kernel ref
  - competition slug
  - submission id
  - output paths when they help operators debug workflow runs

## Tasks

1. Add the summary renderer package and define the public API.
2. Implement deterministic Markdown rendering for all required fields and explicit fallback states.
3. Add unit tests for rendering across dry-run, successful live run, pending score, missing submission, and partial-result cases.
4. Add a small GitHub-specific writer that appends the rendered Markdown to `$GITHUB_STEP_SUMMARY` only when the variable is set.
5. Integrate the writer into `kgh github run` without changing `kgh run` behavior.
6. Decide whether the existing handcrafted workflow summary blocks in `.github/workflows/commit-trigger.yml` should remain as preflight messaging only, or whether some should be removed to avoid duplication.
7. Update docs if the workflow-visible behavior changes materially.

## Acceptance Criteria

- `kgh github run` writes a Markdown summary to `$GITHUB_STEP_SUMMARY` when running inside GitHub Actions.
- The summary includes every field called out in issue `#94`.
- Missing or skipped values are rendered explicitly and never as blank cells or omitted critical rows.
- The renderer uses `execution.Result` as the source of truth and does not introduce unnecessary new execution fields.
- Existing JSON stdout behavior remains intact.

## Testing Plan

- Renderer unit tests:
  - dry-run result
  - live success with kernel ref, submission id, and public score
  - live success with score pending
  - execution with submission disabled or not attempted
  - partial or missing nested structs
- Writer unit tests:
  - writes only when `GITHUB_STEP_SUMMARY` is set
  - appends expected Markdown to the configured path
- Command integration tests:
  - `github run` still emits JSON
  - summary write does not break command success paths
  - summary write failure behavior is defined and tested

## Risks And Open Questions

- Package placement:
  `internal/github` fits the sink and workflow context, while a dedicated reporting package better separates pure rendering from GitHub-specific file writing.
- Status wording:
  The issue asks for stable explicit states, so the exact normalization rules should be settled early to avoid churn in tests and workflow output.
- Failure policy:
  It is still an open decision whether summary write failures should fail the command or only log to stderr. For GitHub Actions observability, failing soft is usually safer unless the summary is considered a release-blocking artifact.
- Duplication with workflow shell output:
  The workflow currently writes pre-execution and mode notes directly. The final implementation should avoid repeated or contradictory status text.

## Rollout Notes

- Scope the first integration to `kgh github run`.
- Keep existing workflow preflight summary messages until the renderer-based output is verified in CI.
- After the renderer lands, simplify workflow shell-written summary blocks if they overlap with the generated report.

## Implementation Order

1. Implement the pure Markdown renderer and unit tests.
2. Implement the GitHub summary writer wrapper and unit tests.
3. Wire summary generation into `cmd/kgh/main.go` for `github run`.
4. Adjust the workflow only if duplication becomes noisy.

## Issue Excerpt

The issue asks for a small renderer that converts `execution.Result` into GitHub Actions Summary Markdown and covers target, notebook path, kernel id, Kaggle run status, submit status, public score, and helpful identifiers, with explicit fallback states instead of blanks.
