# Known Bugs — USDA JobPortal

_Last updated: 2026-05-22_

## Open

**None.** All previously tracked bugs are resolved (see below). File new issues here as they're found.

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
| A 1:1 message was routed into an existing **group** conversation that included the recipient | `findDirectConversation` now restricts the lookup to conversations with exactly two participants; fixed in both `startConversation` and `startGroupConversation` |
| No DB-level guard against duplicate 1:1 conversations | Migration `048`: canonical `conversations.direct_key` + partial unique index; create paths set it, with graceful race recovery |
