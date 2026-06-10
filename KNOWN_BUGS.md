# Known Bugs — USDA JobPortal

_Last updated: 2026-06-08_

## Open

- No open critical or high-priority functional bugs are currently tracked in this file.

---

## Resolved (2026-05-22)

### Original backlog (BUG-001 – BUG-012)

| ID | Issue | Resolution |
|----|-------|------------|
| BUG-001 | Typing indicators didn't work (reused the new-message signal) | Separate SSE `typing` event + EventSource in the chat drawer; `handleTyping` now emits a distinct signal |
| BUG-002 | Read receipts showed "Read" incorrectly (epoch sentinel + strict comparison) | `Year() <= 1970` epoch guard + inclusive `!t.After()` comparison (`isRead` helper) |
| BUG-003 | Bookmarks had no UI buttons | Bookmark/Save buttons added to feed, posting, and article views |
| BUG-004 | File attachments not displayed in feed | `getSocialFeed` loads attachments; `feed.html` renders them |
| BUG-005 | Share/repost had no commentary input | Commentary textarea added to the share form |
| BUG-006 | Badges never awarded (`CheckAndAward` never called) | Wired into post create, connection accept, profile update, kudos received; added the missing **Detail Completed** and **Mentor** checks + their triggers |
| BUG-007 | Skill verification had no UI | "Verify" button + verification counts on profile skills |
| BUG-008 | Pinned contacts not wired | Pin/unpin button on conversation items in the chat drawer |
| BUG-009 | Reactions were text-only | Emoji reactions (👍 Like, 🎉 Celebrate, 💡 Insightful, 🤔 Curious) with counts |
| BUG-010 | Employee Spotlight not implemented | New `internal/spotlight` package: admin create form + public `/spotlight` page |
| BUG-011 | Birthday/work anniversary not wired | Date fields on profile edit; profile display; "celebrations this week" widget |
| BUG-012 | Pinned profile posts not implemented | Pin/unpin routes + "📌 Pinned" section on the profile |

### Found and fixed this session

| Issue | Resolution |
|-------|------------|
| Settings preference endpoints accepted protocol-relative redirect targets (open redirect risk) | `cmd/server/main.go` now uses `middleware.SafeRedirectTarget(...)` for both matching-mode and location-type setting redirects, aligning with other hardened handlers |
| Same-origin middleware allowed unsafe requests with missing `Origin`/`Referer` in some configurations | `internal/middleware/security.go` now blocks all unsafe requests missing both headers, and tests were updated to enforce strict behavior |
| Saved post links in Bookmarks did not reliably deep-link to the saved post | Bookmarked post URLs now include both `highlight` and `#post-{id}` anchors so users land on the exact post context |
| Social composer gave no feedback when a file was selected | Added live attachment filename feedback in the composer (`#attachment-selected`) with progressive enhancement in `static/js/feed.js` |
| Submitting a comment on feed posts caused full-page refreshes | Added asynchronous comment submission in `static/js/feed.js` with per-post incremental refresh (`updateSinglePost`) |
| Managers needed extra navigation steps to reach applicants | Added direct "View Applicants" shortcuts on posting cards in feed/search and on My Posts listings |
| A 1:1 message was routed into an existing **group** conversation that included the recipient | `findDirectConversation` now restricts the lookup to conversations with exactly two participants; fixed in both `startConversation` and `startGroupConversation` |
| No DB-level guard against duplicate 1:1 conversations | Migration `048`: canonical `conversations.direct_key` + partial unique index; create paths set it, with graceful race recovery |
| Dropdown controls across admin/posting/org chart/leaderboard forms were inconsistent (hard to open, value changes not reflected, accidental refreshes) | Standardized explicit submit + dirty-state behavior, removed auto-submit dropdowns, added strict Playwright coverage that verifies select values actually change on option selection |
| "Join now" auth link appeared unstyled | Added explicit auth switch link styling in shared CSS so login/register cross-link is visually consistent and accessible |
| Acceptable Use Policy page did not follow app theming | Removed inline hardcoded colors and migrated AUP styles to shared themed CSS using standard color tokens and card spacing |
| Cross-theme control contrast was inconsistent on key pages (dark primary buttons too light, feed reaction controls too dim, dashboard cards using non-themed fallbacks) | Tuned dark-mode primary tokens for accessible contrast, added dark-mode feed reaction overrides, switched admin dashboard cards/panels to shared theme tokens, and added a Playwright light/dark contrast regression suite |
| User deletion could record a `user_delete` audit entry even when no row was actually deleted | Reworked delete to `DELETE ... RETURNING` in a transaction, only audit after confirmed deletion, and added regression tests for both success and not-found paths |
| Admin Dashboard could fail to show meaningful recent audit entries when actor/details fields were blank-ish | Hardened dashboard/audit queries with trimmed fallbacks (`System`, `(no details)`, `Unknown user`) and added regression coverage for the exact SQL fallback contract |
| Org Chart pages did not follow shared theming (inline hardcoded style blocks/attributes) | Moved org chart styles to shared `static/css/orgchart.css`, imported it via `main.css`, removed inline template styles, and added browser regression checks that org chart pages no longer embed inline style blocks |
| Org Chart was presented as a plain scroll-style list rather than a graph-like hierarchy | Updated org chart markup and styling to a connector-based graph presentation (`orgchart-graph`/`orgchart-children`) with responsive fallback behavior, plus browser regression coverage |
| All tools and messages controls lived in the top nav instead of floating side tabs | Converted tools/messages toggles to fixed side tabs with icon labels (`☰`, `💬`) anchored left/right and validated behavior with browser regression tests |
| Skills Heat Map page did not follow shared theming | Removed inline style block, added shared themed styles in `static/css/insights.css`, introduced `.heatmap-layout`, and enabled fixture rendering/tests for `/insights/heatmap` |
| Workforce Analytics dashboard cards did not follow shared theming | Removed inline style block/attributes and migrated workforce styles to `static/css/analytics.css` using shared theme tokens |
| Feed reaction buttons triggered full-page refreshes | Reactions now submit asynchronously in `static/js/feed.js`, updating only the touched post; added Playwright regression coverage for no-reload behavior |
| Share commentary panel was too small and opening it disrupted Save/Report layout | Converted share form to anchored panel with larger textarea and stable action-row alignment in `static/css/feed.css`; added Playwright geometry/layout regression coverage |
| Social drafts lacked clear post-type context (Social Feed vs Project/Detail) | Updated `templates/feed/drafts.html` with explicit Social Feed labeling and added direct links to create/manage Project/Detail postings |
| Social scheduling lived on feed instead of drafts, and drafts editor was too small/plain | Removed scheduling controls from `templates/feed/feed.html`, moved scheduling into drafts (`/feed/drafts/schedule`), and upgraded drafts inputs to larger rich editors backed by safe markdown rendering in feed (`internal/feed/handler.go`) |
