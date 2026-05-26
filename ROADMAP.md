# USDA JobPortal — Feature Roadmap & Development Report

_Last updated: 2026-05-23_

## Executive Summary

USDA JobPortal is an internal professional networking and workforce development platform built specifically for USDA employees. It addresses the unique needs of a 100,000+ employee organization: connecting employees with temporary detail opportunities, facilitating cross-agency collaboration, and providing leadership with workforce analytics — all at zero licensing cost compared to commercial alternatives.

The platform is built with Go, PostgreSQL, and Redis, containerized with Docker for straightforward deployment to USDA infrastructure.

---

## Completed Features

### Authentication & User Management
- Secure registration/login (bcrypt), Redis-backed sessions, role-based access (Employee/Manager/Admin)
- Seed admin on first deployment
- Admin user management (role/department changes, delete) with a real **audit log** (`/admin/audit`)

### Professional Profiles
- Headline, location, about; experience, education, skills
- Avatar upload (BYTEA, content-type validated)
- Profile completeness indicator
- Skill endorsements **and** manager/peer skill verification
- Birthday / work anniversary (opt-in) with a "celebrations this week" widget
- Pinned profile posts
- Print-friendly full profile view/export

### Social Feed
- Tabbed feed (Social / skill-matched Postings); posts, comments, collapsible threads
- Emoji reactions (👍 🎉 💡 🤔) with counts
- File attachments (display + download), share/repost with commentary
- Draft posts and **scheduled posts**
- Hashtags, trending, real-time updates via SSE
- @mentions in posts/comments with profile links + mention notifications
- Shared URL link previews

### Connections & Networking
- Connection requests (send/accept/reject), bidirectional unique constraint
- Follow (asymmetric)
- Personal network visualization (`/insights/network`)
- Organization chart (`/orgchart`) with admin manager assignment

### Search & Directory
- Employee search (name/headline/location) with connection status
- Department directory

### Messaging
- Direct (1:1) and group conversations, real-time via SSE
- Typing indicators, read receipts, pinned contacts, attachments
- 1:1 dedupe enforced at app + DB layers (no duplicate/misrouted direct chats)

### Notifications
- Real-time badge (SSE), click-to-route, mark read / mark all read

### Content
- Articles (long-form), USDA News, Polls/surveys, **Announcements** (pinned official comms)
- Automatic new-hire welcome posts on signup

### Recognition & Engagement
- Kudos, auto-awarded **badges** (incl. Detail Completed & Mentor), Employee Spotlight, Engagement Leaderboard

### Details & Postings
- Manager/admin postings with required skills + skill-matched feed and match counts
- Application flow: apply → shortlist → accept/reject
- Detail/project **outcomes** and per-employee **detail history**
- "My Posts" management; close-posting workflow

### Career Development
- Mentorship matching, accomplishments tracking + export, resume builder, onboarding checklist
- Weekly digest (in-app / on-screen)

### Leadership & Analytics
- Workforce Analytics Dashboard, Admin Dashboard, Skills Gap Analysis, Skills Heat Map
- Export Reports (standalone print-friendly HTML), FOIA search/export tool

### Collaboration & Navigation
- Project workspaces with members, notes/updates, and workspace-level collaboration
- Shared app shell/navigation with branded navbar, primary app bar, and all-tools drawer

### Platform & Branding
- Public landing page, USDA branding, dark mode, mobile responsive, realistic demo seed data
- Section 508 basics (focus states, contrast)

---

## Remaining Roadmap

### Engagement & Content
| Feature | Description | Value |
|---------|-------------|-------|
| Trending Topics (advanced) | Auto-detected discussion themes (basic trending exists) | Org pulse |

### Leadership & Reporting
| Feature | Description | Value |
|---------|-------------|-------|
| Weekly Digest Email | Email delivery of the existing in-app weekly digest | Re-engagement |
| PDF Export | True generated PDF reports (standalone print-friendly HTML/browser print shipped) | Shareable intelligence |
| Org-scale Network/Heat visualizations | Beyond personal graph + dept heat map | Strategic planning |

### Communication & Collaboration
| Feature | Description | Value |
|---------|-------------|-------|
| Group messaging enhancements | Conversation roles/admin controls, member removal/leave flows, and moderation polish (naming + add-member shipped) | Team communication |

### Career & Development
| Feature | Description | Value |
|---------|-------------|-------|
| Certification Tracking | Track certs with expiry alerts | Compliance/readiness |
| Telework / Out-of-Office Status | In Office / Telework / TDY / leave | Availability awareness |

### Security & Compliance
| Feature | Description | Value |
|---------|-------------|-------|
| MFA (Production) | TOTP two-factor | Security requirement |
| Content Moderation | Flag/report with admin review | Platform safety |
| Personal Data Export | User data export | Policy compliance |
| Session Management | Active sessions view, remote logout | Security |
| PII Redaction Warning | Detect sensitive data before posting | Data protection |
| Privacy Controls | Profile visibility settings | User trust |
| Acceptable Use Policy | Governance framework | Compliance |
| Accessibility (full 508) | Full ARIA + keyboard-navigation audit | Federal requirement |

### Integration
| Feature | Description | Value |
|---------|-------------|-------|
| SSO / Active Directory | PIV card / AD login | Enterprise adoption |
| Calendar Integration | Outlook sync for availability | Workflow integration |
| Notification Preferences | Granular controls | User experience |
| REST API | Third-party integration | Platform extensibility |

### Long-term Vision
| Feature | Description | Value |
|---------|-------------|-------|
| Internal Training Platform | Courses, enrollment, completion tracking tied to skills | Workforce development at scale |

---

## Technical Architecture

- **Language:** Go (stdlib, no web framework)
- **Database:** PostgreSQL 16 with UUID primary keys
- **Cache/Sessions:** Redis 7
- **Real-time:** Server-Sent Events (SSE) for notifications, feed, and messaging
- **File Storage:** PostgreSQL BYTEA (no filesystem dependencies)
- **Containerization:** Docker with multi-stage builds
- **Authentication:** bcrypt password hashing, Redis sessions, role-based middleware
- **Target Scale:** ~100,000 users

---

## Cost Comparison

| | USDA JobPortal | LinkedIn Business | Commercial Alternative |
|---|---|---|---|
| Per-seat cost | $0 | ~$60/user/year | Varies |
| Customization | Full (built for USDA) | None | Limited |
| Data ownership | 100% USDA-controlled | Third-party | Third-party |
| Detail/Project matching | Built-in skill matching | Not available | Not available |
| Government compliance | Built for Section 508, PII protection | Generic | Generic |
| Integration with USDA systems | Planned (AD, Outlook) | No | Varies |

---

*This document is maintained alongside development. Feature status and priorities may shift based on leadership feedback and organizational needs.*
