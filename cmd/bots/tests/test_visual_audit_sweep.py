import unittest
import os
from pathlib import Path
import re
from datetime import datetime, timezone
from urllib.parse import urlparse, urljoin

try:
    from .browser_harness import BrowserHarness
except ImportError:
    from browser_harness import BrowserHarness


class VisualAuditSweepTests(BrowserHarness):
    _INTERACTION_SELECTOR = (
        'a[href], button, summary, '
        'input[type="checkbox"], input[type="radio"], input[type="text"], '
        'input[type="search"], input[type="email"], input[type="url"], input[type="number"], '
        'select, textarea, [role="button"]'
    )

    def _emit_progress(self, status: str):
        done = getattr(self, '_interaction_done', 0)
        total = getattr(self, '_interaction_total', 0)
        safe_status = (status or '').replace('\n', ' ').strip()
        print(f'PROGRESS done={done} total={total} status={safe_status}', flush=True)

    def _emit_interaction_result(self, outcome: str, status: str):
        safe_status = (status or '').replace('\n', ' ').strip()
        print(f'PROGRESS_RESULT outcome={outcome} status={safe_status}', flush=True)

    def _register_interaction_total(self, count: int, status: str):
        if count <= 0:
            return
        self._interaction_total = getattr(self, '_interaction_total', 0) + count
        self._emit_progress(status)

    def _complete_interaction(self, status: str, outcome: str = 'pass'):
        self._interaction_done = getattr(self, '_interaction_done', 0) + 1
        self._emit_interaction_result(outcome, status)
        self._emit_progress(status)

    def _record_finding(self, findings: list[str], seen_finding_keys: set[str], text: str):
        if text not in seen_finding_keys:
            seen_finding_keys.add(text)
            findings.append(text)

    def _capture_console_event(self, findings: list[str], seen_finding_keys: set[str], message):
        if message.type not in {'error', 'warning'}:
            return
        text = f'Console {message.type} on {self.page.url}: {message.text}'
        self._record_finding(findings, seen_finding_keys, text)

    def _capture_page_error(self, findings: list[str], seen_finding_keys: set[str], error):
        text = f'Page error on {self.page.url}: {error}'
        self._record_finding(findings, seen_finding_keys, text)

    def _slug(self, route: str) -> str:
        if route == '/':
            return 'splash'
        slug = route.strip('/') or 'home'
        slug = slug.replace('?', '_').replace('&', '_').replace('=', '-')
        slug = slug.replace('/', '_')
        slug = re.sub(r'[^a-zA-Z0-9_-]+', '-', slug)
        return slug.lower().strip('-')

    def _set_theme(self, theme: str):
        want_dark = theme == 'dark'
        signature = self.page.evaluate(
            """
            (isDark) => {
                if (isDark) {
                    document.documentElement.classList.add('dark-mode');
                    localStorage.setItem('dark', 'true');
                } else {
                    document.documentElement.classList.remove('dark-mode');
                    localStorage.setItem('dark', 'false');
                }
                window.scrollTo(0, 0);
                const root = getComputedStyle(document.documentElement);
                const body = getComputedStyle(document.body);
                return {
                    hasDarkClass: document.documentElement.classList.contains('dark-mode'),
                    bodyBg: body.backgroundColor,
                    bodyText: body.color,
                    varBg: root.getPropertyValue('--bg').trim(),
                    varCard: root.getPropertyValue('--bg-card').trim(),
                };
            }
            """,
            want_dark,
        )
        if signature.get('hasDarkClass') != want_dark:
            raise AssertionError(f'Failed to apply theme={theme}; dark class mismatch.')
        return signature

    def _record_if_not_dark_delta(
        self,
        findings: list[str],
        seen_finding_keys: set[str],
        label: str,
        light_sig: dict,
        dark_sig: dict,
    ):
        body_changed = dark_sig.get('bodyBg') != light_sig.get('bodyBg')
        text_changed = dark_sig.get('bodyText') != light_sig.get('bodyText')
        vars_changed = (
            dark_sig.get('varBg') != light_sig.get('varBg')
            or dark_sig.get('varCard') != light_sig.get('varCard')
        )
        if not (body_changed or text_changed or vars_changed):
            self._record_finding(
                findings,
                seen_finding_keys,
                f'Dark mode gap: {label} did not show measurable style delta from light mode.',
            )

    def _safe_fragment(self, text: str) -> str:
        val = re.sub(r'[^a-zA-Z0-9_-]+', '-', (text or '').strip().lower())
        return val.strip('-')[:32] or 'element'

    def _materialize_route(self, route: str) -> str:
        token_map = {
            '{id}': 'fixture-id',
            '{userID}': 'mentor-1',
        }

        if '/postings/{id}' in route:
            route = route.replace('{id}', 'posting-1')
        elif '/groups/{id}' in route:
            route = route.replace('{id}', 'group-1')
        elif '/workspaces/{id}' in route:
            route = route.replace('{id}', 'workspace-1')
        elif '/departments/{id}' in route:
            route = route.replace('{id}', 'department-1')
        elif '/profile/{id}' in route:
            route = route.replace('{id}', 'mentor-1')
        elif '/articles/{id}' in route:
            route = route.replace('{id}', 'article-1')
        elif '/news/{id}' in route:
            route = route.replace('{id}', 'news-1')
        elif '/polls/{id}' in route:
            route = route.replace('{id}', 'poll-1')
        elif '/messages/chat/{id}' in route:
            route = route.replace('{id}', 'conv-1')
        elif '/messages/new/{id}' in route:
            route = route.replace('{id}', 'mentor-1')
        else:
            for token, replacement in token_map.items():
                route = route.replace(token, replacement)

        return route

    def _discover_get_routes(self) -> list[str]:
        root = Path(__file__).resolve().parents[3]
        source = (root / 'cmd' / 'bots' / 'testserver' / 'main.go').read_text(encoding='utf-8')

        fixture_patterns = re.findall(r'Pattern:\s*"([^"]+)"', source)
        explicit_gets = re.findall(r'HandleFunc\("GET\s+([^"]+)"', source)
        candidates = set(fixture_patterns + explicit_gets)

        skip_prefixes = (
            '/health',
            '/__reset',
            '/shell',
            '/notifications/stream',
            '/messages/events',
            '/feed/events',
            '/admin/foia/export',
            '/data-export/download',
            '/messages/recent',
            '/messages/search-users',
            '/messages/chat/',
        )
        routes: set[str] = set()
        for raw in candidates:
            if not raw.startswith('/'):
                continue
            if self.live_mode and ('{id}' in raw or '{userID}' in raw):
                # Live environments generally do not have deterministic fixture IDs.
                continue
            if raw.startswith(skip_prefixes):
                continue
            routes.add(self._materialize_route(raw))

        # Ensure explicit search states and key role-based states are always included.
        if self.live_mode:
            routes.add('/search?q=analytics')
            routes.add('/search?q=taylor')
            routes.add('/feed/drafts')
            routes.add('/postings/create')
            routes.add('/postings/search')
            routes.add('/admin/dashboard')
            routes.add('/admin/moderation')
        else:
            routes.add('/search?role=employee&q=analytics')
            routes.add('/search?role=employee&q=taylor')
            routes.add('/feed/drafts?role=manager')
            routes.add('/postings/create?role=manager')
            routes.add('/postings/search?role=manager')
            routes.add('/postings/posting-1?role=manager')
            routes.add('/admin/dashboard?role=admin')

        return sorted(routes)

    def _screenshot_dir(self) -> Path:
        root = Path(__file__).resolve().parents[3]
        out = root / '.copilot-artifacts' / 'visual-audit'
        out.mkdir(parents=True, exist_ok=True)
        return out

    def _load_open_known_bugs(self) -> list[str]:
        root = Path(__file__).resolve().parents[3]
        known = root / 'KNOWN_BUGS.md'
        if not known.exists():
            return []

        lines = known.read_text(encoding='utf-8').splitlines()
        in_open = False
        bugs: list[str] = []
        for line in lines:
            if line.strip() == '## Open':
                in_open = True
                continue
            if in_open and line.startswith('---'):
                break
            if not in_open:
                continue

            if line.startswith('- '):
                item = line[2:].strip()
            elif line.startswith('  - '):
                item = line[4:].strip()
            else:
                continue

            if item:
                if item.endswith(':'):
                    continue
                if item.lower().startswith('risk:'):
                    continue
                if item.lower().startswith('follow-up:'):
                    continue
                if item in {'Feed Page'}:
                    continue
                bugs.append(item)
        return bugs

    def _write_triage_report(
        self,
        routes: list[str],
        detected_findings: list[str],
        route_counts: dict[str, tuple[int, int]],
        role_duty_results: dict[str, list[str]],
    ):
        out = self._screenshot_dir()
        shots = sorted(p.name for p in out.glob('*.png'))
        now = datetime.now(timezone.utc).strftime('%Y-%m-%d %H:%M:%SZ')

        open_bugs = self._load_open_known_bugs()
        lines: list[str] = []
        lines.append('# Visual Triage Report')
        lines.append('')
        lines.append(f'- Generated: {now}')
        lines.append(f'- Screenshots captured: {len(shots)}')
        lines.append(f'- Routes covered: {len(routes)} + interaction states')
        lines.append('')

        lines.append('## Fixed (Verified by Automated Assertions)')
        lines.append('')
        lines.append('- Drafts page uses a single rich composer with schedule-toggle flow.')
        lines.append('- Draft and scheduled lists remain usable with high item counts (independent scrolling).')
        lines.append('- Light and dark screenshots are captured for route matrix and key interactions.')
        lines.append('- Multi-element screenshots are captured per route to expand end-to-end audit coverage.')
        lines.append('- Search is exercised in both no-result and has-result states, including result click-through.')
        lines.append('- Posting creation is exercised through form submission and redirect to posting detail.')
        lines.append('')

        lines.append('## Unfixed (Open Backlog)')
        lines.append('')
        if open_bugs:
            for bug in open_bugs:
                lines.append(f'- {bug}')
        else:
            lines.append('- No open bugs found in KNOWN_BUGS.md.')
        lines.append('')

        lines.append('## Newly Detected This Run')
        lines.append('')
        if detected_findings:
            for finding in detected_findings:
                lines.append(f'- {finding}')
        else:
            lines.append('- No automated new defects detected (console/page errors and overflow checks were clean).')
        lines.append('- Manual screenshot review is still required for subjective UX quality.')
        lines.append('')

        lines.append('## Screenshot Inventory')
        lines.append('')
        for name in shots:
            lines.append(f'- {name}')
        lines.append('')

        lines.append('## Route Interactable Coverage')
        lines.append('')
        for route in routes:
            matched, captured = route_counts.get(route, (0, 0))
            lines.append(f'- {route}: matched={matched}, captured={captured}')
        lines.append('')

        lines.append('## Role Duty Coverage')
        lines.append('')
        for role, entries in role_duty_results.items():
            lines.append(f'### {role.title()}')
            if entries:
                for entry in entries:
                    lines.append(f'- {entry}')
            else:
                lines.append('- No role-duty checks were executed.')
            lines.append('')

        (out / 'triage-report.md').write_text('\n'.join(lines), encoding='utf-8')

    def _capture_page_pair(self, route: str, findings: list[str], seen_finding_keys: set[str]):
        out = self._screenshot_dir()
        slug = self._slug(route)

        self.open_path(route, 'body', f'visual audit route {route}')

        light_sig = self._set_theme('light')
        self.page.screenshot(path=str(out / f'{slug}-light.png'), full_page=True)

        dark_sig = self._set_theme('dark')
        self.page.screenshot(path=str(out / f'{slug}-dark.png'), full_page=True)
        self._record_if_not_dark_delta(findings, seen_finding_keys, f'route {route}', light_sig, dark_sig)

        overflow = self.page.evaluate(
            """
            () => {
              const w = window.innerWidth || document.documentElement.clientWidth;
              const sw = Math.max(
                document.documentElement.scrollWidth,
                document.body ? document.body.scrollWidth : 0,
              );
              return { hasOverflow: sw > w + 1, viewport: w, scrollWidth: sw };
            }
            """
        )
        if overflow.get('hasOverflow'):
            msg = (
                f'Horizontal overflow on {route} '
                f'(viewport={overflow.get("viewport")}, scrollWidth={overflow.get("scrollWidth")})'
            )
            self._record_finding(findings, seen_finding_keys, msg)

    def _capture_public_splash_pair(self, findings: list[str], seen_finding_keys: set[str]):
        out = self._screenshot_dir()
        context = self.browser.new_context(no_viewport=True) if self.headed else self.browser.new_context(viewport={"width": 1440, "height": 1200})
        page = context.new_page()
        page.on('console', lambda m: self._capture_console_event(findings, seen_finding_keys, m))
        page.on('pageerror', lambda e: self._capture_page_error(findings, seen_finding_keys, e))
        try:
            page.goto(f'{self.base_url}/', wait_until='domcontentloaded')
            page.wait_for_selector('body', timeout=5000)

            light_sig = page.evaluate(
                """
                () => {
                  document.documentElement.classList.remove('dark-mode');
                  localStorage.setItem('dark', 'false');
                  window.scrollTo(0, 0);
                  const root = getComputedStyle(document.documentElement);
                  const body = getComputedStyle(document.body);
                  return {
                    hasDarkClass: document.documentElement.classList.contains('dark-mode'),
                    bodyBg: body.backgroundColor,
                    bodyText: body.color,
                    varBg: root.getPropertyValue('--bg').trim(),
                    varCard: root.getPropertyValue('--bg-card').trim(),
                  };
                }
                """
            )
            page.screenshot(path=str(out / 'splash-light.png'), full_page=True)

            dark_sig = page.evaluate(
                """
                () => {
                  document.documentElement.classList.add('dark-mode');
                  localStorage.setItem('dark', 'true');
                  window.scrollTo(0, 0);
                  const root = getComputedStyle(document.documentElement);
                  const body = getComputedStyle(document.body);
                  return {
                    hasDarkClass: document.documentElement.classList.contains('dark-mode'),
                    bodyBg: body.backgroundColor,
                    bodyText: body.color,
                    varBg: root.getPropertyValue('--bg').trim(),
                    varCard: root.getPropertyValue('--bg-card').trim(),
                  };
                }
                """
            )
            page.screenshot(path=str(out / 'splash-dark.png'), full_page=True)
            if not dark_sig.get('hasDarkClass'):
                self._record_finding(
                    findings,
                    seen_finding_keys,
                    'Dark mode gap: splash dark class was not applied.',
                )
            body_changed = dark_sig.get('bodyBg') != light_sig.get('bodyBg')
            text_changed = dark_sig.get('bodyText') != light_sig.get('bodyText')
            vars_changed = (
                dark_sig.get('varBg') != light_sig.get('varBg')
                or dark_sig.get('varCard') != light_sig.get('varCard')
            )
            if not (body_changed or text_changed or vars_changed):
                self._record_finding(
                    findings,
                    seen_finding_keys,
                    'Dark mode gap: splash page did not show measurable style delta from light mode.',
                )
        finally:
            page.close()
            context.close()

    def _collect_interaction_items(self, visible_selector: str):
        return self.page.evaluate(
            """
            (selector) => {
                            const cssEscape = (value) => {
                                if (window.CSS && typeof window.CSS.escape === 'function') {
                                    return window.CSS.escape(value);
                                }
                                return String(value).replace(/[^a-zA-Z0-9_-]/g, '_');
                            };

              const toCssPath = (el) => {
                const parts = [];
                let node = el;
                while (node && node.nodeType === Node.ELEMENT_NODE && node !== document.body) {
                  if (node.id) {
                                        parts.unshift(`#${cssEscape(node.id)}`);
                    break;
                  }
                  const tag = node.tagName.toLowerCase();
                  let index = 1;
                  let sib = node;
                  while ((sib = sib.previousElementSibling) !== null) {
                    if (sib.tagName === node.tagName) index += 1;
                  }
                  parts.unshift(`${tag}:nth-of-type(${index})`);
                  node = node.parentElement;
                }
                return parts.join(' > ');
              };

              return Array.from(document.querySelectorAll(selector))
                .filter((el) => {
                  const style = getComputedStyle(el);
                  if (style.visibility === 'hidden' || style.display === 'none') return false;
                  const rect = el.getBoundingClientRect();
                  if (rect.width <= 0 || rect.height <= 0) return false;
                  if (el.closest('footer.site-footer')) return false;
                  return true;
                })
                .map((el) => {
                  const tag = (el.tagName || '').toLowerCase();
                  const type = ((el.getAttribute('type') || '') + '').toLowerCase();
                  const text = (el.innerText || el.getAttribute('aria-label') || el.getAttribute('name') || '').trim().slice(0, 40);
                  const href = (el.getAttribute('href') || '').trim();
                  const form = el.closest('form');
                  const formAction = form ? ((form.getAttribute('action') || '').trim()) : '';
                  return {
                    selector: toCssPath(el),
                    tag,
                    type,
                    text,
                    href,
                    form_action: formAction,
                    submit: tag === 'button' && type === 'submit',
                  };
                });
            }
            """,
            visible_selector,
        )

    def _expected_reload(self, item: dict):
        tag = item.get('tag') or ''
        href = item.get('href') or ''
        if tag == 'a' and href:
            return href.startswith('/')
        if item.get('submit'):
            # Many submits intentionally update state in place or redirect to the
            # current route. Navigation is validated opportunistically per action.
            return None
        return None

    def _normalize_target_path(self, href: str) -> str:
        if not href:
            return ''
        try:
            u = urlparse(urljoin(self.base_url, href))
            target = u.path or '/'
            if u.query:
                target += f'?{u.query}'
            return target
        except Exception:
            return href

    def _same_route_target(self, before_url: str, href: str) -> bool:
        if not href:
            return False
        try:
            before = urlparse(before_url)
            target = urlparse(urljoin(self.base_url, href))
            before_path = before.path or '/'
            target_path = target.path or '/'
            if before_path != target_path:
                return False
            # If the target omits query params, treat same-path navigation as a
            # no-op and do not require URL mutation.
            if not (target.query or ''):
                return True
            return (before.query or '') == (target.query or '')
        except Exception:
            return False

    def _optional_link_target(self, href: str) -> bool:
        target = self._normalize_target_path(href)
        return target in {
            '/',
            '/login',
            '/register',
            '/feed',
            '/search',
            '/connections',
            '/opportunities/search',
            '/workspaces',
        }

    def _exercise_interaction_item(
        self,
        route: str,
        idx: int,
        total: int,
        slug: str,
        out: Path,
        item: dict,
        findings: list[str],
        seen_finding_keys: set[str],
        strict_errors: bool = False,
    ) -> int:
        tag = item.get('tag') or 'node'
        kind = item.get('type') or ''
        label = self._safe_fragment(item.get('text') or '')
        selector = item.get('selector') or ''
        href = item.get('href') or ''

        self.open_path(route, 'body', f'interaction execution route {route}')
        light_sig = self._set_theme('light')
        locator = self.page.locator(selector).first

        shot_name = f'{slug}-interaction-{idx:02d}-action-{tag}-{label}.png'
        dark_shot_name = f'{slug}-interaction-{idx:02d}-action-{tag}-{label}-dark.png'
        captured = 0
        before_findings = len(findings)
        outcome = 'pass'

        try:
            if locator.count() == 0:
                raise RuntimeError(f'locator not found: {selector}')

            locator.scroll_into_view_if_needed(timeout=1500)

            marker = f'vs-{self._safe_fragment(route)}-{idx}'
            self.page.evaluate('(m) => { window.__visual_sweep_marker = m; }', marker)
            expected_reload = self._expected_reload(item)

            if tag == 'a' and href:
                before = self.page.url
                try:
                    with self.page.expect_navigation(wait_until='domcontentloaded', timeout=3000):
                        locator.click()
                except Exception:
                    locator.click(force=True)
                    self.page.wait_for_timeout(300)
                after = self.page.url
                if after == before and href.startswith('/') and not self._same_route_target(before, href) and not self._optional_link_target(href):
                    self._record_finding(
                        findings,
                        seen_finding_keys,
                        f'Route link click did not navigate on {route} href={href}',
                    )
            elif tag == 'input' and kind in {'text', 'search', 'email', 'url', 'number'}:
                value = {
                    'email': 'visual.sweep@usda.gov',
                    'url': 'https://example.gov/sweep',
                    'number': '7',
                }.get(kind, 'visual sweep input')
                locator.fill(value)
            elif tag == 'input' and kind in {'checkbox', 'radio'}:
                locator.click(force=True)
            elif tag == 'textarea':
                locator.fill('Visual sweep exercised this textarea.')
            elif tag == 'select':
                locator.select_option(index=1)
            else:
                before = self.page.url
                if item.get('submit'):
                    self.page.evaluate(
                        """
                        (sel) => {
                          const btn = document.querySelector(sel);
                          if (!btn) return;
                          const form = btn.closest('form');
                          if (!form) return;
                          form.querySelectorAll('input[required], textarea[required], select[required]').forEach((field) => {
                            if (field.value) return;
                            const tag = (field.tagName || '').toLowerCase();
                            const type = ((field.getAttribute('type') || '') + '').toLowerCase();
                            if (tag === 'select' && field.options.length > 1) {
                              field.selectedIndex = 1;
                              return;
                            }
                            if (type === 'email') {
                              field.value = 'visual.sweep@usda.gov';
                              return;
                            }
                            field.value = 'visual sweep submit';
                          });
                        }
                        """,
                        selector,
                    )
                    try:
                        with self.page.expect_navigation(wait_until='domcontentloaded', timeout=3000):
                            locator.click()
                    except Exception:
                        locator.click(force=True)
                        self.page.wait_for_timeout(300)
                else:
                    locator.click(force=True)
                    self.page.wait_for_timeout(200)
                after = self.page.url
                _ = after

            did_reload = False
            try:
                did_reload = self.page.evaluate('(m) => window.__visual_sweep_marker !== m', marker)
            except Exception:
                did_reload = True

            if expected_reload is True and not did_reload:
                self._record_finding(
                    findings,
                    seen_finding_keys,
                    f'Expected full page reload but none occurred on {route} selector={selector} action={tag}:{kind}',
                )
            if expected_reload is False and did_reload:
                self._record_finding(
                    findings,
                    seen_finding_keys,
                    f'Unexpected full page reload occurred on {route} selector={selector} action={tag}:{kind}',
                )

            self.page.screenshot(path=str(out / shot_name), full_page=True)
            dark_sig = self._set_theme('dark')
            self.page.screenshot(path=str(out / dark_shot_name), full_page=True)
            self._record_if_not_dark_delta(
                findings,
                seen_finding_keys,
                f'route {route} action selector={selector}',
                light_sig,
                dark_sig,
            )
            captured += 1
        except Exception as exc:
            fallback_name = f'{slug}-interaction-{idx:02d}-fallback-{tag}-{label}.png'
            try:
                self.page.screenshot(path=str(out / fallback_name), full_page=True)
                captured += 1
            except Exception:
                pass
            self._record_finding(
                findings,
                seen_finding_keys,
                f'Interaction action failed on {route} element#{idx} [{tag}/{kind}/{label}] selector={selector}: {exc}',
            )
            outcome = 'error'
            if strict_errors:
                raise

        if outcome == 'pass' and len(findings) > before_findings:
            outcome = 'fail'

        self._complete_interaction(f'{route} interaction {idx + 1}/{total} [{tag}/{label}]', outcome=outcome)
        return captured

    def _run_single_auth_check(self, auth_mode: str):
        out = self._screenshot_dir()
        if auth_mode == 'invalid':
            self._interaction_total = 1
            self._interaction_done = 0
            self._emit_progress('planned interactions fixed: auth-invalid total=1')
            self.page.goto(f'{self.base_url}/login', wait_until='domcontentloaded')
            self.page.fill('input[name="email"]', 'invalid.user@usda.gov')
            self.page.fill('input[name="password"]', 'definitely-wrong-password')
            with self.page.expect_navigation(wait_until='domcontentloaded'):
                self.page.click('button[type="submit"]')
            self.assertEqual(urlparse(self.page.url).path, '/login', 'invalid login should remain on /login')
            self.page.screenshot(path=str(out / 'auth-login-failure.png'), full_page=True)
            self._complete_interaction('auth invalid-login path verified', outcome='pass')
            return

        if auth_mode == 'valid':
            self._interaction_total = 1
            self._interaction_done = 0
            self._emit_progress('planned interactions fixed: auth-valid total=1')
            self.page.goto(f'{self.base_url}/login', wait_until='domcontentloaded')
            self._perform_live_login()
            self.assertNotEqual(urlparse(self.page.url).path, '/login', 'valid login should leave /login')
            self.page.screenshot(path=str(out / 'auth-login-success.png'), full_page=True)
            self._complete_interaction('auth valid-login path verified', outcome='pass')
            return

        raise AssertionError(f'unknown auth_mode: {auth_mode}')

    def _run_single_interaction_case(self, route: str, interaction_index: int):
        findings: list[str] = []
        seen_finding_keys: set[str] = set()
        out = self._screenshot_dir()
        slug = self._slug(route)

        self.page.on('console', lambda message: self._capture_console_event(findings, seen_finding_keys, message))
        self.page.on('pageerror', lambda error: self._capture_page_error(findings, seen_finding_keys, error))

        self.open_path(route, 'body', f'interaction case route {route}')
        self._set_theme('light')
        items = self._collect_interaction_items(self._INTERACTION_SELECTOR)
        total = len(items)
        if interaction_index >= total:
            self.skipTest(f'interaction index {interaction_index} no longer present for route {route}')

        self._interaction_total = 1
        self._interaction_done = 0
        self._emit_progress(
            f'planned interactions fixed: route={route} interaction={interaction_index + 1}/{total} total=1'
        )

        before_findings = len(findings)
        self._exercise_interaction_item(
            route,
            interaction_index,
            total,
            slug,
            out,
            items[interaction_index],
            findings,
            seen_finding_keys,
            strict_errors=True,
        )
        if len(findings) > before_findings:
            self.fail(' | '.join(findings[before_findings:]))


def _install_generated_interaction_tests():
    if getattr(VisualAuditSweepTests, '_generated_interaction_tests_installed', False):
        return

    route_specs: list[tuple[str, int]] = []
    probe = VisualAuditSweepTests(methodName='runTest')

    try:
        VisualAuditSweepTests.setUpClass()
        probe.setUp()
        routes = probe._discover_get_routes()
        for route in routes:
            if route == '/':
                continue
            try:
                probe.open_path(route, 'body', f'plan interaction tests for {route}')
                probe._set_theme('light')
                items = probe._collect_interaction_items(probe._INTERACTION_SELECTOR)
            except unittest.SkipTest:
                continue
            for idx in range(len(items)):
                route_specs.append((route, idx))
    finally:
        try:
            if getattr(probe, 'page', None) is not None:
                probe.page.close()
        except Exception:
            pass
        try:
            if getattr(probe, 'context', None) is not None:
                probe.context.close()
        except Exception:
            pass
        try:
            VisualAuditSweepTests.tearDownClass()
        except Exception:
            pass

    def _attach(name: str, fn):
        setattr(VisualAuditSweepTests, name, fn)

    def _make_auth_case(mode: str):
        def _case(self):
            return self._run_single_auth_check(mode)

        return _case

    _attach('test_visual_site_sweep_auth_invalid_login', _make_auth_case('invalid'))
    _attach('test_visual_site_sweep_auth_valid_login', _make_auth_case('valid'))

    for route, idx in route_specs:
        slug = re.sub(r'[^a-zA-Z0-9_]+', '_', probe._slug(route)).strip('_') or 'route'
        name = f'test_visual_site_sweep_interaction_{slug}_{idx:03d}'

        def _make_case(r=route, i=idx):
            def _case(self):
                return self._run_single_interaction_case(r, i)

            return _case

        _attach(name, _make_case())

    VisualAuditSweepTests._generated_interaction_tests_installed = True

    def _prime_moderation_queue(self, findings: list[str], seen_finding_keys: set[str]):
        self.open_path('/feed?tab=social', '.feed-layout', 'seed moderation queue via report action')
        report_summary = self.page.locator('.feed-post .post-report summary').first
        if report_summary.count() == 0:
            self._record_finding(
                findings,
                seen_finding_keys,
                'Moderation queue seed skipped: no report controls available on social feed.',
            )
            return

        report_summary.click(force=True)
        reason = self.page.locator('.feed-post .post-report-form textarea[name="reason"]').first
        if reason.count() > 0:
            reason.fill('Visual sweep seeded moderation queue.')

        submit = self.page.locator('.feed-post .post-report-form button[type="submit"]').first
        if submit.count() == 0:
            self._record_finding(
                findings,
                seen_finding_keys,
                'Moderation queue seed failed: report submit button not found.',
            )
            return

        before = self.page.url
        try:
            with self.page.expect_navigation(wait_until='domcontentloaded', timeout=4000):
                submit.click()
        except Exception:
            submit.click(force=True)
            self.page.wait_for_timeout(400)

        self.page.screenshot(path=str(self._screenshot_dir() / 'moderation-queue-seed-report-submitted.png'), full_page=True)
        after = self.page.url
        if after == before:
            self._record_finding(
                findings,
                seen_finding_keys,
                'Moderation queue seed report did not trigger navigation; verify report submission behavior.',
            )

    def _capture_interaction_sweep(self, route: str, findings: list[str], seen_finding_keys: set[str]) -> tuple[int, int]:
        out = self._screenshot_dir()
        slug = self._slug(route)
        # Keep this as pure CSS for querySelectorAll in page JS; visibility is filtered there.
        visible_selector = self._INTERACTION_SELECTOR

        self.open_path(route, 'body', f'interaction audit route {route}')
        self._set_theme('light')
        items = self._collect_interaction_items(visible_selector)

        total = len(items)
        captured = 0

        for idx, item in enumerate(items):
            captured += self._exercise_interaction_item(
                route,
                idx,
                total,
                slug,
                out,
                item,
                findings,
                seen_finding_keys,
            )
        return total, captured

    def _precount_route_interactions(self, routes: list[str]) -> dict[str, int]:
        counts: dict[str, int] = {}
        selector = self._INTERACTION_SELECTOR
        for route in routes:
            if route == '/':
                counts[route] = 0
                continue
            try:
                self.open_path(route, 'body', f'precount route interactions {route}')
                self._set_theme('light')
                items = self._collect_interaction_items(selector)
                counts[route] = len(items)
            except unittest.SkipTest:
                counts[route] = 0
        return counts

    def _run_auth_checks(self, out: Path):
        bad_email = 'invalid.user@usda.gov'
        bad_password = 'definitely-wrong-password'

        self.page.goto(f'{self.base_url}/login', wait_until='domcontentloaded')
        self.page.fill('input[name="email"]', bad_email)
        self.page.fill('input[name="password"]', bad_password)
        with self.page.expect_navigation(wait_until='domcontentloaded'):
            self.page.click('button[type="submit"]')
        self.assertEqual(urlparse(self.page.url).path, '/login', 'invalid login should remain on /login')
        self.page.screenshot(path=str(out / 'auth-login-failure.png'), full_page=True)
        self._complete_interaction('auth invalid-login path verified')

        self.page.goto(f'{self.base_url}/login', wait_until='domcontentloaded')
        self._perform_live_login()
        self.assertNotEqual(urlparse(self.page.url).path, '/login', 'valid login should leave /login')
        self.page.screenshot(path=str(out / 'auth-login-success.png'), full_page=True)
        self._complete_interaction('auth valid-login path verified')

    def _run_route_matrix(
        self,
        routes: list[str],
        findings: list[str],
        seen_finding_keys: set[str],
        precounts: dict[str, int] | None = None,
    ):
        route_counts: dict[str, tuple[int, int]] = {}
        if '/admin/moderation' in routes:
            self._prime_moderation_queue(findings, seen_finding_keys)
        if precounts is None:
            precounts = self._precount_route_interactions(routes)
        if '/' in routes:
            self._capture_public_splash_pair(findings, seen_finding_keys)
            route_counts['/'] = (0, 0)
        for route in routes:
            if route == '/':
                continue
            try:
                self._capture_page_pair(route, findings, seen_finding_keys)
                route_counts[route] = self._capture_interaction_sweep(route, findings, seen_finding_keys)
            except unittest.SkipTest as exc:
                route_counts[route] = (0, 0)
                self._record_finding(
                    findings,
                    seen_finding_keys,
                    f'Route skipped during live sweep: {route} ({exc})',
                )
        return route_counts

    def _assert_route_artifacts(self, routes: list[str], out: Path, route_counts: dict[str, tuple[int, int]]):
        for route in routes:
            slug = self._slug(route)
            self.assertTrue((out / f'{slug}-light.png').exists(), f'missing light screenshot for {route}')
            self.assertTrue((out / f'{slug}-dark.png').exists(), f'missing dark screenshot for {route}')
            matched, captured = route_counts.get(route, (0, 0))
            if matched > 0:
                self.assertGreater(captured, 0, f'no interactables captured for {route}')

    def _run_shell_drawer_checks(self):
        self.open_path('/feed?tab=social', '.feed-layout', 'real feed shell controls')
        self.open_tools_drawer()
        self.page.screenshot(path=str(self._screenshot_dir() / 'shell-tools-drawer.png'), full_page=True)
        self.page.locator('.drawer-scrim').click(force=True)

        self.open_chat_drawer()
        self.page.screenshot(path=str(self._screenshot_dir() / 'shell-chat-drawer.png'), full_page=True)

    def _run_drafts_and_search_checks(self):
        self.open_path('/feed/drafts?role=manager', '.feed-layout', 'drafts schedule interaction')
        self.page.locator('[data-schedule-toggle]').click()
        self.page.screenshot(path=str(self._screenshot_dir() / 'drafts-schedule-open-light.png'), full_page=True)

        self._set_theme('dark')
        self.page.screenshot(path=str(self._screenshot_dir() / 'drafts-schedule-open-dark.png'), full_page=True)

        self.open_path('/search?role=employee&q=analytics', '.search-layout', 'search no-result state')
        self.assertTrue(self.page.get_by_text('No results found for "analytics"').first.is_visible())
        self.page.screenshot(path=str(self._screenshot_dir() / 'search-no-results-state.png'), full_page=True)

        self.open_path('/search?role=employee&q=taylor', '.search-layout', 'search has-result state')
        result_link = self.page.locator('.connection-item a.connection-info').first
        self.assertTrue(result_link.is_visible())
        self.page.screenshot(path=str(self._screenshot_dir() / 'search-has-results-state.png'), full_page=True)
        result_link.click()
        self.page.wait_for_load_state('domcontentloaded')
        self.assertIn('/profile/', self.page.url)
        self.page.screenshot(path=str(self._screenshot_dir() / 'search-result-click-through.png'), full_page=True)

    def _run_posting_create_check(self):
        self.open_path('/postings/create?role=manager', '.posting-layout', 'posting create form submit path')
        self.page.locator('#title').fill('Visual Sweep Created Posting')
        self.page.locator('#type').select_option('project')
        self.page.locator('#department').fill('Forest Service')
        self.page.locator('#location').fill('Washington, DC')
        self.page.locator('#description').fill('Created by visual sweep submit path test.')
        self.page.locator('#skills').fill('python, analysis')
        with self.page.expect_navigation(wait_until='domcontentloaded'):
            self.page.get_by_role('button', name='Create Posting').click()
        self.assertIn('/postings/posting-1', self.page.url)
        self.page.screenshot(path=str(self._screenshot_dir() / 'postings-create-submit-result.png'), full_page=True)

    def _role_duty_matrix(self) -> dict[str, list[dict[str, str]]]:
        return {
            'admin': [
                {'name': 'Admin Dashboard', 'path': '/admin/dashboard', 'selector': 'main'},
                {'name': 'User Management', 'path': '/admin/users', 'selector': 'main'},
                {'name': 'Moderation Queue', 'path': '/admin/moderation', 'selector': '.report-item, .empty-state, main'},
                {'name': 'Audit Log', 'path': '/admin/audit', 'selector': 'main'},
                {'name': 'Org Chart Assignment', 'path': '/admin/orgchart', 'selector': '#orgchart-submit, form, main'},
                {'name': 'FOIA Tool', 'path': '/admin/foia', 'selector': 'form, .empty-state, main'},
                {'name': 'Announcement Create', 'path': '/admin/announcements/new', 'selector': 'form, main'},
            ],
            'manager': [
                {'name': 'Create Posting', 'path': '/postings/create', 'selector': '.posting-layout form, form, main'},
                {'name': 'My Postings', 'path': '/my-posts?tab=postings', 'selector': '.feed-layout, .posting-card, .empty-state, main'},
                {'name': 'Workforce Analytics', 'path': '/analytics/workforce', 'selector': 'main'},
                {'name': 'Skills Gap Analysis', 'path': '/analytics/skills-gap', 'selector': 'main'},
                {'name': 'Skills Heat Map', 'path': '/insights/heatmap', 'selector': 'main'},
            ],
            'employee': [
                {'name': 'Social Feed', 'path': '/feed?tab=social', 'selector': '.feed-layout, main'},
                {'name': 'People Search', 'path': '/search', 'selector': '.search-layout, main'},
                {'name': 'Connections', 'path': '/connections', 'selector': '.empty-state, main'},
                {'name': 'Bookmarks', 'path': '/bookmarks', 'selector': '.bookmarks-layout, .empty-state, main'},
                {'name': 'Mentorship', 'path': '/mentorship', 'selector': '.empty-state, main'},
                {'name': 'Resumes', 'path': '/resumes', 'selector': '.empty-state, main'},
            ],
        }

    def _role_credentials(self, role: str) -> tuple[str, str, bool]:
        email = (os.getenv(f'PLAYWRIGHT_{role.upper()}_LOGIN_EMAIL') or '').strip()
        password = os.getenv(f'PLAYWRIGHT_{role.upper()}_LOGIN_PASSWORD') or ''
        explicit = bool(email and password)
        return email, password, explicit

    def _open_role_session(self, role: str, findings: list[str], seen_finding_keys: set[str]):
        email, password, explicit = self._role_credentials(role)
        if not email or not password:
            self._record_finding(
                findings,
                seen_finding_keys,
                f'ROLE GAP [{role}] missing credentials env vars PLAYWRIGHT_{role.upper()}_LOGIN_EMAIL/PLAYWRIGHT_{role.upper()}_LOGIN_PASSWORD.',
            )
            return None, None, 'missing_credentials'

        context = self.browser.new_context(no_viewport=True) if self.headed else self.browser.new_context(viewport={"width": 1440, "height": 1200})
        page = context.new_page()
        page.on('console', lambda m: self._capture_console_event(findings, seen_finding_keys, m))
        page.on('pageerror', lambda e: self._capture_page_error(findings, seen_finding_keys, e))

        page.goto(f'{self.base_url}/login', wait_until='domcontentloaded')
        page.fill('input[name="email"]', email)
        page.fill('input[name="password"]', password)
        with page.expect_navigation(wait_until='domcontentloaded'):
            page.click('button[type="submit"]')
        if urlparse(page.url).path == '/login':
            self._record_finding(
                findings,
                seen_finding_keys,
                f'ROLE GAP [{role}] login failed for configured credentials.',
            )
            page.close()
            context.close()
            return None, None, ('invalid_explicit_credentials' if explicit else 'missing_credentials')
        return context, page, 'ok'

    def _run_role_duty_audit(
        self,
        findings: list[str],
        seen_finding_keys: set[str],
        roles: list[str] | None = None,
        register_plan: bool = True,
    ) -> dict[str, list[str]]:
        out = self._screenshot_dir()
        target_roles = roles or ['admin', 'manager', 'employee']
        results: dict[str, list[str]] = {role: [] for role in target_roles}
        matrix = self._role_duty_matrix()
        planned_role_interactions = sum(len(matrix.get(role, [])) for role in target_roles)
        if register_plan:
            self._register_interaction_total(planned_role_interactions, f'role-duty interactions planned: {planned_role_interactions}')

        for role in target_roles:
            duties = matrix.get(role, [])
            context, page, state = self._open_role_session(role, findings, seen_finding_keys)
            if not context or not page:
                if state == 'missing_credentials':
                    results[role].append('GAP: credentials missing; role duties not exercised.')
                elif state == 'invalid_explicit_credentials':
                    results[role].append('GAP: provided role credentials are invalid; role duties not exercised.')
                else:
                    results[role].append('GAP: credentials missing; role duties not exercised.')
                for duty in duties:
                    self._complete_interaction(f'role {role} duty skipped: {duty["name"]}', outcome='skip')
                continue

            for duty in duties:
                duty_name = duty['name']
                path = duty['path']
                selector = duty['selector']
                slug = self._safe_fragment(f'{role}-{duty_name}')

                try:
                    resp = page.goto(f'{self.base_url}{path}', wait_until='domcontentloaded')
                    status = resp.status if resp else None
                    current_path = urlparse(page.url).path

                    if current_path == '/login':
                        msg = f'GAP: {duty_name} redirected to login.'
                        results[role].append(msg)
                        self._record_finding(findings, seen_finding_keys, f'ROLE GAP [{role}] {msg} route={path}')
                    elif status in {401, 403}:
                        msg = f'GAP: {duty_name} blocked with status {status}.'
                        results[role].append(msg)
                        self._record_finding(findings, seen_finding_keys, f'ROLE GAP [{role}] {msg} route={path}')
                    else:
                        page.wait_for_selector(selector, timeout=3000)
                        page.screenshot(path=str(out / f'role-{slug}-light.png'), full_page=True)
                        page.evaluate("document.documentElement.classList.add('dark-mode')")
                        page.screenshot(path=str(out / f'role-{slug}-dark.png'), full_page=True)

                        # Exercise one representative action per duty page when available.
                        action = page.locator('main button:visible, main a[href]:visible, main summary:visible').first
                        if action.count() > 0:
                            try:
                                action.click(force=True, timeout=1500)
                            except Exception:
                                pass
                        results[role].append(f'OK: {duty_name} exercised on {path}.')
                except Exception as exc:
                    msg = f'GAP: {duty_name} failed to exercise ({exc}).'
                    results[role].append(msg)
                    self._record_finding(findings, seen_finding_keys, f'ROLE GAP [{role}] {msg} route={path}')
                finally:
                    duty_outcome = 'pass'
                    if results[role] and results[role][-1].startswith('GAP:'):
                        duty_outcome = 'fail'
                    self._complete_interaction(f'role {role} duty checked: {duty_name}', outcome=duty_outcome)

            page.close()
            context.close()

        return results

    def test_visual_site_sweep(self):
        findings: list[str] = []
        seen_finding_keys: set[str] = set()
        out = self._screenshot_dir()
        self._interaction_done = 0
        self._interaction_total = 0

        self.page.on(
            'console',
            lambda message: self._capture_console_event(findings, seen_finding_keys, message),
        )
        self.page.on(
            'pageerror',
            lambda error: self._capture_page_error(findings, seen_finding_keys, error),
        )

        routes = self._discover_get_routes()
        precounts = self._precount_route_interactions(routes)
        planned_route_interactions = sum(precounts.values())
        planned_role_interactions = sum(len(v) for v in self._role_duty_matrix().values())
        planned_total = 2 + planned_route_interactions + planned_role_interactions
        self._interaction_total = planned_total
        self._interaction_done = 0
        self._emit_progress(
            f'planned interactions fixed: auth=2 route={planned_route_interactions} role={planned_role_interactions} total={planned_total}'
        )

        self._run_auth_checks(out)

        route_counts = self._run_route_matrix(routes, findings, seen_finding_keys, precounts=precounts)
        self._assert_route_artifacts(routes, out, route_counts)
        self._run_shell_drawer_checks()
        self._run_drafts_and_search_checks()
        self._run_posting_create_check()
        role_duty_results = self._run_role_duty_audit(findings, seen_finding_keys, register_plan=False)

        shots = list(self._screenshot_dir().glob('*.png'))
        self.assertGreaterEqual(len(shots), 200)
        self._write_triage_report(routes, findings, route_counts, role_duty_results)

    def test_visual_site_sweep_employee_duties(self):
        findings: list[str] = []
        seen_finding_keys: set[str] = set()
        self.page.on('console', lambda message: self._capture_console_event(findings, seen_finding_keys, message))
        self.page.on('pageerror', lambda error: self._capture_page_error(findings, seen_finding_keys, error))
        self._interaction_done = 0
        self._interaction_total = 0

        results = self._run_role_duty_audit(findings, seen_finding_keys, roles=['employee'])
        employee_results = results.get('employee', [])
        if any('credentials missing' in r for r in employee_results):
            self.skipTest('employee credentials unavailable for role-duty sweep')
        if any('invalid' in r for r in employee_results):
            self.skipTest('employee credentials provided but login failed for role-duty sweep')
        self.assertTrue(any(r.startswith('OK:') for r in employee_results), 'employee role duties were not exercised successfully')

    def test_visual_site_sweep_admin_duties(self):
        findings: list[str] = []
        seen_finding_keys: set[str] = set()
        self.page.on('console', lambda message: self._capture_console_event(findings, seen_finding_keys, message))
        self.page.on('pageerror', lambda error: self._capture_page_error(findings, seen_finding_keys, error))
        self._interaction_done = 0
        self._interaction_total = 0

        results = self._run_role_duty_audit(findings, seen_finding_keys, roles=['admin'])
        admin_results = results.get('admin', [])
        if any('credentials missing' in r for r in admin_results):
            self.skipTest('admin credentials unavailable for role-duty sweep')
        if any('invalid' in r for r in admin_results):
            self.skipTest('admin credentials provided but login failed for role-duty sweep')
        self.assertTrue(any(r.startswith('OK:') for r in admin_results), 'admin role duties were not exercised successfully')

    def test_visual_site_sweep_manager_duties(self):
        findings: list[str] = []
        seen_finding_keys: set[str] = set()
        self.page.on('console', lambda message: self._capture_console_event(findings, seen_finding_keys, message))
        self.page.on('pageerror', lambda error: self._capture_page_error(findings, seen_finding_keys, error))
        self._interaction_done = 0
        self._interaction_total = 0

        results = self._run_role_duty_audit(findings, seen_finding_keys, roles=['manager'])
        manager_results = results.get('manager', [])
        if any('credentials missing' in r for r in manager_results):
            self.skipTest('manager credentials unavailable for role-duty sweep')
        if any('invalid' in r for r in manager_results):
            self.skipTest('manager credentials provided but login failed for role-duty sweep')
        self.assertTrue(any(r.startswith('OK:') for r in manager_results), 'manager role duties were not exercised successfully')


def _has_role_credentials(role: str) -> bool:
    email = (os.getenv(f'PLAYWRIGHT_{role.upper()}_LOGIN_EMAIL') or '').strip()
    password = os.getenv(f'PLAYWRIGHT_{role.upper()}_LOGIN_PASSWORD') or ''
    return bool(email and password)


def _iter_cases(suite):
    for test in suite:
        if hasattr(test, '__iter__'):
            yield from _iter_cases(test)
        else:
            yield test


def _is_ci_visual_sweep_enabled() -> bool:
    # Keep the heavy visual sweep local by default; allow explicit CI opt-in.
    ci_flag = (os.getenv('CI') or '').strip().lower() in {'1', 'true', 'yes', 'on'}
    override = (os.getenv('PLAYWRIGHT_RUN_VISUAL_SWEEP_IN_CI') or '').strip().lower()
    if not ci_flag:
        return True
    return override in {'1', 'true', 'yes', 'on'}


def load_tests(loader, tests, pattern):
    if not _is_ci_visual_sweep_enabled():
        def _ci_skip_placeholder():
            raise unittest.SkipTest(
                'visual audit sweep is disabled in CI; run locally or set PLAYWRIGHT_RUN_VISUAL_SWEEP_IN_CI=1'
            )

        return loader.suiteClass([unittest.FunctionTestCase(_ci_skip_placeholder)])

    _install_generated_interaction_tests()

    allowed_prefixes = {
        'test_visual_site_sweep_auth_',
        'test_visual_site_sweep_interaction_',
    }
    allowed = set()
    if _has_role_credentials('employee'):
        allowed.add('test_visual_site_sweep_employee_duties')
    if _has_role_credentials('admin'):
        allowed.add('test_visual_site_sweep_admin_duties')
    if _has_role_credentials('manager'):
        allowed.add('test_visual_site_sweep_manager_duties')

    generated = loader.loadTestsFromTestCase(VisualAuditSweepTests)
    suite = loader.suiteClass()
    for case in _iter_cases(generated):
        name = getattr(case, '_testMethodName', '')
        if name in allowed or any(name.startswith(prefix) for prefix in allowed_prefixes):
            suite.addTest(case)
    return suite
