import unittest

try:
    from .browser_harness import BrowserHarness
except ImportError:
    from browser_harness import BrowserHarness


class ThemeContrastTests(BrowserHarness):
    ROUTES = [
        ("/login", 'form[action="/login"]', "login page"),
        ("/feed?role=employee", ".feed-layout", "feed"),
        ("/search?role=employee", ".search-layout", "search"),
        ("/postings/posting-1?role=manager", ".posting-layout", "posting detail"),
        ("/admin/dashboard?role=admin", ".dash-layout", "admin dashboard"),
        ("/admin/users?role=admin", ".admin-users-table", "admin users"),
        ("/admin/foia?role=admin", ".foia-layout", "foia"),
        ("/aup?role=employee", ".aup-layout", "aup"),
        ("/orgchart?role=admin", ".orgchart-layout", "org chart"),
    ]

    def set_theme(self, dark: bool):
        self.page.evaluate(
            """
            (isDark) => {
              document.documentElement.classList.toggle('dark-mode', isDark);
              document.body.classList.toggle('dark-mode', isDark);
              localStorage.setItem('dark', isDark ? 'true' : 'false');
            }
            """,
            dark,
        )
        self.page.wait_for_function(
            """
            (isDark) =>
              document.documentElement.classList.contains('dark-mode') === isDark &&
              document.body.classList.contains('dark-mode') === isDark
            """,
            arg=dark,
        )
        self.page.wait_for_timeout(10)

    def contrast_failures(self, min_ratio: float = 4.5):
        return self.page.evaluate(
            r"""
            ({ minRatio }) => {
              function parseColor(value) {
                if (!value) return null;
                const m = value.match(/rgba?\(([^)]+)\)/i);
                if (!m) return null;
                const parts = m[1].split(',').map((p) => p.trim());
                const r = Number(parts[0]);
                const g = Number(parts[1]);
                const b = Number(parts[2]);
                const a = parts.length > 3 ? Number(parts[3]) : 1;
                if ([r, g, b, a].some((n) => Number.isNaN(n))) return null;
                return { r, g, b, a };
              }

              function effectiveBackground(el) {
                let node = el;
                while (node) {
                  const bg = parseColor(getComputedStyle(node).backgroundColor);
                  if (bg && bg.a >= 0.98) {
                    return { bg, source: node.tagName + (node.className ? '.' + node.className : '') };
                  }
                  node = node.parentElement;
                }
                const bodyBg = parseColor(getComputedStyle(document.body).backgroundColor);
                if (bodyBg) {
                  return { bg: bodyBg, source: 'BODY' };
                }
                return { bg: { r: 255, g: 255, b: 255, a: 1 }, source: 'fallback' };
              }

              function channel(v) {
                const x = v / 255;
                return x <= 0.03928 ? x / 12.92 : Math.pow((x + 0.055) / 1.055, 2.4);
              }

              function luminance(c) {
                return 0.2126 * channel(c.r) + 0.7152 * channel(c.g) + 0.0722 * channel(c.b);
              }

              function contrastRatio(fg, bg) {
                const l1 = luminance(fg);
                const l2 = luminance(bg);
                const hi = Math.max(l1, l2);
                const lo = Math.min(l1, l2);
                return (hi + 0.05) / (lo + 0.05);
              }

              function visible(el) {
                const style = getComputedStyle(el);
                if (style.display === 'none' || style.visibility === 'hidden' || Number(style.opacity) === 0) {
                  return false;
                }
                if (el.hasAttribute('hidden') || el.getAttribute('aria-hidden') === 'true') {
                  return false;
                }
                const rect = el.getBoundingClientRect();
                return rect.width > 0 && rect.height > 0;
              }

              function labelFor(el) {
                const t = (el.innerText || el.textContent || el.value || '').replace(/\s+/g, ' ').trim();
                if (t) return t.slice(0, 70);
                const aria = el.getAttribute('aria-label') || el.getAttribute('title') || el.getAttribute('id') || el.tagName;
                return String(aria).slice(0, 70);
              }

              const controls = Array.from(document.querySelectorAll('a, button, summary, [role="button"], input[type="submit"], input[type="button"]'));
              const failures = [];

              for (const el of controls) {
                if (!visible(el)) continue;
                if (el.closest('.sr-only') || el.classList.contains('skip-link')) continue;

                const text = labelFor(el);
                if (!text) continue;

                const fg = parseColor(getComputedStyle(el).color);
                if (!fg) continue;
                if (fg.r === 0 && fg.g === 0 && fg.b === 238) continue;
                const bgInfo = effectiveBackground(el);
                const bg = bgInfo.bg;
                const ratio = contrastRatio({ r: fg.r, g: fg.g, b: fg.b }, { r: bg.r, g: bg.g, b: bg.b });

                if (ratio < minRatio) {
                  failures.push({
                    selector: el.tagName.toLowerCase() + (el.id ? '#' + el.id : ''),
                    text,
                    ratio: Number(ratio.toFixed(2)),
                  });
                }
              }

              return failures;
            }
            """,
            {"minRatio": min_ratio},
        )

    def test_actionable_elements_have_readable_contrast_in_both_themes(self):
        all_failures = []

        for route, ready_selector, note in self.ROUTES:
            self.open_path(route, ready_selector, note)

            for dark in (False, True):
                self.page.evaluate(
                    """
                    (isDark) => {
                      localStorage.setItem('dark', isDark ? 'true' : 'false');
                    }
                    """,
                    dark,
                )
                self.page.reload(wait_until="domcontentloaded")
                self.page.wait_for_selector(ready_selector, timeout=5000)
                self.set_theme(dark)
                failures = self.contrast_failures(min_ratio=3.0)
                if failures:
                    all_failures.append(
                        {
                            "route": route,
                            "theme": "dark" if dark else "light",
                            "failures": failures[:10],
                        }
                    )

        self.assertEqual([], all_failures, f"contrast failures detected: {all_failures}")


if __name__ == "__main__":
    unittest.main()
