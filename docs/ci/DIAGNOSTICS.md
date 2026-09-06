# CI/CD Workflow Diagnostics

## Issue Summary
Historically, GitHub Actions workflows (`ci.yml`) stopped triggering for `push` or `pull_request` events, notably resulting in no runs for PR #15 or its merge.

## Current State & Evidence
The `ci.yml` workflow is currently **active** and **successfully executing**.
Opening PR #24 generated a successful `pull_request` run (**#105**, ID `34014644270`). This run completed all steps (Route event, capability discovery, golangci-lint, dependency review, `go vet`, `go test`, and Release Validation Gate) without requiring any changes to the workflow YAML file.

This establishes that:
1. **Workflow Syntax is Valid:** The schema is fully compliant with GitHub Actions standards (`actionlint` returns 0 errors).
2. **Execution is Permitted:** There are no repository or account-level policy blockers currently preventing `pull_request` events from triggering this workflow.

## Historical Absence of Runs
The chronological evidence is:
* **2026-04-08:** The last `push` and `pull_request` runs before this incident occurred.
* **2026-06-08:** The last `schedule` run occurred.
* **2026-09-05:** PR #15 was created and merged, but no workflow run was triggered.
* **2026-09-06:** PR #24 was created and successfully triggered run #105.

Since more than 60 days of repository inactivity elapsed between June and September, it is highly probable that the workflow was automatically disabled by GitHub (`disabled_inactivity` state) prior to PR #15. While we cannot definitively prove the exact moment of reactivation (whether manual or automated), the workflow is now fully active.

## Inconclusive Diagnostic Tests
Attempting to trigger the workflow manually using `workflow_dispatch` via the API resulted in a `401 Bad credentials` error. This was an authentication failure due to the diagnostic environment lacking a valid `GITHUB_TOKEN`, and is not evidence of a workflow admission policy or registration issue.

## Local Test Results
- `actionlint .github/workflows/ci.yml` passed with 0 errors.
- `go test ./...` passed successfully.
- `go vet ./...` passed successfully.

## Verification Required
* PR-triggered CI is now demonstrably working (Run #105).
* Verification of `push` event routing to the `main` branch is still required after this diagnostic PR is merged or closed, to completely resolve the acceptance criteria of the original issue.
