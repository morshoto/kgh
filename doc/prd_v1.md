# PRD: `kgh` v1 — PR-native Kaggle Submit MVP

## 1. Overview

`kgh` is a GitHub-native developer tool for Kaggle workflows.

The v1 goal is to let a developer trigger a Kaggle notebook/kernel run from GitHub, optionally submit the resulting `submission.csv` to a Kaggle competition, and surface the result back into GitHub through Action Summary and PR comments.

The core product idea is simple:

* GitHub remains the source of truth for notebooks and experiment code
* Kaggle remains the execution environment for notebook runs and GPU-backed workloads
* Pull requests become the place where experiment outcomes are reviewed

In v1, `kgh` is intentionally narrow. It does not try to be a Kaggle platform, experiment tracker, or notebook synchronization suite. It focuses on one high-value loop:

1. select a target from GitHub
2. run it on Kaggle
3. optionally submit it
4. report the score and compare it to the base branch

---

## 2. Problem Statement

Teams and individuals who use both GitHub and Kaggle face a workflow gap.

Today, common workflows look like this:

* notebooks are edited in GitHub, but execution happens manually on Kaggle
* notebook code is versioned in GitHub, but submission outcomes live only in Kaggle UI
* experiment comparison is manual and detached from code review
* GitHub Actions can automate parts of the process, but existing tools operate at a low level and do not provide PR-native reporting or score comparison

This creates several problems:

* it is hard to review notebook changes together with their Kaggle outcome
* it is easy to lose the mapping between code revision and Kaggle submission
* comparing PR branch vs base branch performance is manual and error-prone
* existing Kaggle GitHub integrations are mostly wrappers around Kaggle primitives, not developer workflow tools

`kgh` v1 addresses this by turning a PR into a lightweight experiment review surface.

---

## 3. Goals

### Primary goals

`kgh` v1 must:

* allow a user to trigger a Kaggle run from GitHub
* resolve a specific target notebook/kernel from repository config
* generate Kaggle execution metadata dynamically
* execute the notebook/kernel on Kaggle
* download outputs from Kaggle
* optionally submit `submission.csv` to a configured competition
* retrieve the latest public score
* publish the result into GitHub Actions Summary
* publish or update a PR comment with the result
* compare the current result against the latest successful base branch result

### Secondary goals

`kgh` v1 should:

* be easy to adopt in a single repository
* avoid introducing any database or external backend
* be secure by default in GitHub Actions contexts
* be CLI-first so local debugging and testing are practical

---

## 4. Non-goals

The following are explicitly out of scope for v1:

* database-backed experiment history
* Weights & Biases integration
* multi-target execution in a single run
* broad dynamic override support for all Kaggle metadata fields
* full dataset switching from commit message overrides
* notebook content rewriting
* automatic notebook diff rendering
* support for arbitrary external contributor PR submit flows
* advanced leaderboard history management
* full synchronization between GitHub and Kaggle notebooks in both directions

---

## 5. Target Users

### Primary users

* Kaggle competitors who store notebook and experiment code in GitHub
* engineers and researchers who want PR-based review for Kaggle experiments
* users who rely on Kaggle compute, especially GPU-backed notebook execution

### Secondary users

* teams running shared Kaggle repos
* maintainers who want reproducible, reviewable notebook submission workflows

---

## 6. User Value

A successful v1 gives users the following value:

* one clear path from GitHub commit to Kaggle result
* PR-level visibility into experiment outcomes
* a safer and more reviewable alternative to manual Kaggle UI workflows
* a reproducible mapping from code revision to Kaggle run and competition submission
* lightweight baseline comparison without adding extra infrastructure

---

## 7. Product Principles

### 7.1 GitHub-first

GitHub is the source of truth for notebooks, configuration, and review.

### 7.2 Kaggle-native execution

`kgh` does not try to replace Kaggle compute. It orchestrates Kaggle execution and submissions.

### 7.3 Explicit over magical

`kgh` should not guess which notebook to run. Target selection must be explicit.

### 7.4 Low operational burden

v1 must not require a database, queue, or external service.

### 7.5 PR as experiment surface

The pull request is the main place where results should appear.

### 7.6 Secure by default

Fork PRs and missing-secrets cases must fail safely.

---

## 8. v1 User Experience

### 8.1 Main trigger model

The primary trigger is a commit message command:

```text
submit: <target>
```

Examples:

```text
submit: exp142
submit: exp142 gpu=false
submit: exp142 internet=true
submit: exp142 gpu=false internet=true
```

This is the main v1 UX because it is:

* easy to understand
* easy to audit in Git history
* lightweight compared with manual workflow inputs
* explicit about intent

### 8.2 Target resolution

Targets are defined in repository config under `.kgh/config.yaml`.

Example:

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
```

When `submit: exp142` is encountered, `kgh` resolves the target, merges supported runtime overrides, and builds the final execution spec.

### 8.3 Workflow dispatch

`workflow_dispatch` exists as a secondary path for re-runs and operational recovery. It is not the main UX.

### 8.4 PR label trigger

PR labels are not the primary v1 path. They may be used later as re-run helpers, but target ambiguity makes them a poor main trigger.

---

## 9. Functional Requirements

## 9.1 Configuration

`kgh` must support a repository-level config file at:

```text
.kgh/config.yaml
```

v1 config must support:

* target name
* notebook path
* Kaggle kernel id
* competition slug
* submit enabled or disabled
* GPU default
* internet default
* privacy default
* competition sources
* dataset sources
* expected outputs

`kgh` must validate config structure and fail with actionable errors on invalid configuration.

---

## 9.2 Commit message parsing

`kgh` must parse commit messages in the form:

```text
submit: <target> [gpu=<bool>] [internet=<bool>]
```

v1 parser requirements:

* must extract the target name
* must support `gpu` override
* must support `internet` override
* must reject malformed syntax clearly
* must not support multiple targets

---

## 9.3 Target resolution

`kgh` must:

* look up the referenced target in config
* merge allowed overrides
* produce a resolved execution spec
* fail fast on unknown targets
* avoid implicit guessing from changed files

Changed files may be used for validation or reporting, but not for target inference in v1.

---

## 9.4 Execution spec generation

`kgh` must generate Kaggle execution metadata dynamically for each run.

This includes building a valid `kernel-metadata.json` in a temporary working directory, derived from:

* target config
* runtime overrides
* notebook path
* Kaggle kernel id
* Kaggle sources and resource settings

The notebook itself must not be modified in place by v1.

---

## 9.5 Kaggle execution

`kgh` must:

* authenticate to Kaggle via GitHub secrets-backed credentials
* push the execution bundle to Kaggle
* poll Kaggle execution status until completion, failure, or timeout
* expose a structured run result for downstream reporting

---

## 9.6 Output retrieval

After a successful Kaggle execution, `kgh` must:

* download kernel outputs
* look for configured outputs such as:

  * `submission.csv`
  * `metrics.json`
* make these outputs available to later steps

If expected outputs are missing, `kgh` must report this clearly.

---

## 9.7 Competition submission

If the resolved target has `submit: true` and `submission.csv` exists, `kgh` must:

* submit the output to the configured Kaggle competition
* associate the submit step with the current run context
* return submission metadata for reporting

If `submit: false`, `kgh` must skip this step cleanly.

---

## 9.8 Score retrieval

After submission, `kgh` must:

* retrieve the latest relevant Kaggle submission result
* extract at minimum:

  * submission id
  * public score
  * submission status if available
  * timestamp if available

v1 only requires public score handling.

---

## 9.9 GitHub reporting

`kgh` must write:

### GitHub Actions Summary

Include:

* target name
* notebook path
* kernel id
* Kaggle run status
* submit status
* public score
* useful identifiers or references

### PR comment

`kgh` must create or update a single bot comment on the PR including:

* target name
* resolved configuration summary
* Kaggle run status
* submission result
* public score
* comparison vs base branch if available

The PR comment must be updated in place on re-runs rather than duplicated.

---

## 9.10 Baseline comparison

`kgh` must compare the current score to the latest successful result from the base branch.

v1 comparison requirements:

* use the latest successful base branch run as baseline
* compute delta between current public score and baseline public score
* show comparison in GitHub Summary and PR comment
* degrade gracefully if no baseline exists

v1 must not depend on a database to store history.

---

## 10. Technical Architecture

## 10.1 High-level architecture

`kgh` v1 will be a **Go CLI** with a **thin GitHub Action wrapper**.

This architecture is preferred because:

* business logic remains testable outside GitHub Actions
* local development and debugging are easier
* GitHub Action implementation remains thin
* future expansion is easier than shell-first designs

## 10.2 Architecture layers

### Layer 1: UX and policy layer

Handles:

* commit message parsing
* target resolution
* override merge
* safety decisions
* run intent determination

### Layer 2: execution spec layer

Handles:

* resolved runtime config
* generated Kaggle metadata
* temporary execution bundle construction

### Layer 3: adapter layer

Handles:

* Kaggle CLI invocation
* GitHub Summary writes
* PR comment updates
* baseline lookup

---

## 10.3 Proposed repo structure

```text
.kgh/
  config.yaml

cmd/kgh/
internal/
  config/
  parser/
  planner/
  spec/
  kaggle/
  github/
  compare/
```

---

## 10.4 Core command flow

A typical `kgh` CI run should follow this internal flow:

1. parse trigger input
2. load config
3. resolve target
4. generate execution bundle
5. push kernel to Kaggle
6. poll run status
7. download outputs
8. optionally submit to competition
9. retrieve score
10. compare with base branch
11. publish GitHub Summary
12. upsert PR comment

---

## 11. Security Requirements

Security is a core v1 requirement.

### 11.1 Secrets handling

`kgh` must use Kaggle credentials from GitHub Secrets or environment variables.

`kgh` must never log sensitive values in plain text.

### 11.2 Fork PR safety

Forked PRs must not perform real Kaggle submission in v1.

They must either:

* perform a dry-run of config and target resolution only, or
* fail safely with a clear explanation

### 11.3 Minimal permissions

GitHub permissions should be minimized. v1 should require only what is needed for:

* reading repository contents
* writing PR comments when appropriate

### 11.4 Safe failure

When credentials are missing, config is invalid, or execution cannot proceed, `kgh` must fail with a clear explanation rather than silently skipping critical behavior.

---

## 12. Success Metrics

v1 will be considered successful if the following are true in a real sample repo:

* a repository can define at least one target in `.kgh/config.yaml`
* a commit message `submit: <target>` triggers a Kaggle run
* a Kaggle kernel executes successfully
* outputs are downloaded successfully
* a competition submission can be made when configured
* a public score is retrieved
* GitHub Summary shows the result
* PR comment shows the result and a delta vs base when available
* unsafe contexts fail safely

---

## 13. Risks and Tradeoffs

## 13.1 Kaggle execution is asynchronous

Polling introduces latency and possible timeout complexity.

**Mitigation:** explicit timeout states and clear status reporting.

## 13.2 Kaggle account state may block execution

Users may not have joined a competition or accepted rules.

**Mitigation:** surface Kaggle-side failures clearly and document prerequisites.

## 13.3 Target ambiguity

A PR may change multiple notebooks or shared code paths.

**Mitigation:** require explicit target selection. Do not auto-guess in v1.

## 13.4 Secrets and public contribution workflows

Fork-based open source workflows do not safely support real submit behavior.

**Mitigation:** safe dry-run defaults for forks.

## 13.5 Baseline retrieval without a DB

Using latest successful base run is simple but limited.

**Mitigation:** accept this as a deliberate v1 simplification.

---

## 14. Milestones

### Phase 1 — Core plumbing

* bootstrap Go CLI project
* define config schema
* implement commit message parser
* implement target resolution
* generate Kaggle metadata

### Phase 2 — Kaggle execution

* Kaggle auth and CLI adapter
* kernel push
* status polling
* output download
* competition submit
* score retrieval

### Phase 3 — GitHub UX

* GitHub Summary reporting
* PR comment upsert
* baseline comparison
* GitHub Action wrapper

### Phase 4 — Polish and security

* secure defaults
* dry-run handling
* docs
* demo assets

---

## 15. Definition of Done

`kgh` v1 is done when:

* a user can define one or more targets in `.kgh/config.yaml`
* `submit: <target>` triggers a fully resolved Kaggle workflow
* a Kaggle run can execute successfully
* outputs can be fetched
* a competition submission can be made when configured
* public score is reported in GitHub
* PR comment shows current result and base comparison
* unsafe or missing-secret contexts fail safely
* the README is sufficient for first adoption

---

## 16. Future Work After v1

The following are logical post-v1 extensions:

* path fallback for target selection
* richer runtime overrides
* dataset override support
* competition profiles
* code competition-specific flows
* richer metrics rendering from `metrics.json`
* W&B integration
* persistent experiment history
* multi-target execution

---

## 17. One-line Product Statement

**`kgh` lets developers run Kaggle notebooks from GitHub and review submission results directly in pull requests.**
