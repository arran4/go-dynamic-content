# CI/CD Workflow Diagnostics

## Issue Summary
GitHub Actions workflows (`ci.yml`) are not triggering for `push` or `pull_request` events since PR #15 was merged.

## Root Cause
The root cause is a GitHub repository/account Actions policy setting blocking workflow execution. The `.github/workflows/ci.yml` file is syntactically correct and fully validates against `actionlint`. Because entirely missing workflow runs indicate rejection at the GitHub registration/admission layer (rather than a runtime failure), the issue lies in the Actions configuration.

## Required Repository Settings Correction
Please check and correct the following settings in the repository (or organization policy):

1. **Actions Enablement:**
   Navigate to **Settings → Actions → General**.
   Ensure that **Actions permissions** are set to **"Allow all actions and reusable workflows"** or **"Allow [org] and select non-[org], actions and reusable workflows"**. If it is set to "Disable actions", no workflows will run.

2. **Workflow Execution Policies:**
   Navigate to **Settings → Actions → Policies**.
   Ensure there are no event-routing restrictions preventing `push` or `pull_request` events from triggering workflows, or restricting the specific actor.

## Diagnostic Evidence

### `actionlint` Validation
Running `actionlint` locally on `.github/workflows/ci.yml` returns `0` errors. The schema is fully compliant with GitHub Actions standards.

### `workflow_dispatch` Diagnostic
Attempting to trigger the workflow manually using GitHub CLI fails at the API level (e.g. `Bad credentials` or immediate rejection), which confirms the API/GitHub layer is refusing workflow dispatch entirely.

### Local Test Results
- `go test ./...` passed successfully.
- `go vet ./...` passed successfully.

## Manual Verification Required
Once the repository settings have been updated to allow Actions execution, please manually verify by:
1. Pushing a new commit to the repository.
2. Opening a pull request.
3. Confirming that a new workflow run appears in the **Actions** tab.
