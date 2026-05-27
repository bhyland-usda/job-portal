# Known Bugs — USDA JobPortal

_Last updated: 2026-05-22_

## Open

- Join Now link is unstyled
- User Acceptance Policy needs to follow correct theming
- Admin Dashboard needs to follow correct themeing
- Admin User Management Screen
  - Dropdowns are hard to open (multiple clicks needed)
  - When Set button is clicked for department, the page refreshes but the new value isn't persisted
  - When Actions dropdown is finally opened (hard to open) clicking a value doesn't update the value to trigger the user to click the Update button
  - Clicking update to update a user's permissions doesn't do anything.
  - Needs a back to dashboard button.
- Audit log catches changes, but the changes were not effective (see above bugs)
- Audit log has entries, but Admin Dashboard doesn't show anything in the Recent Audit Activity
- Assign Managers dropdowns are hard to open
- Org Chart theme needs to follow correct themeing
- Org Chart should be more of a hierarchal graph than a scrollable list
- All tools needs to be a floating tab on the lefthand side with a hamburger menu icon and moved off of the top of the page
- Messages needs to be a floating tab on the righthand side of the page with a Chat Bubble icon and moved off the top of the page
- Skills Heat Map needs to follow the correct themeing
- Workforce Analytics dashboard cards need to follow the correct themeing
- Feed Page
  - Clicking a emoji button refreshes the page, it should just update the button and likes.
  - The share commentary text box is too small, height and width
  - When share commentary box is open the Save and Report buttons have their layout disrupted.
  - Clicking share doesn't really have a function in the first place.
  - Saving a social post, and then clicking on it in the Saved Page just takes the user back to the feed page, not the post itself.
  - Attaching a file when writing a new social post doesn't show anything to say it's been attached successfully. There should be the files name or a small preview.
  - Adding a comment on a post refreshes the page, it should just add the comment and only refresh that post's comment section.
- Notifications are not sent to the manager when someone applies to their post.
- Too many clicks for managers to get to the applicants of their postings.
- Change status dropdown needs to follow the correct themeing.
- Change status does not update the status when an option is chosen from the dropdown.
- Change status dropdown has a hard time opening (can be seen in rendering flashes on the dropdown itself), but does open on first click.
- Record Outcome page for a posting:
  - Dropdown is hard to open (same as other dropdowns)
  - Clicking a selection on the Outcome Status does not update the value.
  - When posting has been closed the Record Outcome button is still active (maybe switch it to update outcome?)
- Postings need to have a workflow of when applicants can no longer apply without closing it.
- Social Post drafts text box is too small and the Save Draft button should be under it on the right hand side.
- Social Post drafts should have the same scheduling mechanism as posting right now on the Social Feed. In fact, it should be removed from the Social Feed page and placed in drafts.
- Manager Create Post page, same issue with the dropdown as every other page, hard to open and selecting something does not update the value.
- Create Post page dropdown has no keyboard shortcut to open it. There needs to be a keyboard workflow for EVERY page.
- Opportunities/Browse Postings Type filter dropdown has the same issue as all the other dropdowns tested.

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
