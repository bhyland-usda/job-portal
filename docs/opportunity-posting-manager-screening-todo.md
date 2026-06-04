# Opportunity Posting and Manager Screening TODO

## Scope
Implement missing fields and workflows for opportunity posting, application responses, and manager screening actions.

## Naming Convention
- [ ] Use opportunity language in all user-facing copy (UI text, page titles, labels, buttons).
- [ ] Keep current internal names initially (tables like postings, posting_applications; package path internal/posting) to avoid risky broad renames.
- [ ] Introduce compatibility routes if desired (for example, /opportunities/... alongside existing /postings/...) before any route migration.
- [ ] Plan a separate technical-debt pass for internal renames only after feature completion and test coverage is green.

## 1) Database and Domain Model
- [ ] Add posting fields:
  - [ ] start_date (DATE)
  - [ ] duration_days (INTEGER, positive)
  - [ ] reporting_manager_name (VARCHAR)
  - [ ] reporting_manager_email (VARCHAR)
  - [ ] location_type (VARCHAR enum: remote, hybrid, onsite)
  - [ ] application_close_date (DATE)
  - [ ] number_of_people (INTEGER, positive)
  - [ ] learning_outcomes (TEXT)
- [ ] Add application fields:
  - [ ] selection_why (TEXT)
  - [ ] project_tackle_approach (TEXT)
- [ ] Add constraints and indexes:
  - [ ] posting date sanity checks
  - [ ] status and close-date index support

## 2) Opportunity Posting Form
- [ ] Add the new fields to posting create form UI
- [ ] Validate all new required values server-side
- [ ] Persist the new posting fields in create handler
- [ ] Render the new posting metadata on posting detail page

## 3) Application Form
- [ ] Replace/augment current cover letter prompt with:
  - [ ] Why should you be selected?
  - [ ] Given what you know about the project, how would you tackle this issue?
- [ ] Validate both answers server-side
- [ ] Store both answers with each application
- [ ] Show both answers in manager applications review page

## 4) Manager Screening Form/Actions
- [ ] Keep close opportunity action available for author/admin
- [ ] Add edit opportunity flow for manager/author:
  - [ ] GET edit page
  - [ ] POST update handler
  - [ ] Prefilled values and validation
- [ ] Add candidate decision controls in applications page:
  - [ ] shortlist
  - [ ] accept
  - [ ] reject
- [ ] Preserve access rules (author or admin only for screening)

## 5) Routing, Templates, and Styling
- [ ] Register edit routes
- [ ] Add posting edit template
- [ ] Update create/apply/applications/view templates to include new fields
- [ ] Extend postings stylesheet for new form groups and answer blocks

## 6) Testing
- [ ] Add migration tests or verification steps
- [ ] Add handler tests for new validation and permissions
- [ ] Add template visibility tests for edit/close/screening controls
- [ ] Run go test ./...

## 7) Rollout and QA
- [ ] Backfill defaults for existing rows where needed
- [ ] Verify old postings still render correctly
- [ ] Verify application status transitions and acceptance flow
- [ ] Verify mobile form layout for create/apply/edit pages
