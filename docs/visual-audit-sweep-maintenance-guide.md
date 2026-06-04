# Visual Audit Sweep Maintenance Guide

## Audience
New engineers (including interns) maintaining:

- `cmd/bots/tests/test_visual_audit_sweep.py`
- `cmd/bots/tests/browser_harness.py`
- `scripts/run-playwright-tests.sh`
- `scripts/playwright_progress_runner.py`

---

## 1) What this test system does

This suite performs a **visual + interaction audit** of the Job Portal UI by:

1. Discovering routes
2. Capturing light/dark screenshots
3. Exercising interactable UI elements
4. Running auth checks (invalid + valid login)
5. Optionally running role-duty audits (admin/manager/employee if creds are provided)
6. Writing a triage report at:
   - `.copilot-artifacts/visual-audit/triage-report.md`

---

## 2) Key architecture (how it is organized)

### Main test file
`cmd/bots/tests/test_visual_audit_sweep.py`

Important components:

- **Progress protocol**
  - `PROGRESS done=... total=... status=...`
  - `PROGRESS_RESULT outcome=pass|fail|error|skip status=...`
- **Route discovery**
  - `_discover_get_routes()` parses `cmd/bots/testserver/main.go`
  - Adds specific known routes for live/fixture mode
- **Interaction engine**
  - `_collect_interaction_items(...)`
  - `_exercise_interaction_item(...)`
- **Dark mode validation**
  - `_set_theme(...)`
  - `_record_if_not_dark_delta(...)`
- **Artifacts and reporting**
  - `_screenshot_dir()`
  - `_write_triage_report(...)`
- **Dynamic unittest generation**
  - `_install_generated_interaction_tests()`
  - `load_tests(...)` controls which tests are actually loaded

### Harness
`cmd/bots/tests/browser_harness.py`

- Starts browser
- Handles login helpers
- Handles console/page error assertions
- Supports live mode (`PLAYWRIGHT_BASE_URL`)

### Runner scripts
- `scripts/run-playwright-tests.sh`  
  Wrapper and CLI flags
- `scripts/playwright_progress_runner.py`  
  Parses `PROGRESS`/`PROGRESS_RESULT` and renders progress bar

---

## 3) How tests are discovered (critical)

Even though several test methods exist, **`load_tests(...)` is authoritative**.

Current behavior:
- Always includes generated:
  - `test_visual_site_sweep_auth_*`
  - `test_visual_site_sweep_interaction_*`
- Includes role duty methods only if role creds are provided:
  - `test_visual_site_sweep_admin_duties`
  - `test_visual_site_sweep_manager_duties`
  - `test_visual_site_sweep_employee_duties`
- Excludes methods not in allowlist (including monolithic fallback methods unless explicitly allowed)

If you change test names, update `load_tests(...)`.

---

## 4) Running the suite

From `scripts/` directory (as you have been doing):

### Live app run (recommended)
`./run-playwright-tests.sh --headed --base-url http://127.0.0.1:8080 --no-reset --login-email '<email>' --login-password '<password>' -- -k visual_site_sweep`

### Add admin duty coverage
`./run-playwright-tests.sh --headed --base-url http://127.0.0.1:8080 --no-reset --login-email '<email>' --login-password '<password>' --admin-login-email '<admin-email>' --admin-login-password '<admin-password>' -- -k visual_site_sweep`

### Clean artifacts before run
`rm -f ../.copilot-artifacts/visual-audit/*.png ../.copilot-artifacts/visual-audit/triage-report.md`

---

## 5) Credentials model

- Base creds:
  - `PLAYWRIGHT_LOGIN_EMAIL`
  - `PLAYWRIGHT_LOGIN_PASSWORD`
- Role creds:
  - `PLAYWRIGHT_ADMIN_LOGIN_EMAIL/PASSWORD`
  - `PLAYWRIGHT_MANAGER_LOGIN_EMAIL/PASSWORD`
  - `PLAYWRIGHT_EMPLOYEE_LOGIN_EMAIL/PASSWORD`

Role-duty tests are included only when role creds exist.

---

## 6) Artifact model

Output dir:
- `.copilot-artifacts/visual-audit/`

Key files:
- `*-light.png`, `*-dark.png`
- `*-interaction-*.png` action/fallback captures
- `auth-login-failure.png`
- `auth-login-success.png`
- `triage-report.md`

Important:
- `/` route slug is `splash`
- Splash is captured in a guest context (`_capture_public_splash_pair`)

---

## 7) Dark mode enforcement rules

Dark mode is not filename-only. The script checks:

1. `dark-mode` class state
2. Computed style signature differences (`bodyBg`, `bodyText`, CSS vars)

If light and dark are visually identical by style signature, a finding is recorded:
- `Dark mode gap: ... did not show measurable style delta ...`

---

## 8) Interaction semantics

For each discovered interactable, script tries to act based on element type:

- anchors: click + navigation expectation
- inputs/textareas/selects: fill/select
- submit buttons: prefill required fields then submit
- generic buttons/summary/role=button: click

It records findings for:
- expected reload missing
- unexpected reload present
- interaction exceptions
- missing route artifacts

---

## 9) How to safely add new coverage

### Add a new route
1. Prefer adding route in app/testserver source so discovery picks it up
2. If needed, add explicit inclusion in `_discover_get_routes()`
3. Run and confirm light/dark + interaction captures exist

### Add a new interactable type
1. Extend `_INTERACTION_SELECTOR`
2. Add behavior in `_exercise_interaction_item(...)`
3. Ensure both action and fallback screenshots are possible

### Add role duty checks
1. Update `_role_duty_matrix()`
2. Use stable selectors (avoid brittle text-only selectors)
3. Confirm role credential gates still work via `load_tests(...)`

---

## 10) Common failure modes and fixes

### A) `Ran 1 test` unexpectedly
- Cause: wrong discovery path or stale loader wiring
- Check:
  - `load_tests(...)` in `test_visual_audit_sweep.py`
  - forwarding wrapper in `tests/test_visual_audit_sweep.py` (if used)

### B) `_FailedTest` / `no such test method`
- Cause: generating a probe with invalid method name
- Ensure probe uses `methodName='runTest'`

### C) Progress denominator jumps/reset
- Cause: totals discovered late or per-segment reset logic
- Verify progress runner is using fixed planned totals and cumulative outcomes

### D) Dark screenshot looks light
- Verify `_set_theme('dark')` is called before capture
- Verify dark class + localStorage keys match app behavior

### E) Too few screenshots
- Run may be interrupted early
- Confirm test process completed and report was written

---

## 11) Minimal maintenance checklist (before PR)

1. `python3 -m py_compile cmd/bots/tests/test_visual_audit_sweep.py`
2. `python3 -m py_compile scripts/playwright_progress_runner.py`
3. `bash -n scripts/run-playwright-tests.sh`
4. Run one local sweep
5. Confirm:
   - `triage-report.md` generated
   - splash light/dark both present
   - dark-mode deltas not silently broken
   - no unexpected discovery regressions

---

## 12) Handoff notes for future maintainers

- Treat `load_tests(...)` as production-critical; it controls what actually runs.
- Avoid giant methods. Keep helper functions small and single-purpose.
- Preserve artifact naming consistency; external reviewers rely on it.
- Do not silently swallow errors that should become findings.
- If behavior is intentionally skipped, record explicit reason in findings/report.

---
