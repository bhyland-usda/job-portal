from pathlib import Path

try:
    from .browser_harness import BrowserHarness
except ImportError:
    from browser_harness import BrowserHarness


class VisualSmokeTests(BrowserHarness):
    def _screenshot_dir(self) -> Path:
        root = Path(__file__).resolve().parents[3]
        out = root / ".copilot-artifacts" / "visual-smoke"
        out.mkdir(parents=True, exist_ok=True)
        return out

    def _set_theme(self, dark: bool) -> dict:
        return self.page.evaluate(
            """
            (isDark) => {
                document.documentElement.classList.toggle('dark-mode', isDark);
                localStorage.setItem('dark', isDark ? 'true' : 'false');
                const root = getComputedStyle(document.documentElement);
                const body = getComputedStyle(document.body);
                return {
                    isDarkClass: document.documentElement.classList.contains('dark-mode'),
                    bg: body.backgroundColor,
                    text: body.color,
                    varBg: root.getPropertyValue('--bg').trim(),
                    varCard: root.getPropertyValue('--bg-card').trim(),
                };
            }
            """,
            dark,
        )

    def _assert_visual_delta(self, light_sig: dict, dark_sig: dict, label: str):
        self.assertFalse(light_sig.get("isDarkClass"), f"{label}: expected light mode class off")
        self.assertTrue(dark_sig.get("isDarkClass"), f"{label}: expected dark mode class on")

        changed = (
            light_sig.get("bg") != dark_sig.get("bg")
            or light_sig.get("text") != dark_sig.get("text")
            or light_sig.get("varBg") != dark_sig.get("varBg")
            or light_sig.get("varCard") != dark_sig.get("varCard")
        )
        self.assertTrue(changed, f"{label}: expected measurable light/dark visual delta")

    def _assert_no_horizontal_overflow(self, label: str):
        overflow = self.page.evaluate(
            """
            () => {
                const viewport = window.innerWidth || document.documentElement.clientWidth;
                const docWidth = Math.max(
                    document.documentElement.scrollWidth,
                    document.body ? document.body.scrollWidth : 0,
                );
                return { viewport, docWidth };
            }
            """
        )
        self.assertLessEqual(
            overflow["docWidth"],
            overflow["viewport"] + 1,
            f"{label}: horizontal overflow detected (viewport={overflow['viewport']} docWidth={overflow['docWidth']})",
        )

    def _capture_light_dark_pair(self, path: str, ready_selector: str, slug: str):
        out = self._screenshot_dir()
        self.open_path(path, ready_selector, f"visual smoke route {path}")

        light_sig = self._set_theme(False)
        self.page.screenshot(path=str(out / f"{slug}-light.png"), full_page=True)

        dark_sig = self._set_theme(True)
        self.page.screenshot(path=str(out / f"{slug}-dark.png"), full_page=True)

        self._assert_visual_delta(light_sig, dark_sig, slug)
        self._assert_no_horizontal_overflow(slug)

    def test_shell_visual_smoke(self):
        self._capture_light_dark_pair("/shell", ".site-header", "shell")

        self.assertTrue(self.page.locator(".navbar-app-link").first.is_visible())
        self.assertTrue(self.page.locator(".site-footer").first.is_visible())

    def test_feed_visual_smoke(self):
        self._capture_light_dark_pair("/feed?tab=social", ".feed-layout", "feed-social")

        self.assertGreater(self.page.locator(".feed-post").count(), 0)
        self.assertTrue(self.page.locator(".feed-composer").first.is_visible())

    def test_landing_visual_smoke(self):
        self._capture_light_dark_pair("/landing", ".hero", "landing")

        self.assertTrue(self.page.locator(".hero").first.is_visible())
        self.assertTrue(self.page.get_by_role("link", name="Sign In").first.is_visible())
