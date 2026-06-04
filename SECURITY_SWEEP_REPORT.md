# Full Security Sweep Report

Date: 2026-05-28
Scope: USDA JobPortal application in this repository only
Assessment Type: Static code review plus live authenticated pentesting against local runtime
Constraint honored: No source code files were modified during this exercise

## 1) Executive Summary

This security sweep identified multiple high-impact vulnerabilities, including:

- Cross-Site Request Forgery (CSRF) across state-changing endpoints
- Server-Side Request Forgery (SSRF) through link preview fetching
- Insecure Direct Object Reference (IDOR) on feed attachments
- Open redirect via untrusted Referer handling
- Public unauthenticated avatar access
- Weak/insecure runtime defaults (secrets, transport, exposed data stores)

In addition, several reliability/quality bugs were confirmed and documented.

## 2) Methodology

### 2.1 Static Review

Reviewed server bootstrap, middleware chain, auth/session handling, feed/link preview, attachment serving, admin/notification/profile handlers, and deployment configuration.

Primary reviewed files:

- cmd/server/main.go
- internal/auth/session.go
- internal/auth/handler.go
- internal/middleware/auth.go
- internal/feed/handler.go
- internal/feed/linkpreview.go
- internal/posting/handler.go
- internal/attachment/validate.go
- internal/bookmark/handler.go
- internal/messaging/handler.go
- internal/user/handler.go
- internal/notification/handler.go
- docker-compose.yml

### 2.2 Live Pentesting

Live runtime tests were executed against localhost:8080 with docker compose services up.

Tested areas:

- Authenticated state change behavior under malicious Origin/Referer conditions
- Cross-account object access behavior (attachment endpoints)
- Link preview behavior using internal URLs
- Redirect target trust behavior
- Public endpoint exposure
- Login abuse controls
- Session cookie attributes in HTTP responses

## 3) Findings

Severity scale: Critical, High, Medium, Low

---

### F-01 Critical: CSRF protections missing on state-changing actions

Category: Web application request integrity

Risk:

- Logged-in users can be forced by third-party sites to execute unintended actions.

Static evidence:

- POST state-changing routes registered broadly without CSRF middleware:
  - internal/feed/handler.go:41-53
  - internal/admin/handler.go:115-117
  - internal/messaging/handler.go:37-46
- Middleware chain in server bootstrap has auth and AUP gating but no CSRF enforcement:
  - cmd/server/main.go:173-175
- Forms do not contain CSRF token fields:
  - templates/feed/feed.html:12, 46, 49, 97, 103, 109, 115, 125, 139, 164

Live exploit evidence:

- A state-changing request succeeded with malicious Origin and Referer headers:
  - POST /connections/request/{id}
  - Origin: http://evil.example
  - Referer: http://evil.example/pwn
  - Response: 303 redirect
  - Database confirmed inserted pending connection row

Impact:

- Account actions, profile changes, moderation actions, admin updates, and content actions can be cross-site triggered.

Remediation:

1. Add CSRF middleware globally for unsafe methods.
2. Embed CSRF token hidden fields in all HTML forms.
3. Require token validation on POST/PUT/PATCH/DELETE.
4. Optionally add strict Origin/Referer checks as defense-in-depth.

---

### F-02 Critical: SSRF via link preview fetcher

Category: Server-side outbound request abuse

Risk:

- User-supplied URLs in posts are fetched server-side with no host/IP allow/deny validation.

Static evidence:

- URL extraction and async fetch trigger:
  - internal/feed/handler.go:478-482
- Remote request built directly from user URL:
  - internal/feed/linkpreview.go:117-118

Live exploit evidence:

- Posted content containing internal URL: http://localhost:8080/login
- link_previews table stored fetched preview entry for that internal URL:
  - url: http://localhost:8080/login
  - title field populated

Impact:

- Internal service probing and metadata endpoint targeting are feasible.

Remediation:

1. Validate scheme strictly to http/https only.
2. Resolve DNS and block private, loopback, link-local, multicast, and localhost destinations.
3. Re-validate redirect hops or disable redirects.
4. Add egress firewall controls at infrastructure level.
5. Enforce fetch size/time limits (already partially present) plus domain policy.

---

### F-03 High: IDOR on feed attachment retrieval

Category: Broken object-level authorization

Risk:

- Feed attachment retrieval does not verify that requester is authorized to view parent post.

Static evidence:

- Handler loads attachment by id only:
  - internal/feed/handler.go:512-523

Live exploit evidence:

- User1 uploaded private feed attachment.
- User2 (not connected/authorized by feed visibility model) fetched direct URL /feed/attachment/{id}.
- Response: HTTP 200 and attachment bytes returned.

Impact:

- Cross-user data exposure of private/shared files.

Remediation:

1. Join attachment lookup to parent post and enforce post visibility authorization before serving.
2. Return 404 for unauthorized access to reduce enumeration signal.
3. Add authorization tests for cross-user attachment access.

---

### F-04 High: Open redirect via untrusted Referer

Category: Unvalidated redirect target

Risk:

- Redirect target is read directly from request Referer and used in response.

Static evidence:

- internal/bookmark/handler.go:164-168
- internal/messaging/handler.go:665-669
- internal/user/handler.go:1091-1095

Live exploit evidence:

- POST /bookmarks/add with Referer: https://evil.example/phish
- Response: HTTP 303 Location: https://evil.example/phish

Impact:

- Phishing and user redirection abuse.

Remediation:

1. Redirect only to relative in-app paths.
2. If absolute URLs are allowed, enforce same-origin host allowlist.
3. Use safe fallback routes when validation fails.

---

### F-05 High: Insecure/default operational configuration

Category: Secrets and transport hardening

Risk:

- Weak defaults and exposed services increase compromise likelihood.

Static evidence:

- Default secrets and credentials:
  - cmd/server/main.go:329-333
  - docker-compose.yml:9, 20
- Non-TLS DB URL default:
  - cmd/server/main.go:329
  - docker-compose.yml:7
- DB/Redis ports published broadly:
  - docker-compose.yml:22, 34

Impact:

- Easier credential abuse, accidental insecure deployments, and direct datastore exposure.

Remediation:

1. Remove insecure defaults for production paths.
2. Fail startup when secrets are default-like or missing in non-dev environments.
3. Avoid publishing DB/Redis ports in production.
4. Enforce TLS where applicable.
5. Move secrets to managed secret stores.

---

### F-06 High: Public unauthenticated avatar exposure

Category: Data exposure / access control

Risk:

- Avatar endpoint is public and returns user media without auth checks.

Static evidence:

- Public route registration:
  - cmd/server/main.go:230
- Avatar serving by profile id without auth/visibility checks:
  - internal/user/handler.go:1000-1018

Live exploit evidence:

- GET /avatar/{user-id} without cookies
- Response: HTTP 200, image bytes returned

Impact:

- Unauthenticated access to internal user media.

Remediation:

1. Require authentication for avatar retrieval.
2. Enforce profile visibility rules before serving.
3. Consider signed short-lived URLs for media.

---

### F-07 Medium: Missing login rate limiting / lockout

Category: Authentication abuse resistance

Risk:

- Repeated failed logins are accepted indefinitely without throttling.

Static evidence:

- Login handler performs credential check only:
  - internal/auth/handler.go:262-302

Live exploit evidence:

- Five consecutive bad password attempts all returned normal login response path.
- Immediate successful login with correct password still allowed.

Impact:

- Increased brute-force and credential stuffing risk.

Remediation:

1. Add per-IP and per-account rate limits.
2. Add temporary lockouts and/or progressive delay.
3. Alert on suspicious auth failure patterns.

---

### F-08 Medium: Large multipart parsing without explicit body cap in chat upload

Category: Resource exhaustion / DoS

Risk:

- Chat send parses multipart at large size without MaxBytesReader wrapper.

Static evidence:

- internal/messaging/handler.go:531-533

Impact:

- Elevated memory/disk pressure potential during upload abuse.

Remediation:

1. Wrap request body with MaxBytesReader before ParseMultipartForm.
2. Use strict category-based file and total request size limits.
3. Apply global request size protections at proxy/server layer.

---

### F-09 Medium: Session cookie may be non-Secure in common runtime mode

Category: Session transport protection

Risk:

- Cookie Secure attribute depends on env flag and can be absent.

Static evidence:

- internal/auth/session.go:110, 166

Live evidence:

- Set-Cookie from login response:
  - session_id with HttpOnly and SameSite=Lax
  - Secure attribute absent in current runtime

Impact:

- Session leakage risk over non-TLS transport or misconfigured deployment paths.

Remediation:

1. Require Secure cookies in non-local environments.
2. Enforce HTTPS and HSTS in production.
3. Fail startup when secure cookie policy is not enabled in production profile.

---

### F-10 Medium bug: Notification sender avatar URL malformed

Category: Application correctness / UX bug

Static evidence:

- Sender avatar URL constructed as /avatar/ + avatar_url:
  - internal/notification/handler.go:362
- Avatar URL stored as /avatar during upload:
  - internal/user/handler.go:987

Live evidence:

- notifications/recent JSON returned:
  - sender_avatar: /avatar//avatar

Impact:

- Broken avatar rendering path in notifications surfaces.

Remediation:

1. Build avatar route from sender user id, not avatar_url string concatenation.
2. Normalize avatar path handling consistently across handlers.

---

### F-11 Low-Medium bug: Error handling gaps in group conversation participant inserts

Category: Transactional reliability bug

Static evidence:

- Participant insertion operations are executed without checked errors in startGroupConversation flow around:
  - internal/messaging/handler.go:312, 318

Impact:

- Conversation creation can partially succeed with incomplete membership.

Remediation:

1. Check each ExecContext result.
2. Abort and rollback on first insert failure.
3. Add tests for partial-failure transactional behavior.

## 4) Additional Notes

- No obvious SQL injection was observed in reviewed handlers; most SQL usage is parameterized.
- This assessment focused on this application only, per request.
- This report includes previously identified issues and adds runtime exploit evidence from pentesting.

## 5) Prioritized Remediation Plan

Phase 0 (immediate, block release):

1. Implement CSRF protections globally (F-01).
2. Fix SSRF guardrails for link preview fetcher (F-02).
3. Enforce object-level auth for feed attachment access (F-03).
4. Remove open redirect behavior (F-04).

Phase 1 (short-term hardening):

1. Lock down secrets/defaults/port exposure (F-05).
2. Restrict avatar endpoint access (F-06).
3. Add login throttling and lockout policy (F-07).

Phase 2 (stability and defense-in-depth):

1. Add upload body cap in chat path (F-08).
2. Enforce Secure cookie policy in prod profile (F-09).
3. Fix notification avatar URL bug (F-10).
4. Tighten transaction error handling in messaging group creation (F-11).

## 6) Pentest Evidence Snapshot

Key validated exploit outcomes during live test:

1. CSRF-like request with malicious Origin/Referer changed state and created connection row.
2. User2 fetched User1 private feed attachment directly and received HTTP 200 with file bytes.
3. Internal URL preview entry was created for http://localhost:8080/login in link_previews.
4. Bookmark add endpoint issued 303 redirect to external evil.example URL from Referer.
5. Avatar endpoint returned HTTP 200 without authentication.
6. Repeated login failures showed no lockout/rate limiting behavior.
7. Login Set-Cookie lacked Secure attribute in runtime.
8. notifications/recent returned malformed sender_avatar value /avatar//avatar.
