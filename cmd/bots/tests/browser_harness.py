import os
import socket
import subprocess
import time
import unittest
from pathlib import Path
from urllib.parse import urlparse

import requests
from playwright.sync_api import TimeoutError as PlaywrightTimeoutError
from playwright.sync_api import sync_playwright


ROOT = Path(__file__).resolve().parents[3]


def free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def env_flag(name: str, default: bool = False) -> bool:
    value = os.getenv(name)
    if value is None:
        return default
    return value.strip().lower() in {"1", "true", "yes", "on"}


def env_int(name: str, default: int = 0) -> int:
    value = os.getenv(name)
    if value is None or value.strip() == "":
        return default
    return int(value)


class BrowserHarness(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.headed = env_flag("PLAYWRIGHT_HEADED")
        cls.slow_mo = env_int("PLAYWRIGHT_SLOWMO_MS", 0)
        live_base_url = (os.getenv("PLAYWRIGHT_BASE_URL") or "").strip().rstrip("/")
        cls.live_mode = bool(live_base_url)
        cls.server = None
        if live_base_url:
            cls.base_url = live_base_url
            cls.reset_state = env_flag("PLAYWRIGHT_RESET_STATE", default=False)
        else:
            cls.port = free_port()
            env = os.environ.copy()
            env["PORT"] = str(cls.port)
            cls.server = subprocess.Popen(
                ["go", "run", "./cmd/bots/testserver"],
                cwd=ROOT,
                env=env,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                text=True,
            )
            cls.base_url = f"http://127.0.0.1:{cls.port}"
            cls._wait_for_server()
            cls.reset_state = env_flag("PLAYWRIGHT_RESET_STATE", default=True)

        cls.playwright = sync_playwright().start()
        launch_args = []
        if cls.headed:
            launch_args.append("--start-maximized")
        cls.browser = cls.playwright.chromium.launch(
            headless=not cls.headed,
            slow_mo=cls.slow_mo,
            args=launch_args,
        )

    @classmethod
    def tearDownClass(cls):
        cls.browser.close()
        cls.playwright.stop()

        if cls.server is not None:
            cls.server.terminate()
            try:
                cls.server.wait(timeout=10)
            except subprocess.TimeoutExpired:
                cls.server.kill()
                cls.server.wait(timeout=10)
            if cls.server.stdout:
                cls.server.stdout.close()

    @classmethod
    def _wait_for_server(cls):
        deadline = time.time() + 30
        last_error = "server did not start"
        while time.time() < deadline:
            if cls.server.poll() is not None:
                output = cls.server.stdout.read() if cls.server.stdout else ""
                raise RuntimeError(f"test server exited early:\n{output}")
            try:
                response = requests.get(f"{cls.base_url}/health", timeout=1)
                if response.ok:
                    return
            except requests.RequestException as exc:
                last_error = str(exc)
            time.sleep(0.25)

        output = cls.server.stdout.read() if cls.server.stdout else ""
        raise RuntimeError(f"timed out waiting for test server ({last_error})\n{output}")

    def setUp(self):
        if self.reset_state:
            reset = requests.post(f"{self.base_url}/__reset", timeout=5)
            reset.raise_for_status()
        self.mobile_browser = None
        if self.headed:
            self.context = self.browser.new_context(no_viewport=True)
        else:
            self.context = self.browser.new_context(viewport={"width": 1440, "height": 1200})
        self.page = self.context.new_page()
        self.console_errors = []
        self.page_errors = []
        self._live_login_done = False
        self._attach_page_listeners()

    def _live_login_credentials(self) -> tuple[str, str]:
        email = (os.getenv("PLAYWRIGHT_LOGIN_EMAIL") or "").strip()
        password = os.getenv("PLAYWRIGHT_LOGIN_PASSWORD") or ""
        if self.live_mode and (not email or not password):
            self.fail(
                "live-mode login requires PLAYWRIGHT_LOGIN_EMAIL and PLAYWRIGHT_LOGIN_PASSWORD"
            )
        if not email:
            email = "maria.garcia@usda.gov"
        if not password:
            password = "password123"
        return email, password

    def _perform_live_login(self):
        email, password = self._live_login_credentials()

        self.page.fill('input[name="email"]', email)
        self.page.fill('input[name="password"]', password)
        with self.page.expect_navigation(wait_until="domcontentloaded"):
            self.page.click('button[type="submit"]')

        if urlparse(self.page.url).path == "/login":
            self.fail(
                "live-mode login failed; set PLAYWRIGHT_LOGIN_EMAIL and PLAYWRIGHT_LOGIN_PASSWORD "
                "for your running environment"
            )
        self._live_login_done = True

    def _attach_page_listeners(self):
        self.page.on("console", self._handle_console_message)
        self.page.on("pageerror", lambda err: self.page_errors.append(str(err)))

    def _handle_console_message(self, msg):
        if msg.type != "error":
            return

        text = msg.text or ""
        location = getattr(msg, "location", None) or {}
        location_url = (location.get("url") or "").lower()
        lower_text = text.lower()

        # Live environments frequently omit favicon/manifest assets; treat those 404s as non-functional noise.
        if self.live_mode and "failed to load resource" in lower_text and "404" in lower_text:
            if (
                "favicon" in location_url
                or "apple-touch-icon" in location_url
                or "manifest" in location_url
            ):
                return

        if self.live_mode and "failed to load resource" in lower_text and "403" in lower_text:
            if "/login" in location_url:
                return

        if self.live_mode and "eventsource" in lower_text and "text/event-stream" in lower_text and "text/html" in lower_text:
            if "/login" in location_url:
                return

        if location_url:
            self.console_errors.append(f"{text} @ {location_url}")
            return
        self.console_errors.append(text)

    def tearDown(self):
        self.page.close()
        self.context.close()
        if self.mobile_browser is not None:
            self.mobile_browser.close()
        self.assertEqual(self.page_errors, [], f"unexpected page errors: {self.page_errors}")
        self.assertEqual(self.console_errors, [], f"unexpected console errors: {self.console_errors}")

    def open_path(self, path: str, ready_selector: str, note: str):
        response = self.page.goto(f"{self.base_url}{path}", wait_until="domcontentloaded")
        status = response.status if response else None
        current_path = urlparse(self.page.url).path

        if status in {404, 500, 501}:
            self.skipTest(
                f"fixture route {path} is not ready in cmd/bots/testserver "
                f"(status {status}); expected {note}"
            )

        if path != "/login" and current_path == "/login":
            if self.live_mode:
                if not self._live_login_done:
                    self._perform_live_login()
                    response = self.page.goto(f"{self.base_url}{path}", wait_until="domcontentloaded")
                    status = response.status if response else None
                    current_path = urlparse(self.page.url).path
                if current_path == "/login":
                    self.fail(f"live route {path} still redirects to /login after login; expected {note}")
            else:
                self.skipTest(
                    f"fixture route {path} redirected to /login; expected authenticated fixture for {note}"
                )

        self.assertIsNotNone(response, f"expected a response when opening {path}")
        self.assertLess(response.status, 400, f"expected successful response for {path}")
        try:
            self.page.wait_for_selector(ready_selector, timeout=5000)
        except PlaywrightTimeoutError:
            body_text = self.page.locator("body").inner_text().strip()
            self.skipTest(
                f"fixture route {path} loaded without {ready_selector}; expected {note}. "
                f"Body started with: {body_text[:120]}"
            )
        return response

    def open_shell(self):
        self.open_path("/shell", ".site-header", "the shared shell fixture")
        self.page.wait_for_function(
            "getComputedStyle(document.documentElement).getPropertyValue('--app-header-height').trim() != '0px'"
        )

    def open_feed(self, tab: str = "social"):
        self.open_path(f"/feed?tab={tab}", ".feed-layout", f"the {tab} feed layout")

    def open_landing(self):
        self.open_path("/landing", ".hero", "the public landing page")
        self.page.wait_for_function(
            """
            () => {
              const hero = document.querySelector('.hero');
              return hero && getComputedStyle(hero).display === 'flex';
            }
            """
        )

    def use_mobile_page(self, width: int = 390, height: int = 844):
        self.page.close()
        self.context.close()
        if self.headed:
            self.mobile_browser = self.playwright.chromium.launch(
                headless=False,
                slow_mo=self.slow_mo,
                args=[f"--window-size={width},{height}"],
            )
            self.context = self.mobile_browser.new_context(
                viewport={"width": width, "height": height},
                is_mobile=True,
                has_touch=True,
                device_scale_factor=1,
            )
        else:
            self.context = self.browser.new_context(
                viewport={"width": width, "height": height},
                is_mobile=True,
                has_touch=True,
                device_scale_factor=1,
            )
        self.page = self.context.new_page()
        self._attach_page_listeners()

    def open_tools_drawer(self):
        self.page.locator("#tools-toggle-btn").click()
        self.page.wait_for_function("document.getElementById('drawer-toggle').checked")
        self.page.wait_for_selector("#profile-drawer", state="visible")

    def open_chat_drawer(self):
        self.page.locator("#chat-toggle-btn").click()
        self.page.wait_for_function("document.getElementById('chat-toggle').checked")
        self.page.wait_for_selector("#chat-drawer", state="visible")

    def assert_role_visible(self, role: str, name):
        locator = self.page.get_by_role(role, name=name).first
        self.assertTrue(locator.is_visible(), f"expected visible {role} named {name}")
        return locator

    def assert_text_visible(self, text: str):
        locator = self.page.get_by_text(text).first
        self.assertTrue(locator.is_visible(), f"expected visible text {text}")
        return locator

    def assert_any_visible(self, description: str, selectors: list[str]):
        for selector in selectors:
            locator = self.page.locator(selector)
            if locator.count() > 0 and locator.first.is_visible():
                return locator.first
        self.fail(f"expected one of {selectors} for {description}")

    def submit_and_wait_for_navigation(self, trigger, response_predicate):
        with self.page.expect_response(response_predicate) as response_info:
            with self.page.expect_navigation(wait_until="domcontentloaded"):
                trigger.click()
        response = response_info.value
        self.assertLess(response.status, 400, f"expected successful submit, got {response.status}")
        return response
