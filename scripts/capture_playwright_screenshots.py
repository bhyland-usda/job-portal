import argparse
import os
import socket
import subprocess
import time
from pathlib import Path

import requests
from playwright.sync_api import sync_playwright


ROOT = Path(__file__).resolve().parents[1]
ARTIFACTS_DIR = ROOT / ".copilot-artifacts" / "visual-audit"


def free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def wait_for_server(base_url: str, server: subprocess.Popen[str]) -> None:
    deadline = time.time() + 30
    last_error = "server did not start"
    while time.time() < deadline:
        if server.poll() is not None:
            output = server.stdout.read() if server.stdout else ""
            raise RuntimeError(f"test server exited early:\n{output}")
        try:
            response = requests.get(f"{base_url}/health", timeout=1)
            if response.ok:
                return
        except requests.RequestException as exc:
            last_error = str(exc)
        time.sleep(0.25)

    output = server.stdout.read() if server.stdout else ""
    raise RuntimeError(f"timed out waiting for test server ({last_error})\n{output}")


def reset_state(base_url: str) -> None:
    response = requests.post(f"{base_url}/__reset", timeout=5)
    response.raise_for_status()


def capture_page(page, base_url: str, path: str, output_path: Path, actions=None) -> None:
    reset_state(base_url)
    page.goto(f"{base_url}{path}", wait_until="domcontentloaded")
    page.wait_for_timeout(800)
    if actions:
        actions(page)
        page.wait_for_timeout(400)
    page.screenshot(path=str(output_path), full_page=True)


def main() -> None:
    parser = argparse.ArgumentParser(description="Capture visual audit screenshots for Playwright fixtures.")
    parser.add_argument("--headed", action="store_true", help="Show the browser while capturing screenshots.")
    parser.add_argument("--slowmo-ms", type=int, default=0, help="Playwright slow motion in milliseconds.")
    parser.add_argument(
        "--output-dir",
        default=str(ARTIFACTS_DIR),
        help="Directory to write screenshots into.",
    )
    args = parser.parse_args()

    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    port = free_port()
    env = os.environ.copy()
    env["PORT"] = str(port)
    server = subprocess.Popen(
        ["go", "run", "./cmd/bots/testserver"],
        cwd=ROOT,
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    base_url = f"http://127.0.0.1:{port}"

    try:
        wait_for_server(base_url, server)

        with sync_playwright() as playwright:
            launch_args = []
            if args.headed:
                launch_args.append("--start-maximized")
            browser = playwright.chromium.launch(
                headless=not args.headed,
                slow_mo=args.slowmo_ms,
                args=launch_args,
            )
            if args.headed:
                page = browser.new_page(no_viewport=True)
            else:
                page = browser.new_page(viewport={"width": 1440, "height": 1200})

            capture_plan = [
                ("01-landing.png", "/landing", None),
                ("02-shell-manager.png", "/shell?role=manager", None),
                (
                    "03-shell-tools-drawer.png",
                    "/shell?role=manager",
                    lambda p: (
                        p.locator("#tools-toggle-btn").click(),
                        p.wait_for_function("document.getElementById('drawer-toggle').checked"),
                    ),
                ),
                (
                    "04-shell-notifications.png",
                    "/shell?role=employee",
                    lambda p: (
                        p.locator("#notif-trigger").click(),
                        p.wait_for_selector(".notif-item"),
                    ),
                ),
                (
                    "05-shell-chat-modal.png",
                    "/shell?role=employee",
                    lambda p: (
                        p.locator("#chat-toggle-btn").click(),
                        p.wait_for_function("document.getElementById('chat-toggle').checked"),
                        p.locator("#chat-new-btn").click(),
                        p.wait_for_selector("#new-chat-modal.active"),
                    ),
                ),
                ("06-feed-social.png", "/feed?tab=social&role=employee", None),
                ("07-feed-postings.png", "/feed?tab=postings&role=manager", None),
                ("08-connections.png", "/connections?role=employee", None),
                ("09-group.png", "/groups/group-1?role=employee", None),
                ("10-mentorship-find.png", "/mentorship?tab=find&role=employee", None),
                ("11-workspace.png", "/workspaces/workspace-1?role=employee", None),
                ("12-posting.png", "/postings/posting-1?role=manager", None),
                ("13-polls.png", "/polls?role=manager", None),
                ("14-admin-dashboard.png", "/admin/dashboard?role=admin", None),
                ("15-orgchart.png", "/orgchart?role=admin", None),
                ("16-search-diana.png", "/search?role=employee&q=diana", None),
                ("17-workspaces.png", "/workspaces?role=employee", None),
                ("18-notification-preferences.png", "/notifications/preferences?role=employee", None),
                ("19-active-sessions.png", "/settings/sessions?role=employee", None),
                ("20-spotlight.png", "/spotlight?role=employee", None),
                ("21-onboarding.png", "/onboarding?role=employee", None),
                ("22-admin-foia.png", "/admin/foia?role=admin", None),
                ("23-mentorship-active.png", "/mentorship?tab=active&role=employee", None),
                ("24-aup.png", "/aup?role=employee", None),
                ("25-certifications.png", "/certifications?role=employee", None),
                ("26-workforce-analytics.png", "/analytics/workforce?role=manager", None),
                ("27-profile.png", "/profile/me?role=employee", None),
                ("28-resumes.png", "/resume?role=employee", None),
                ("29-accomplishments.png", "/accomplishments?role=employee", None),
                ("30-trending.png", "/feed/trending?role=employee", None),
                ("31-moderation-queue.png", "/admin/moderation?role=admin", None),
            ]

            for filename, path, actions in capture_plan:
                capture_page(page, base_url, path, output_dir / filename, actions)

            browser.close()
    finally:
        server.terminate()
        try:
            server.wait(timeout=10)
        except subprocess.TimeoutExpired:
            server.kill()
            server.wait(timeout=10)
        if server.stdout:
            server.stdout.close()


if __name__ == "__main__":
    main()
