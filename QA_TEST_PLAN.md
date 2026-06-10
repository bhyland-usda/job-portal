# USDA JobPortal -- QA Manual Test Plan

**Application:** USDA JobPortal
**Version:** 1.0
**Date:** 2026-03-17
**Base URL:** http://localhost:8080 (or Docker-assigned port)

---

## Test Environment Setup

Before running tests, ensure:

1. Docker containers are running (`docker compose up`)
2. Database migrations have completed (check server logs for "server starting")
3. Seed admin account exists: `admin@jobportal.local` / `changeme123`
4. At least two browser windows available (for real-time SSE tests, use separate browser profiles or incognito)

### Test Accounts to Create

| Account | Email | Password | Role |
|---------|-------|----------|------|
| Admin | admin@jobportal.local | changeme123 | admin (seeded) |
| User A | usera@test.local | password123 | employee |
| User B | userb@test.local | password123 | employee |
| Manager | manager@test.local | password123 | manager (set via admin) |

---

## 1. Landing Page

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| LAND-001 | Guest sees landing page | Not logged in | 1. Navigate to `/` | Landing page renders with hero section, "Connect. Collaborate. Grow." heading, feature cards, and CTA section | [ ] |
| LAND-002 | Sign In button works | Not logged in | 1. On landing page, click "Sign In" button in header | Redirected to `/login` | [ ] |
| LAND-003 | Join Now button works | Not logged in | 1. On landing page, click "Join Now" button in header | Redirected to `/register` | [ ] |
| LAND-004 | Hero Sign In link | Not logged in | 1. On landing page, click "Sign In" in the hero actions area | Redirected to `/login` | [ ] |
| LAND-005 | Hero Join Now link | Not logged in | 1. On landing page, click "Join Now" in the hero actions area | Redirected to `/register` | [ ] |
| LAND-006 | CTA Create Account link | Not logged in | 1. Scroll to bottom CTA section, click "Create your account" | Redirected to `/register` | [ ] |
| LAND-007 | Logged-in user redirects to feed | Logged in as any user | 1. Navigate to `/` | Redirected to `/feed` | [ ] |

---

## 2. Authentication

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| AUTH-001 | Register new account | None | 1. Navigate to `/register` 2. Enter first name, last name, email, password (8+ chars) 3. Click Register | Account created, auto-logged in, redirected to `/feed` | [ ] |
| AUTH-002 | Register with missing fields | None | 1. Navigate to `/register` 2. Leave one or more fields blank 3. Submit | Error message "All fields required." displayed | [ ] |
| AUTH-003 | Register with short password | None | 1. Navigate to `/register` 2. Enter all fields, password < 8 chars 3. Submit | Error message "Password must be at least 8 characters." | [ ] |
| AUTH-004 | Register with duplicate email | Existing account with that email | 1. Register with an email already in use | Error message "An account with this email already exists." | [ ] |
| AUTH-005 | Login with valid credentials | Registered account exists | 1. Navigate to `/login` 2. Enter correct email/password 3. Submit | Logged in, redirected to `/feed` | [ ] |
| AUTH-006 | Login with invalid email | None | 1. Navigate to `/login` 2. Enter non-existent email 3. Submit | Error message "Invalid email or password." | [ ] |
| AUTH-007 | Login with wrong password | Registered account exists | 1. Navigate to `/login` 2. Enter correct email, wrong password 3. Submit | Error message "Invalid email or password." | [ ] |
| AUTH-008 | Logout | Logged in | 1. Open avatar dropdown 2. Click "Sign Out" | Redirected to `/login`, session destroyed | [ ] |
| AUTH-009 | Session persists across navigation | Logged in | 1. Navigate to `/feed` 2. Navigate to `/profile/me` 3. Navigate to `/connections` | All pages load authenticated (navbar shows avatar/name) | [ ] |
| AUTH-010 | Unauthenticated access blocked | Not logged in | 1. Navigate to `/feed` directly | Redirected to `/login` | [ ] |

---

## 3. Profiles

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| PROF-001 | View own profile | Logged in | 1. Click "View Profile" in avatar dropdown (or navigate to `/profile/me`) | Redirected to `/profile/{id}`, shows own name, email, headline, about, avatar, experience, education, skills sections | [ ] |
| PROF-002 | Edit profile | Logged in | 1. Click "Edit Profile" in avatar dropdown 2. Change first name, last name, headline, location, about 3. Submit | Profile updated, redirected to profile view with changes visible | [ ] |
| PROF-003 | Upload avatar (JPEG) | Logged in | 1. On own profile, use avatar upload form 2. Select a JPEG file under 5MB 3. Submit | Avatar saved, visible on profile page and in navbar | [ ] |
| PROF-004 | Upload avatar (PNG) | Logged in | 1. Upload a PNG file as avatar | Avatar saved and displayed correctly | [ ] |
| PROF-005 | Upload avatar -- invalid type | Logged in | 1. Upload a non-image file (e.g., .txt) | Error "Invalid file type" displayed | [ ] |
| PROF-006 | Upload avatar -- too large | Logged in | 1. Upload an image > 5MB | Error "File too large (max 5MB)" displayed | [ ] |
| PROF-007 | Avatar served publicly | Avatar uploaded | 1. Copy avatar URL `/avatar/{userID}` 2. Open in incognito window (no login) | Avatar image displays (public route, no auth) | [ ] |
| PROF-008 | View another user's profile | Logged in, User B exists | 1. Navigate to `/profile/{userB-id}` | User B's profile shown; "Connect" or "Message" button visible; edit controls NOT shown | [ ] |
| PROF-009 | Profile completeness bar | Logged in, own profile | 1. Navigate to own profile | Completeness bar/percentage shown; increases as headline, location, about, experience, education, skills are added | [ ] |

---

## 4. Experience

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| EXP-001 | Add experience | Logged in | 1. Navigate to `/profile/experience/add` 2. Fill in title, company, location, start date, description 3. Submit | Experience appears on profile page | [ ] |
| EXP-002 | Add experience -- validation | Logged in | 1. Navigate to add experience 2. Leave title, company, or start date blank 3. Submit | Error "Title, company, and start date are required." | [ ] |
| EXP-003 | Edit experience | Has at least 1 experience | 1. On profile, click edit on an experience 2. Change title and company 3. Submit | Changes saved, visible on profile | [ ] |
| EXP-004 | Delete experience | Has at least 1 experience | 1. On profile, click delete on an experience | Experience removed from profile | [ ] |
| EXP-005 | Experience with end date | Logged in | 1. Add experience with both start and end date | Both dates displayed correctly | [ ] |
| EXP-006 | Current position (no end date) | Logged in | 1. Add experience with start date only (no end date) | Experience shows as current/ongoing | [ ] |

---

## 5. Education

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| EDU-001 | Add education | Logged in | 1. Navigate to `/profile/education/add` 2. Fill in school, degree, field of study, start year 3. Submit | Education appears on profile page | [ ] |
| EDU-002 | Add education -- validation | Logged in | 1. Leave school or start year blank 2. Submit | Error "School and start year are required." | [ ] |
| EDU-003 | Edit education | Has at least 1 education | 1. Click edit on an education entry 2. Change school name 3. Submit | Changes saved, visible on profile | [ ] |
| EDU-004 | Delete education | Has at least 1 education | 1. Click delete on an education entry | Education removed from profile | [ ] |

---

## 6. Skills

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| SKILL-001 | Add skill | Logged in | 1. Navigate to `/profile/skills/edit` 2. Enter skill name 3. Submit | Skill appears in skills list | [ ] |
| SKILL-002 | Add duplicate skill | Has skill "Go" | 1. Add skill "Go" again | No duplicate created (ON CONFLICT DO NOTHING) | [ ] |
| SKILL-003 | Delete skill | Has at least 1 skill | 1. Click delete on a skill | Skill removed from list | [ ] |
| SKILL-004 | Endorse another user's skill | Logged in as User A, viewing User B's profile, User B has skills | 1. On User B's profile, click endorse button next to a skill | Endorsement count increments by 1; button shows as endorsed | [ ] |
| SKILL-005 | Un-endorse a skill | Previously endorsed User B's skill | 1. Click the endorse button again on the same skill | Endorsement removed; count decrements | [ ] |
| SKILL-006 | Cannot self-endorse | Logged in, viewing own profile | 1. Attempt to POST to `/profile/skills/{id}/endorse` for own skill | Error "Cannot endorse your own skill" (HTTP 400) | [ ] |

---

## 7. Connections

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| CONN-001 | Send connection request | Logged in as User A, not connected to User B | 1. Navigate to User B's profile 2. Click "Connect" button | Request sent; User B's profile shows "Pending" status; notification sent to User B | [ ] |
| CONN-002 | Accept connection request | Logged in as User B, pending request from User A | 1. Navigate to `/connections` 2. Find User A in Pending section 3. Click "Accept" | Connection accepted; User A appears in accepted connections; notification sent to User A | [ ] |
| CONN-003 | Reject connection request | Logged in as User B, pending request from User A | 1. Navigate to `/connections` 2. Find User A in Pending section 3. Click "Reject" | Request removed from pending list; no connection created | [ ] |
| CONN-004 | View connections list | Logged in, has accepted connections | 1. Navigate to `/connections` | Page shows two sections: "Pending Invitations" and "Accepted Connections" with correct users | [ ] |
| CONN-005 | Cannot connect with self | Logged in | 1. Attempt to POST to `/connections/request/{ownID}` | Error "Cannot connect with yourself" (HTTP 400) | [ ] |
| CONN-006 | Duplicate request blocked | Already sent request to User B | 1. Try to send another connection request to User B | Redirected to User B's profile (no duplicate row created) | [ ] |

---

## 8. Search

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| SRCH-001 | Search by name | Logged in, multiple users exist | 1. Navigate to `/search` 2. Enter a user's first or last name 3. Submit | Matching users displayed in results | [ ] |
| SRCH-002 | Search by headline | Users have headlines set | 1. Search by a keyword in a user's headline | User with matching headline appears in results | [ ] |
| SRCH-003 | Search by location | Users have location set | 1. Search by a location string | Users in that location appear | [ ] |
| SRCH-004 | Connection status in results | Connected to some users, not others | 1. Search for users | Each result shows correct status: "Connect" button, "Pending", or "Connected" | [ ] |
| SRCH-005 | No results | Logged in | 1. Search for a string that matches no users | Empty results displayed (no errors) | [ ] |
| SRCH-006 | Own profile excluded | Logged in | 1. Search for your own name | Own profile does NOT appear in results | [ ] |

---

## 9. Notifications

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| NOTIF-001 | Notification dropdown displays | Logged in, has notifications | 1. Click the "Notifications" button in navbar | Dropdown opens showing recent notifications with sender initials/avatar, message, and time | [ ] |
| NOTIF-002 | Badge count shows unread | Has unread notifications | 1. Observe navbar notification button | Red badge with unread count is visible | [ ] |
| NOTIF-003 | Real-time badge update (SSE) | Two browsers: User A and User B connected | 1. In Browser A (User A), observe notification badge 2. In Browser B (User B), send connection request to User A | User A's badge count increments without page refresh | [ ] |
| NOTIF-004 | Click notification marks as read | Has unread notification | 1. Open notification dropdown 2. Click a notification link | Notification marked as read; redirected based on type (connection -> `/connections`, feed -> `/feed`, etc.) | [ ] |
| NOTIF-005 | Mark all as read | Has multiple unread notifications | 1. Open notification dropdown 2. Click "Mark all read" | All notifications marked as read; badge disappears or shows 0 | [ ] |
| NOTIF-006 | Delete notification | Has at least 1 notification | 1. Delete a notification from dropdown | Notification removed from list | [ ] |
| NOTIF-007 | Notification click routing -- connection request | Has connection request notification | 1. Click the notification | Redirected to `/connections` | [ ] |
| NOTIF-008 | Notification click routing -- message | Has message notification | 1. Click the notification | Redirected to `/feed?open_chat={convID}`, chat drawer opens | [ ] |
| NOTIF-009 | Notification click routing -- feed | Has feed notification (like/comment) | 1. Click the notification | Redirected to `/feed` | [ ] |
| NOTIF-010 | Notification click routing -- kudos | Has kudos notification | 1. Click the notification | Redirected to `/kudos` | [ ] |

---

## 10. Feed

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| FEED-001 | Create a post | Logged in | 1. Navigate to `/feed` 2. Type content in post composer 3. Submit | Post appears in feed with author name, avatar, timestamp | [ ] |
| FEED-002 | Feed shows connections' posts | Connected to User B, User B has posts | 1. Navigate to `/feed` on Social tab | User B's posts visible in feed | [ ] |
| FEED-003 | Delete own post | Has at least 1 post | 1. Click delete on own post | Post removed from feed | [ ] |
| FEED-004 | Cannot delete others' posts | Viewing another user's post | 1. Verify no delete button appears on posts authored by others | Delete button only appears on own posts | [ ] |
| FEED-005 | Like a post | Viewing a post | 1. Click Like on a post | Like count increments; button shows as liked | [ ] |
| FEED-006 | Unlike a post | Previously liked a post | 1. Click Like again on the same post | Like removed; count decrements | [ ] |
| FEED-007 | React with Celebrate | Viewing a post | 1. Select "Celebrate" reaction on a post | Reaction recorded; reaction type shown as "celebrate" | [ ] |
| FEED-008 | React with Insightful | Viewing a post | 1. Select "Insightful" reaction on a post | Reaction recorded; reaction type shown as "insightful" | [ ] |
| FEED-009 | Change reaction type | Already reacted "like" to a post | 1. Select "Celebrate" on the same post | Reaction updated from "like" to "celebrate" | [ ] |
| FEED-010 | Add a comment | Viewing a post | 1. Expand comments section 2. Type a comment 3. Submit | Comment appears under the post with author name and timestamp | [ ] |
| FEED-011 | Comment notification | User A comments on User B's post | 1. User A adds a comment | User B receives notification "User A commented on your post" | [ ] |
| FEED-012 | Reaction notification | User A reacts to User B's post | 1. User A reacts to a post | User B receives notification "User A reacted to your post" | [ ] |
| FEED-013 | Hashtag in post | Logged in | 1. Create a post containing `#golang` | Hashtag stored in hashtags table; post linked via post_hashtags | [ ] |
| FEED-014 | Hashtag creates clickable link | Post with hashtag exists | 1. View the post in feed | `#golang` renders as a clickable link to `/feed/hashtag/golang` | [ ] |
| FEED-015 | Hashtag feed page | Posts tagged with #golang exist | 1. Navigate to `/feed/hashtag/golang` | Page shows all posts tagged with #golang | [ ] |
| FEED-016 | @mention sends notification | User B exists | 1. Create a post with `@FirstName LastName` matching User B | User B receives notification "mentioned you in a post" | [ ] |
| FEED-017 | File attachment upload | Logged in | 1. Create a post with a file attached via the file input | Post created; attachment stored and linked to post | [ ] |
| FEED-018 | Attachment displays/downloads | Post with attachment exists | 1. View post with attachment 2. Click attachment link | File downloads via `/feed/attachment/{id}` with correct name and content type | [ ] |
| FEED-019 | Share a post (repost) | Viewing another user's post | 1. Click Share on a post | Shared post record created; author notified "shared your post" | [ ] |
| FEED-020 | Share with commentary | Viewing another user's post | 1. Click Share 2. Add commentary text 3. Submit | Shared post created with commentary field populated | [ ] |
| FEED-021 | Real-time feed update (SSE) | Two browsers, both on /feed | 1. User A creates a post 2. Observe User B's feed | User B's feed refreshes automatically (SSE event triggers reload) | [ ] |
| FEED-022 | Postings tab | Logged in, has skills matching active postings | 1. Navigate to `/feed?tab=postings` | Shows skill-matched postings from the postings table | [ ] |
| FEED-023 | Trending page | Posts with hashtags from last 7 days exist | 1. Navigate to `/feed/trending` | Shows top trending hashtags with post counts | [ ] |
| FEED-024 | Drafts -- save draft | Logged in | 1. Navigate to `/feed/drafts` 2. Enter content 3. Click Save Draft | Draft saved, appears in drafts list | [ ] |
| FEED-025 | Drafts -- publish draft | Has at least 1 draft | 1. Click Publish on a draft | Draft converted to post, removed from drafts, appears in feed | [ ] |
| FEED-026 | Drafts -- delete draft | Has at least 1 draft | 1. Click Delete on a draft | Draft removed from list | [ ] |

---

## 11. Messaging (Chat Drawer)

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| MSG-001 | Open chat drawer | Logged in | 1. Click the chat drawer toggle (checkbox) in the page layout | Chat drawer slides open showing recent conversations list | [ ] |
| MSG-002 | Click existing conversation | Has at least 1 conversation | 1. Open chat drawer 2. Click a conversation | Conversation view opens with message history, input field at bottom | [ ] |
| MSG-003 | Send a message | In a conversation | 1. Type a message in the input 2. Press Send/Enter | Message appears in chat as own message (right-aligned bubble) | [ ] |
| MSG-004 | Receive message real-time | Two browsers: User A and User B in same conversation | 1. User A sends a message 2. Observe User B's chat | User B sees the new message appear within a few seconds (polling) | [ ] |
| MSG-005 | Start new conversation via + New | Logged in, connected to User B | 1. Open chat drawer 2. Click "+ New" button 3. Search for User B 4. Select User B 5. Click Start | New conversation created; chat view opens | [ ] |
| MSG-006 | Search contacts -- Connections filter | Has connections | 1. In new chat modal, search with Connections scope | Only connected users appear in results | [ ] |
| MSG-007 | Search contacts -- All Users filter | Users exist | 1. In new chat modal, select "All Users" scope 2. Search | All matching users appear (not just connections) | [ ] |
| MSG-008 | Create group conversation (3+ people) | Connected to at least 2 other users | 1. Click + New 2. Search and select User B 3. Search and select User C 4. Click Start | Group conversation created with 3 participants | [ ] |
| MSG-009 | Group chat shows all participant names | In group conversation | 1. View the group conversation | Header shows all participant names (comma-separated) | [ ] |
| MSG-010 | Typing indicator | Two browsers: User A and User B in same conversation | 1. User A starts typing in the input field | Typing event sent to server via `/messages-typing/{id}`; User B's chat may refresh | [ ] |
| MSG-011 | Read receipt on sent messages | User A sent message, User B opened the conversation | 1. User A views the conversation after User B has opened it | User A's messages show "Read" indicator | [ ] |
| MSG-012 | Open chat from notification click | Has a message notification | 1. Click message notification in dropdown | Redirected to `/feed?open_chat={convID}`, chat drawer opens to that conversation | [ ] |
| MSG-013 | Unread count on conversations | User B sent messages User A hasn't read | 1. User A opens chat drawer | Conversation with User B shows unread count badge | [ ] |
| MSG-014 | First message with new chat | Starting a new conversation | 1. Click + New 2. Select a user 3. Type a first message in the modal 4. Click Start | Conversation created AND first message sent immediately | [ ] |
| MSG-015 | Message via profile button | Viewing User B's profile, connected | 1. Click "Message" button on User B's profile | Chat drawer opens, conversation with User B loaded | [ ] |
| MSG-016 | Pin a contact | In chat drawer | 1. Pin a contact via `/messages-pin/{userID}` | Contact appears in "Pinned" section of chat drawer | [ ] |
| MSG-017 | Unpin a contact | Contact is pinned | 1. Unpin the contact | Contact removed from "Pinned" section | [ ] |

---

## 12. Postings

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| POST-001 | Manager creates a posting | Logged in as manager | 1. Navigate to `/opportunities/create` 2. Fill in title, description, type (project/detail), location, department, skills (comma-separated) 3. Submit | Posting created, redirected to `/opportunities/{id}` showing the posting with skills listed | [ ] |
| POST-002 | Employee cannot create posting | Logged in as employee | 1. Navigate to `/opportunities/create` | Access denied (403 Forbidden) | [ ] |
| POST-003 | Posting appears in matched feed | Employee has skills matching posting's skills | 1. Navigate to `/feed?tab=postings` | Posting appears in the Postings tab | [ ] |
| POST-004 | Posting does NOT appear for non-matching | Employee has NO matching skills | 1. Navigate to `/feed?tab=postings` | Posting does NOT appear | [ ] |
| POST-005 | Browse all postings | Logged in | 1. Navigate to `/opportunities/search` | Search page displays, can see active postings | [ ] |
| POST-006 | Search postings by keyword | Active postings exist | 1. On `/opportunities/search`, enter a search query 2. Submit | Matching postings shown (searches title, description, skills) | [ ] |
| POST-007 | Search postings by type | Active postings of both types exist | 1. Search with type filter "project" | Only project-type postings shown | [ ] |
| POST-008 | Employee applies to posting | Logged in as employee, viewing active posting | 1. Navigate to `/opportunities/{id}/apply` 2. Fill in both application responses 3. Submit | Application submitted; redirected to posting view | [ ] |
| POST-009 | Duplicate application blocked | Already applied to posting | 1. Try to apply again | Error "You have already applied to this posting" | [ ] |
| POST-010 | Application to closed posting | Posting status is "closed" | 1. Navigate to `/opportunities/{id}/apply` | Error "This posting is no longer accepting applications" | [ ] |
| POST-011 | Manager views applications | Logged in as posting author | 1. Navigate to `/opportunities/{id}/applications` | List of all applications with applicant names and statuses | [ ] |
| POST-012 | Manager changes application status | Viewing applications | 1. Change an application status to "shortlisted", "accepted", or "rejected" | Status updated; page refreshes showing new status | [ ] |
| POST-013 | Manager closes a posting | Logged in as posting author | 1. Click close on `/opportunities/{id}/close` | Posting status changed to "closed" | [ ] |
| POST-014 | Closed posting shows status | Posting is closed | 1. View the posting at `/opportunities/{id}` | Status displays as "closed" | [ ] |
| POST-015 | My Posts -- postings tab | Logged in as manager with postings | 1. Navigate to `/my-posts?tab=postings` | Shows list of own postings with title, type, status, match count, date | [ ] |

---

## 13. Resumes

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| RES-001 | Upload PDF resume | Logged in | 1. Navigate to `/resumes` 2. Select a PDF file 3. Submit | Resume uploaded, appears in list with original filename | [ ] |
| RES-002 | Upload non-PDF blocked | Logged in | 1. Try to upload a .docx or .txt file | Error "Only PDF files are allowed" | [ ] |
| RES-003 | Download resume | Has uploaded resume | 1. Click download link on a resume | PDF downloads with original filename, Content-Disposition: attachment | [ ] |
| RES-004 | Delete resume | Has uploaded resume | 1. Click delete on a resume | Resume removed from list | [ ] |
| RES-005 | Generate resume from profile | Has profile data (experience, education, skills) | 1. Navigate to `/resumes/generate` | Print-friendly HTML page renders with name, headline, location, about, experiences, education, skills | [ ] |
| RES-006 | Print/Save generated resume | On generated resume page | 1. Use browser Print (Ctrl+P / Cmd+P) 2. Select "Save as PDF" | Clean PDF generated from the print-friendly layout | [ ] |
| RES-007 | Maximum 5 resumes enforced | Has exactly 5 uploaded resumes | 1. Try to upload a 6th resume | Error "Maximum 5 resumes allowed. Delete one before uploading." (HTTP 400) | [ ] |

---

## 14. Roles and Admin

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| ADMIN-001 | Seed admin exists on first boot | Fresh database | 1. Start the application 2. Check server logs | Log message "seeded default admin account" with admin@jobportal.local | [ ] |
| ADMIN-002 | Admin access to user management | Logged in as admin | 1. Open avatar dropdown 2. Click "User Management" | `/admin/users` loads showing all users with role dropdowns | [ ] |
| ADMIN-003 | Admin changes user role | On admin users page | 1. Change a user's role dropdown to "manager" 2. Submit | Role updated; audit log entry appears in server logs | [ ] |
| ADMIN-004 | Admin cannot change own role | On admin users page | 1. Attempt to change own role via POST to `/admin/users/{ownID}/role` | Error "Cannot change your own role" (HTTP 400) | [ ] |
| ADMIN-005 | Non-admin gets 403 on admin routes | Logged in as employee | 1. Navigate to `/admin/users` | HTTP 403 Forbidden | [ ] |
| ADMIN-006 | Manager sees manager items in menu | Logged in as manager | 1. Open avatar dropdown | "My Postings" and "Skills Gap Analysis" links visible | [ ] |
| ADMIN-007 | Admin sees admin items in menu | Logged in as admin | 1. Open avatar dropdown | "User Management", "FOIA Tool", and "Skills Gap Analysis" links visible | [ ] |
| ADMIN-008 | Employee sees no manager/admin items | Logged in as employee | 1. Open avatar dropdown | No "User Management", "FOIA Tool", "My Postings", or "Skills Gap Analysis" links | [ ] |
| ADMIN-009 | /me/role endpoint | Logged in | 1. Fetch `GET /me/role` | Returns plain text role: "employee", "manager", or "admin" | [ ] |

---

## 15. News

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| NEWS-001 | Admin creates news article | Logged in as admin | 1. Navigate to `/news/create` 2. Enter title and content 3. Submit | Article created, redirected to `/news/{id}` | [ ] |
| NEWS-002 | Non-admin cannot create news | Logged in as employee | 1. Navigate to `/news/create` | HTTP 403 Forbidden | [ ] |
| NEWS-003 | Admin publishes article | Viewing unpublished article as author | 1. Click publish/unpublish toggle | Article published status toggled | [ ] |
| NEWS-004 | Published articles visible to all | Article is published | 1. Log in as any user 2. Navigate to `/news` | Published article appears in news list | [ ] |
| NEWS-005 | Unpublished article hidden from non-authors | Article is unpublished | 1. Log in as a non-author user 2. Navigate to `/news/{id}` | HTTP 404 Not Found | [ ] |
| NEWS-006 | Unpublished article visible to author | Article is unpublished | 1. Log in as the article author 2. Navigate to `/news/{id}` | Article displays with unpublished status indicator | [ ] |
| NEWS-007 | News creation validation | Logged in as admin | 1. Submit create form with empty title or content | Error "Title and content are required." | [ ] |

---

## 16. Articles

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| ART-001 | Any user can write article | Logged in as employee | 1. Navigate to `/articles/write` 2. Enter title and content 3. Submit | Article created, redirected to `/articles/{id}` | [ ] |
| ART-002 | Author can edit article | Viewing own article | 1. Navigate to `/articles/{id}/edit` 2. Change title and content 3. Submit | Article updated | [ ] |
| ART-003 | Non-author cannot edit | Viewing another user's article | 1. Navigate to `/articles/{id}/edit` for someone else's article | HTTP 404 (only author's articles returned by query) | [ ] |
| ART-004 | Articles list shows published | Multiple published articles exist | 1. Navigate to `/articles` | All published articles listed in reverse chronological order | [ ] |
| ART-005 | Article creation validation | Logged in | 1. Submit write form with empty fields | Error "Title and content are required" | [ ] |

---

## 17. Polls

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| POLL-001 | Manager creates poll | Logged in as manager | 1. Navigate to `/polls/create` 2. Enter question and at least 2 options 3. Submit | Poll created, appears on `/polls` page | [ ] |
| POLL-002 | Employee cannot create poll | Logged in as employee | 1. Navigate to `/polls/create` | HTTP 403 Forbidden | [ ] |
| POLL-003 | User votes on poll | Viewing a poll, has not voted | 1. Select an option 2. Submit | Vote recorded; results display with percentages | [ ] |
| POLL-004 | User cannot vote twice | Already voted on a poll | 1. Try to vote again | Redirected to `/polls` (no duplicate vote; existing vote preserved) | [ ] |
| POLL-005 | Results display after voting | Has voted on a poll | 1. View the polls page | Voted poll shows vote counts, percentages, and which option was selected | [ ] |
| POLL-006 | Poll creation requires 2+ options | Logged in as manager | 1. Create poll with only 1 option | Error "At least 2 options are required." | [ ] |
| POLL-007 | Poll question required | Logged in as manager | 1. Submit with empty question | Error "Question is required." | [ ] |

---

## 18. Groups

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| GRP-001 | Create a group | Logged in | 1. Navigate to `/groups/create` 2. Enter name and description 3. Submit | Group created; creator auto-added as admin member; redirected to group page | [ ] |
| GRP-002 | Group creation validation | Logged in | 1. Submit with empty name | Error "Group name is required." | [ ] |
| GRP-003 | Join a group | Not a member of a group | 1. Navigate to `/groups/{id}` 2. Click "Join" | Joined as member; membership reflected on page | [ ] |
| GRP-004 | Post in a group (member) | Member of a group | 1. On group page, enter post content 2. Submit | Post appears in group's post list | [ ] |
| GRP-005 | Non-member cannot post | Not a member of a group | 1. Attempt to POST to `/groups/{id}/post` | Error "You must be a member to post" (HTTP 403) | [ ] |
| GRP-006 | Leave a group | Member of a group (not sole admin) | 1. On group page, click "Leave" | Removed from group; no longer a member | [ ] |
| GRP-007 | Sole admin cannot leave | Only admin of a group | 1. Click "Leave" | Error "Cannot leave: you are the only admin" (HTTP 400) | [ ] |
| GRP-008 | Groups list | Groups exist | 1. Navigate to `/groups` | All groups listed with name, description, member count, and join/member status | [ ] |
| GRP-009 | Group detail shows posts | Group has posts | 1. Navigate to `/groups/{id}` | Group info displayed with posts listed chronologically | [ ] |

---

## 19. Department Directory

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| DEPT-001 | View department list | Departments exist in DB | 1. Navigate to `/departments` | All departments listed with name, description, parent department, employee count | [ ] |
| DEPT-002 | View department detail | Department has employees | 1. Click on a department | Department detail page shows department info and list of employees with name, headline, avatar | [ ] |

---

## 20. Accomplishments

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| ACC-001 | Add accomplishment | Logged in | 1. Navigate to `/accomplishments/add` 2. Fill in title, description, period type (quarterly/yearly), start date, end date 3. Submit | Accomplishment appears in list | [ ] |
| ACC-002 | Add accomplishment validation | Logged in | 1. Leave required fields blank 2. Submit | Error "All fields are required." | [ ] |
| ACC-003 | Edit accomplishment | Has at least 1 accomplishment | 1. Click edit on an accomplishment 2. Change title and description 3. Submit | Changes saved, visible in list | [ ] |
| ACC-004 | Delete accomplishment | Has at least 1 accomplishment | 1. Click delete on an accomplishment | Accomplishment removed from list | [ ] |
| ACC-005 | Filter by quarterly | Has quarterly and yearly accomplishments | 1. Navigate to `/accomplishments?period=quarterly` | Only quarterly accomplishments shown | [ ] |
| ACC-006 | Filter by yearly | Has quarterly and yearly accomplishments | 1. Navigate to `/accomplishments?period=yearly` | Only yearly accomplishments shown | [ ] |
| ACC-007 | Export accomplishments | Has accomplishments | 1. Navigate to `/accomplishments/export` | Print-friendly page showing accomplishments grouped by year with quarterly and yearly sections | [ ] |

---

## 21. Kudos

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| KUDO-001 | Send kudos | Logged in, viewing User B | 1. Navigate to `/kudos/send/{userB-id}` 2. Write a message 3. Submit | Kudos sent; redirected to User B's profile | [ ] |
| KUDO-002 | Kudos notification | Kudos sent to User B | 1. Check User B's notifications | User B received notification with sender name and kudos message | [ ] |
| KUDO-003 | View received kudos | Has received kudos | 1. Navigate to `/kudos` (defaults to "received" tab) | List of received kudos with sender name, message, date | [ ] |
| KUDO-004 | View sent kudos | Has sent kudos | 1. Navigate to `/kudos?tab=sent` | List of sent kudos with receiver name, message, date | [ ] |
| KUDO-005 | Kudos message required | On send kudos page | 1. Submit with empty message | Error "Message cannot be empty." | [ ] |

---

## 22. Bookmarks

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| BKM-001 | Bookmark a post | Viewing feed with posts | 1. Submit bookmark form with target_type=post, target_id={postID} via POST `/bookmarks/add` | Bookmark saved; redirected back to referrer | [ ] |
| BKM-002 | Bookmark an article | Viewing an article | 1. Submit bookmark form with target_type=article, target_id={articleID} | Bookmark saved | [ ] |
| BKM-003 | View bookmarks page | Has bookmarks | 1. Navigate to `/bookmarks` | All bookmarks listed with resolved titles (post content preview, article title, etc.) | [ ] |
| BKM-004 | Filter bookmarks by type | Has bookmarks of multiple types | 1. Navigate to `/bookmarks?tab=post` | Only post bookmarks shown | [ ] |
| BKM-005 | Delete a bookmark | Has at least 1 bookmark | 1. Click delete on a bookmark | Bookmark removed from list | [ ] |
| BKM-006 | Duplicate bookmark prevented | Already bookmarked a post | 1. Bookmark the same post again | No duplicate created (ON CONFLICT DO NOTHING) | [ ] |

---

## 23. Badges

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| BADGE-001 | View badges page | Logged in | 1. Navigate to `/badges` | Page shows "Earned" badges and "Available" badges sections | [ ] |
| BADGE-002 | First Post badge auto-awarded | User has created at least 1 post | 1. Trigger badge check (CheckAndAward) 2. Navigate to `/badges` | "First Post" badge appears in earned section | [ ] |
| BADGE-003 | Connector badge auto-awarded | User has 10+ accepted connections | 1. Trigger badge check 2. Navigate to `/badges` | "Connector" badge appears in earned section | [ ] |
| BADGE-004 | Profile Complete badge | User has headline, location, about, experience, education, and skills | 1. Trigger badge check 2. Navigate to `/badges` | "Profile Complete" badge appears in earned section | [ ] |
| BADGE-005 | Team Player badge | User has received 5+ kudos | 1. Trigger badge check 2. Navigate to `/badges` | "Team Player" badge appears in earned section | [ ] |

---

## 24. Follow

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| FOLLOW-001 | Follow a user | Viewing another user's profile | 1. Click Follow button (POST `/follow/{userID}`) | Follow relationship created; redirected to their profile | [ ] |
| FOLLOW-002 | Unfollow a user | Already following a user | 1. Click Follow/Unfollow button again | Follow relationship removed; redirected to their profile | [ ] |
| FOLLOW-003 | Cannot follow self | Logged in | 1. Attempt POST `/follow/{ownID}` | Error "Cannot follow yourself" (HTTP 400) | [ ] |

---

## 25. Mentorship

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| MENT-001 | View mentorship page | Logged in | 1. Navigate to `/mentorship` | Shows active/pending mentorships and "Find Mentor" tab | [ ] |
| MENT-002 | Find available mentors | On mentorship page | 1. Navigate to `/mentorship?tab=find` | List of potential mentors (users with more experience, sorted by shared skills) | [ ] |
| MENT-003 | Request a mentor | Available mentors listed | 1. Click "Request" on a mentor | Mentorship request created (status: pending); mentor receives notification | [ ] |
| MENT-004 | Mentor accepts request | Logged in as mentor with pending request | 1. Click "Accept" on the mentorship | Status changes to "active"; mentee receives notification | [ ] |
| MENT-005 | Mentor declines request | Logged in as mentor with pending request | 1. Click "Decline" on the mentorship | Mentorship deleted; mentee receives notification | [ ] |
| MENT-006 | Mark mentorship complete | Active mentorship exists | 1. Click "Complete" on an active mentorship | Status changes to "completed"; other party receives notification | [ ] |
| MENT-007 | Cannot mentor self | Logged in | 1. Attempt POST `/mentors/request/{ownID}` | Error "Cannot mentor yourself" (HTTP 400) | [ ] |

---

## 26. Feedback

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| FB-001 | View feedback page | Logged in | 1. Navigate to `/feedback` | Shows sent and received feedback requests with tabs | [ ] |
| FB-002 | Request feedback | Logged in, User B exists | 1. Navigate to `/feedback-ask/{userB-id}` 2. Enter subject 3. Submit | Feedback request created; User B receives notification | [ ] |
| FB-003 | Feedback subject required | On request form | 1. Submit with empty subject | Error "Subject is required" | [ ] |
| FB-004 | Reviewer responds | Logged in as reviewer, pending request | 1. Navigate to `/feedback-respond/{id}` 2. Enter feedback text 3. Submit | Request status changes to "completed"; requester receives notification | [ ] |
| FB-005 | Feedback text required | On respond form | 1. Submit with empty feedback | Error "Feedback text is required" | [ ] |
| FB-006 | Reviewer declines | Logged in as reviewer, pending request | 1. Click "Decline" on the feedback request | Status changes to "declined"; requester receives notification | [ ] |
| FB-007 | Only reviewer can respond | Logged in as non-reviewer | 1. Navigate to `/feedback-respond/{id}` for a request not assigned to you | HTTP 403 Forbidden | [ ] |

---

## 27. Onboarding

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| ONB-001 | View onboarding checklist | Logged in, onboarding steps exist in DB | 1. Navigate to `/onboarding` | Checklist of steps displayed with title, description, completion status | [ ] |
| ONB-002 | Mark step complete | On onboarding page | 1. Click "Complete" on an uncompleted step | Step marked as completed; progress percentage updates | [ ] |
| ONB-003 | Progress bar updates | Some steps completed | 1. View onboarding page | Progress bar shows correct percentage (completed / total * 100) | [ ] |
| ONB-004 | Already completed step | Step already marked complete | 1. Try to complete the same step again | No error (ON CONFLICT DO NOTHING); no change | [ ] |

---

## 28. Analytics

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| ANA-001 | Skills Gap Analysis (manager) | Logged in as manager, active postings with skills exist | 1. Navigate to `/analytics/skills-gap` | Table showing skills, demand count (postings), supply count (employees with skill), gap percentage | [ ] |
| ANA-002 | Skills Gap Analysis (employee blocked) | Logged in as employee | 1. Navigate to `/analytics/skills-gap` | HTTP 403 Forbidden | [ ] |
| ANA-003 | Learning Recommendations | Logged in, active postings exist | 1. Navigate to `/analytics/learning` | Shows skills in demand that the current user does NOT have, sorted by posting count | [ ] |

---

## 29. FOIA (Admin)

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| FOIA-001 | Access FOIA tool | Logged in as admin | 1. Navigate to `/admin/foia` | FOIA search page loads with search form | [ ] |
| FOIA-002 | Non-admin blocked | Logged in as employee | 1. Navigate to `/admin/foia` | HTTP 403 Forbidden | [ ] |
| FOIA-003 | Search by user name | Posts/comments/messages exist | 1. Enter a user name in the search field 2. Submit | Results show matching content authored by that user | [ ] |
| FOIA-004 | Search by date range | Content exists within date range | 1. Set from and to dates 2. Submit | Results filtered to content within that date range | [ ] |
| FOIA-005 | Search by content type | Content of various types exists | 1. Select "post" from type filter 2. Submit | Only posts shown (not comments or messages) | [ ] |
| FOIA-006 | Export results as JSON | Search results exist | 1. Click export link (GET `/admin/foia/export` with same query params) | JSON file downloads with Content-Disposition: attachment, filename "foia_export.json" | [ ] |

---

## 30. Dark Mode

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| DARK-001 | Toggle dark mode | Logged in | 1. Open avatar dropdown 2. Click "Dark Mode" | Page switches to dark theme; toggle text changes to "Light Mode" | [ ] |
| DARK-002 | Dark mode persists across navigation | Dark mode enabled | 1. Navigate to `/feed` 2. Navigate to `/profile/me` 3. Navigate to `/connections` | Dark mode stays active on all pages | [ ] |
| DARK-003 | Dark mode persists across restart | Dark mode enabled | 1. Close browser tab 2. Reopen the application | Dark mode is still active (stored in localStorage) | [ ] |
| DARK-004 | Toggle back to light mode | Dark mode enabled | 1. Open avatar dropdown 2. Click "Light Mode" | Page returns to light theme | [ ] |
| DARK-005 | All text readable in dark mode | Dark mode enabled | 1. Navigate through all major pages (feed, profile, connections, search, notifications, messaging, admin, etc.) | All text is clearly readable against dark backgrounds | [ ] |
| DARK-006 | Input fields visible in dark mode | Dark mode enabled | 1. View forms (post composer, search box, login, register, profile edit) | Input fields have visible borders, text is readable, placeholders visible | [ ] |
| DARK-007 | Cards distinguishable in dark mode | Dark mode enabled | 1. View feed posts, profile cards, connection cards | Cards are visually distinct from the page background | [ ] |

---

## 31. Mobile / Responsive

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| MOB-001 | Navbar wraps on small screens | Browser window < 600px | 1. Resize browser to mobile width | Navbar items wrap or collapse appropriately, no horizontal overflow | [ ] |
| MOB-002 | Chat drawer resizes | Browser window < 600px | 1. Open chat drawer on mobile-sized window | Drawer fits within viewport, no horizontal scrollbar | [ ] |
| MOB-003 | Feed posts readable | Browser window < 600px | 1. View feed on mobile-sized window | Posts, comments, and reactions are readable and tappable | [ ] |
| MOB-004 | Landing page responsive | Browser window < 600px | 1. View landing page at mobile width | Hero section stacks vertically, feature cards stack to single column | [ ] |
| MOB-005 | Forms usable on mobile | Browser window < 600px | 1. Try creating a post, editing profile, sending a message | Forms are usable, inputs are full-width, submit buttons are reachable | [ ] |

---

## 32. Navigation

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| NAV-001 | Avatar dropdown shows user info | Logged in | 1. Click avatar in navbar | Dropdown shows full name and role (employee/manager/admin) | [ ] |
| NAV-002 | Avatar dropdown role-based items | Logged in as admin | 1. Open avatar dropdown | Shows: View Profile, Edit Profile, My Network, User Management, FOIA Tool, Skills Gap Analysis, Dark Mode toggle, Sign Out | [ ] |
| NAV-003 | Explore dropdown shows all links | Logged in | 1. Click "Explore" in navbar | Shows: Search People, USDA News, Articles, Polls, Groups, Directory, Trending, Browse Postings | [ ] |
| NAV-004 | Chat drawer toggle | Logged in | 1. Click the chat drawer checkbox/toggle | Chat drawer opens/closes | [ ] |
| NAV-005 | No duplicate navigation items | Logged in | 1. Review navbar, explore dropdown, avatar dropdown, and chat drawer | No feature link appears in more than one location (no duplicates across menus) | [ ] |
| NAV-006 | Notification dropdown interaction | Logged in | 1. Click Notifications button 2. Click elsewhere on page | Dropdown opens on click, closes when clicking elsewhere | [ ] |

---

## 33. Health Check

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| HEALTH-001 | Health endpoint | Server running | 1. GET `/health` | HTTP 200 with body "ok" | [ ] |

---

## 34. Security / Edge Cases

| ID | Feature | Preconditions | Steps | Expected Result | Pass/Fail |
|----|---------|---------------|-------|-----------------|-----------|
| SEC-001 | Auth required on all protected routes | Not logged in | 1. Navigate to `/feed`, `/profile/me`, `/connections`, `/resumes`, `/admin/users` | All redirect to `/login` | [ ] |
| SEC-002 | Cannot edit another user's experience | Logged in as User A | 1. POST to `/profile/experience/{userB-exp-id}/edit` | HTTP 404 (WHERE user_id clause prevents access) | [ ] |
| SEC-003 | Cannot delete another user's resume | Logged in as User A | 1. POST to `/resumes/{userB-resume-id}/delete` | No deletion (WHERE user_id clause) | [ ] |
| SEC-004 | Cannot access others' conversations | Logged in as User A | 1. GET `/messages/chat/{conv-id-not-participant}` | HTTP 403 Forbidden | [ ] |
| SEC-005 | Manager role required for postings | Logged in as employee | 1. POST to `/opportunities/create` | HTTP 403 Forbidden | [ ] |
| SEC-006 | Admin role required for FOIA | Logged in as manager (not admin) | 1. GET `/admin/foia` | HTTP 403 Forbidden | [ ] |
| SEC-007 | Avatar content type sniffing | Upload image with wrong extension but valid image bytes | 1. Upload a JPEG renamed to .txt | Server detects content type via magic bytes; upload succeeds if it is a valid image | [ ] |
| SEC-008 | XSS in post content | Logged in | 1. Create a post with `<script>alert('xss')</script>` | Script tags are escaped by Go html/template; no alert fires | [ ] |

---

## Test Execution Summary

| Section | Total | Passed | Failed | Blocked |
|---------|-------|--------|--------|---------|
| Landing Page | 7 | | | |
| Authentication | 10 | | | |
| Profiles | 9 | | | |
| Experience | 6 | | | |
| Education | 4 | | | |
| Skills | 6 | | | |
| Connections | 6 | | | |
| Search | 6 | | | |
| Notifications | 10 | | | |
| Feed | 26 | | | |
| Messaging | 17 | | | |
| Postings | 15 | | | |
| Resumes | 7 | | | |
| Roles and Admin | 9 | | | |
| News | 7 | | | |
| Articles | 5 | | | |
| Polls | 7 | | | |
| Groups | 9 | | | |
| Department Directory | 2 | | | |
| Accomplishments | 7 | | | |
| Kudos | 5 | | | |
| Bookmarks | 6 | | | |
| Badges | 5 | | | |
| Follow | 3 | | | |
| Mentorship | 7 | | | |
| Feedback | 7 | | | |
| Onboarding | 4 | | | |
| Analytics | 3 | | | |
| FOIA | 6 | | | |
| Dark Mode | 7 | | | |
| Mobile / Responsive | 5 | | | |
| Navigation | 6 | | | |
| Health Check | 1 | | | |
| Security / Edge Cases | 8 | | | |
| **TOTAL** | **238** | | | |

---

**Tester:** ___________________________
**Date Started:** ___________________________
**Date Completed:** ___________________________
**Environment/Build:** ___________________________
