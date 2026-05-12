# Milestone

**Milestone:** `v1: PR-native Kaggle submit MVP`

**Goal:**
Enable a GitHub-driven Kaggle workflow where a commit message like `submit: <target>` triggers a Kaggle kernel run, optionally submits to a competition, and reports results back to GitHub via Action Summary and PR comments.

---

# Issue 1 — Bootstrap project skeleton

**Title**
Bootstrap `kgh` project skeleton

**Body**

```md
## Summary
Create the initial repository structure for `kgh` as a Go-based CLI plus a thin GitHub Action wrapper.

## Scope
- Create Go module
- Add `cmd/kgh`
- Add internal packages:
  - `internal/config`
  - `internal/parser`
  - `internal/spec`
  - `internal/kaggle`
  - `internal/github`
  - `internal/compare`
- Add basic CI for build and test
- Add README placeholder
- Add `.gitignore`, Makefile, and development scripts if needed

## Why
We want `kgh` to be a CLI-first architecture, not just a shell-based GitHub Action. This will make local debugging, testing, and future extensibility much easier.

## Acceptance Criteria
- `go test ./...` runs successfully
- `go build ./cmd/kgh` succeeds
- Repository structure is in place
- README includes a short project description

## Notes
Keep the initial CLI minimal. A placeholder `kgh version` command is enough for now.
```

---

# Issue 2 — Define config schema and validation

**Title**
Define `.kgh/config.yaml` schema and validation

**Body**

````md
## Summary
Implement the v1 config schema for `.kgh/config.yaml` and validate it.

## Scope
Support a config structure like:

```yaml
targets:
  exp142:
    notebook: notebooks/exp142.ipynb
    kernel_id: yourname/exp142
    competition: playground-series-s6e2
    submit: true
    resources:
      gpu: true
      internet: false
      private: true
    sources:
      competition_sources:
        - playground-series-s6e2
      dataset_sources:
        - yourname/feature-pack-v3
    outputs:
      submission: submission.csv
      metrics: metrics.json
````

## Why

We need a stable, explicit config layer so users do not have to encode all Kaggle execution settings in commit messages or workflow inputs.

## Acceptance Criteria

* `.kgh/config.yaml` can be loaded into typed Go structs
* Invalid configs produce actionable error messages
* Missing required fields are detected
* A sample config file is added to the repo

## Notes

v1 should support only the fields needed for the MVP.
Do not over-design future overrides yet.

---

# Issue 3 — Implement commit message parser

**Title**  
Implement commit message parser for `submit: <target>`

**Body**
```md
## Summary
Parse commit messages that trigger `kgh` runs.

## Scope
Support the following v1 patterns:

- `submit: exp142`
- `submit: exp142 gpu=false`
- `submit: exp142 internet=true`
- `submit: exp142 gpu=false internet=true`

## Why
The main UX for v1 is commit-message-driven execution. This must be easy to understand, deterministic, and fail clearly on invalid syntax.

## Acceptance Criteria
- A parser extracts:
  - command (`submit`)
  - target (`exp142`)
  - optional overrides (`gpu`, `internet`)
- Invalid syntax returns clear error messages
- Unit tests cover valid and invalid cases

## Notes
Do not support multiple targets in v1.
Do not support arbitrary `--set` overrides in v1.
````

---

# Issue 4 — Resolve target and merge overrides

**Title**
Implement target resolution and runtime override merge

**Body**

```md
## Summary
Resolve a target from commit message + config and produce a final execution plan.

## Scope
- Look up target in `.kgh/config.yaml`
- Merge runtime overrides from parsed commit message
- Produce a resolved execution spec
- Fail fast on unknown targets

## Why
We need a clear separation between:
- target defaults from config
- temporary runtime overrides from the triggering commit

## Acceptance Criteria
- `submit: exp142` resolves to a complete target spec
- `gpu` and `internet` overrides are merged correctly
- Unknown target names fail with a clear message
- Resolved output can be printed as structured data for debugging

## Notes
This layer should not call Kaggle or GitHub APIs yet.
Keep it pure and testable.
```

---

# Issue 5 — Generate Kaggle kernel metadata dynamically

**Title**
Generate `kernel-metadata.json` dynamically from resolved target spec

**Body**

```md
## Summary
Render a valid Kaggle `kernel-metadata.json` from the resolved execution spec.

## Scope
Generate fields needed for v1, including:
- title
- id / kernel_id
- code_file
- language / kernel_type if needed
- enable_gpu
- enable_internet
- competition_sources
- dataset_sources
- kernel_sources
- model_sources
- is_private if applicable

## Why
We want `kgh` to treat notebook execution settings as generated metadata, not as something manually edited or hardcoded per run.

## Acceptance Criteria
- A valid `kernel-metadata.json` is produced for a resolved target
- Output JSON is deterministic
- Metadata reflects config + overrides correctly
- Tests verify generated fields

## Notes
This should write into a temporary working directory used for Kaggle push.
```

---

# Issue 6 — Implement Kaggle auth and CLI adapter

**Title**
Implement Kaggle CLI adapter and authentication handling

**Body**

```md
## Summary
Create the adapter layer that safely invokes the Kaggle CLI from `kgh`.

## Scope
- Detect or configure Kaggle credentials from environment
- Wrap execution of Kaggle CLI commands
- Capture stdout/stderr
- Handle failures cleanly
- Add timeouts where appropriate

## Why
`kgh` will orchestrate Kaggle through the official CLI rather than reimplementing the API. This layer should be thin, safe, and testable.

## Acceptance Criteria
- `kgh` can invoke Kaggle CLI commands reliably
- Missing credentials produce a clear error
- Command failures surface useful debugging information
- CLI wrapper can be mocked in tests

## Notes
Do not add submit logic here yet.
This issue is just the adapter foundation.
```

---

# Issue 7 — Push kernel and poll for execution status

**Title**
Implement Kaggle kernel push and execution status polling

**Body**

```md
## Summary
Push a generated kernel bundle to Kaggle and wait for execution to complete.

## Scope
- Push kernel to Kaggle
- Poll kernel status until terminal state
- Handle success, failure, and timeout
- Return structured execution status to caller

## Why
This is the core of the MVP: GitHub triggers execution, but compute happens on Kaggle.

## Acceptance Criteria
- A kernel can be pushed successfully
- Status polling reaches a terminal result
- Timeout behavior is configurable and explicit
- Failures are reported clearly

## Notes
Do not handle output downloads yet in this issue.
Focus only on push + status lifecycle.
```

---

# Issue 8 — Download kernel outputs

**Title**
Download Kaggle kernel outputs after successful execution

**Body**

```md
## Summary
Retrieve kernel outputs from Kaggle after the run completes.

## Scope
- Download outputs into a temporary directory
- Detect expected files such as:
  - `submission.csv`
  - `metrics.json`
- Expose output paths to downstream steps

## Why
We need the output artifacts to support competition submission and GitHub reporting.

## Acceptance Criteria
- Outputs can be downloaded after a successful run
- Missing expected files are handled explicitly
- Returned output paths are structured and testable

## Notes
Do not submit to competitions yet in this issue.
This issue is about artifact retrieval only.
```

---

# Issue 9 — Submit competition output to Kaggle

**Title**
Submit `submission.csv` to Kaggle competition when configured

**Body**

```md
## Summary
If a target is configured with `submit: true` and a `submission.csv` exists, submit it to the configured Kaggle competition.

## Scope
- Detect `submission.csv`
- Build a submission message
- Submit to the target competition
- Support skip behavior when submit is disabled

## Why
Kernel execution alone is not enough for the intended workflow. We need to turn successful Kaggle outputs into actual competition submissions.

## Acceptance Criteria
- `submission.csv` is submitted successfully when present
- Submit is skipped cleanly when disabled
- Missing submission file is handled explicitly
- Submission metadata is returned for later reporting

## Notes
Submission message can be simple in v1, e.g. target name + commit SHA.
```

---

# Issue 10 — Retrieve latest submission score

**Title**
Retrieve latest Kaggle submission score and metadata

**Body**

```md
## Summary
Fetch the latest competition submission details after submit.

## Scope
- Read latest submission from Kaggle
- Extract:
  - submission id
  - public score
  - timestamp / status if available
- Return structured result

## Why
This is required for GitHub Summary, PR comment reporting, and baseline comparison.

## Acceptance Criteria
- Latest submission can be fetched after submit
- Public score is parsed correctly
- Result structure is suitable for reporting
- Failure modes are explicit

## Notes
v1 only needs public score handling.
No leaderboard history persistence yet.
```

---

# Issue 11 — Add GitHub Actions Summary reporting

**Title**
Add GitHub Actions Summary reporting for run result

**Body**

```md
## Summary
Write a clear execution summary to the GitHub Actions job summary.

## Scope
Include:
- target name
- notebook path
- kernel id
- kernel execution status
- submit status
- public score
- links or identifiers where helpful

## Why
The workflow run page should be useful even without opening PR comments.

## Acceptance Criteria
- Summary is written on successful runs
- Summary is written on failed runs with useful context
- Output is readable and compact

## Notes
Design the summary format so it can be reused in docs/screenshots later.
```

---

# Issue 12 — Add PR comment upsert reporting

**Title**
Add PR comment upsert for Kaggle run results

**Body**

```md
## Summary
Post or update a single bot comment on the PR with the current `kgh` result.

## Scope
- Detect existing `kgh` bot comment
- Update it instead of creating duplicates
- Include:
  - target
  - kernel status
  - submit status
  - public score
  - comparison section placeholder if available

## Why
PR comment reporting is the core user-facing surface of `kgh`.

## Acceptance Criteria
- A PR gets exactly one `kgh` result comment
- Re-runs update the same comment
- Comment format is readable and compact

## Notes
If no PR context is available, skip gracefully.
```

---

# Issue 13 — Compare current score vs base branch baseline

**Title**
Implement score comparison against latest successful base branch run

**Body**

```md
## Summary
Compare the current run's score against a baseline from the base branch.

## Scope
- Retrieve the latest successful baseline result from base branch
- Compare public score
- Compute delta
- Return structured comparison result

## Why
This is a key product differentiator: making PRs the place where Kaggle experiment improvements are reviewed.

## Acceptance Criteria
- Current run can be compared to base branch when baseline exists
- Missing baseline is handled gracefully
- Delta is included in summary/report payloads

## Notes
For v1, use the latest successful run on base branch.
Do not add DB-backed history.
```

---

# Issue 14 — Build thin GitHub Action wrapper

**Title**
Create thin GitHub Action wrapper around `kgh` CLI

**Body**

```md
## Summary
Package `kgh` for GitHub Actions use with a thin wrapper around the CLI.

## Scope
- Define Action metadata
- Invoke `kgh` from Action
- Support minimal inputs if needed
- Expose outputs where useful
- Add example workflow YAML

## Why
The product should be easy to adopt in GitHub Actions without forcing users to wire the CLI manually.

## Acceptance Criteria
- Action can be used from a workflow in a sample repo
- Wrapper remains thin and delegates logic to the CLI
- Example workflow is documented

## Notes
Prefer keeping most logic in Go rather than in YAML or shell scripts.
```

---

# Issue 15 — Harden security defaults

**Title**
Harden security defaults for secrets, permissions, and PR execution

**Body**

```md
## Summary
Ensure `kgh` behaves safely by default in GitHub environments.

## Scope
- Minimize GitHub permissions
- Prevent real submit on forked PRs
- Fail or dry-run safely when secrets are unavailable
- Avoid leaking sensitive values in logs
- Provide clear operator-facing error messages

## Why
`kgh` interacts with Kaggle credentials and PR-triggered automation. Secure defaults are required for trust and adoption.

## Acceptance Criteria
- Fork PRs do not perform real submit
- Missing secrets are handled safely
- Logs do not expose credentials
- Default recommended permissions are documented

## Notes
This issue should include documentation updates, not just code changes.
```

---

# Issue 16 — Write README, quickstart, and demo assets

**Title**
Write v1 README, quickstart, and demo assets

**Body**

```md
## Summary
Document `kgh` clearly enough that users can adopt it from the README alone.

## Scope
- Project overview
- Quickstart
- Example `.kgh/config.yaml`
- Example workflow YAML
- Example commit message trigger
- Example PR comment / summary output
- Demo GIF or screenshots if possible

## Why
For this kind of developer tool, documentation is product surface area.

## Acceptance Criteria
- README explains what `kgh` does in one screen
- Quickstart is copy-pasteable
- Example config and workflow are included
- Outputs are shown visually

## Notes
The README should make the value proposition obvious:
“Run Kaggle kernels from GitHub PRs and compare scores automatically.”
```
