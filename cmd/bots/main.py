"""
USDA JobPortal - Reactive Bot Framework

20 bots that monitor and respond to activity on the platform.
Each bot has a unique personality, response delay, and interaction style.

Usage:
    uv run python main.py                  # Run all bots
    uv run python main.py --bot clark.kent # Run a single bot
"""

import json
import random
import re
import sys
import time
import threading
from pathlib import Path

import requests

# Load config files
CONFIG_DIR = Path(__file__).parent
with open(CONFIG_DIR / "config.json") as f:
    config = json.load(f)
with open(CONFIG_DIR / "responses.json") as f:
    responses = json.load(f)

POLL_INTERVAL = 5


class Bot:
    def __init__(self, bot_config):
        self.email = bot_config["email"]
        self.password = bot_config["password"]
        self.delay_min = bot_config["delay_min"]
        self.delay_max = bot_config["delay_max"]
        self.style = bot_config["style"]
        self.name = self.email.split("@")[0].replace(".", " ").title()
        self.session = requests.Session()
        self.logged_in = False
        self.seen_posts = set()
        self.seen_messages = set()
        self.seen_notifications = set()
        self.running = True
        self.base_url = config.get("base_url", "http://localhost:8080")

    def log(self, msg):
        print(f"  [{self.name}] {msg}")

    def delay(self):
        time.sleep(random.uniform(self.delay_min, self.delay_max))

    def login(self):
        try:
            self.session.get(f"{self.base_url}/login")
            resp = self.session.post(
                f"{self.base_url}/login",
                data={"email": self.email, "password": self.password},
                allow_redirects=True,
            )
            # Debug: print where we landed
            self.log(f"Login response: status={resp.status_code} url={resp.url}")

            # Success if we landed anywhere except the login page
            if "/login" not in resp.url:
                self.logged_in = True
                self.log("Logged in successfully")
                return True
            self.log("Login failed - still on login page")
            return False
        except Exception as e:
            self.log(f"Login error: {e}")
            return False

    def get_comment(self):
        pool = responses.get(self.style, responses.get("encouraging", ["Great work!"]))
        return random.choice(pool)

    def get_message_reply(self, incoming):
        style_replies = {
            "encouraging": [
                "That sounds great! Happy to support.",
                "I agree. Let us make it happen.",
                "Good thinking. Keep me posted.",
            ],
            "analytical": [
                "Interesting. What data supports this?",
                "Let me review and get back to you.",
                "Have we run this through compliance?",
            ],
            "enthusiastic": [
                "YES! Count me in!",
                "This is going to be amazing!",
                "I love it! When do we start?!",
            ],
            "sarcastic": [
                "Fine. But I am not doing the paperwork.",
                "Sounds like a plan. A terrible, wonderful plan.",
                "Oh brilliant. More work.",
            ],
            "punny": [
                "I am totally caught in this web of excitement!",
                "Consider me attached to this thread.",
                "No bugs detected in that logic.",
            ],
            "elegant": [
                "A wise approach. Let us proceed.",
                "I concur. There is strength in this direction.",
                "Well considered.",
            ],
            "bubbly": [
                "Oh my God yes! I mean... professionally, yes!",
                "That is SO exciting! Sorry, I will calm down.",
                "I am here for ALL of this.",
            ],
            "nerdy": [
                "Ooh I already have a name for this.",
                "The specs look good. Can I tinker?",
                "This is giving me major project vibes.",
            ],
            "formal": [
                "Acknowledged. Proceeding as discussed.",
                "Understood. The timeline is acceptable.",
                "Roger that.",
            ],
            "empathetic": [
                "I can tell this means a lot. Happy to help.",
                "That makes sense. How are you feeling about the timeline?",
                "Let us work through it together.",
            ],
            "eager": [
                "Absolutely! What do you need from me?",
                "Yes! I am ready!",
                "Such a great opportunity!",
            ],
            "impatient": [
                "Got it. Already on it.",
                "Done. What is next?",
                "Can we move faster on this?",
            ],
            "action-oriented": [
                "Copy that. Executing now.",
                "On it. Results by end of day.",
                "Less planning, more doing.",
            ],
            "technical": [
                "Architecture looks solid. I reviewed the specs.",
                "Let me check the logs and get back to you.",
                "I can have a prototype ready tomorrow.",
            ],
            "optimistic": [
                "This is going to be amazing!",
                "Every challenge is an opportunity!",
                "The future is bright!",
            ],
            "political": [
                "We need to consider stakeholder implications.",
                "I will draft a policy brief.",
                "This aligns with Secretary priorities.",
            ],
            "confident": [
                "No problem. I have handled bigger.",
                "Consider it done.",
                "Leave it to me.",
            ],
            "precise": [
                "Processing. Metrics confirm this approach.",
                "System check complete. All green.",
                "Data supports your approach.",
            ],
            "collaborative": [
                "Love this! Want to brainstorm over coffee?",
                "I know people who would be great additions.",
                "Team effort. That is how the best things get built.",
            ],
        }
        replies = style_replies.get(self.style, style_replies["encouraging"])
        return random.choice(replies)

    def check_pending_connections(self):
        try:
            resp = self.session.get(f"{self.base_url}/connections")
            if resp.status_code != 200:
                return

            accept_urls = re.findall(
                r'action="/connections/accept/([^"]+)"', resp.text
            )
            for conn_id in accept_urls:
                self.delay()
                self.session.post(
                    f"{self.base_url}/connections/accept/{conn_id}"
                )
                self.log(f"Accepted connection from {conn_id[:8]}...")
        except Exception:
            pass

    def check_messages(self):
        try:
            resp = self.session.get(f"{self.base_url}/messages/recent")
            if resp.status_code != 200:
                return

            data = resp.json()
            for conv in data.get("recent", []):
                if conv.get("unread", 0) == 0:
                    continue

                conv_id = conv["id"]
                if conv_id in self.seen_messages:
                    continue

                msg_resp = self.session.get(
                    f"{self.base_url}/messages/chat/{conv_id}"
                )
                if msg_resp.status_code != 200:
                    continue

                messages = msg_resp.json()
                if not messages:
                    continue

                if messages[-1].get("is_own"):
                    continue

                self.seen_messages.add(conv_id)
                self.delay()

                reply = self.get_message_reply(messages[-1].get("content", ""))
                self.session.post(
                    f"{self.base_url}/messages/chat/{conv_id}",
                    data={"content": reply},
                )
                self.log(f"Replied to {conv['name']}: {reply[:50]}...")
        except Exception:
            pass

    def check_feed(self):
        try:
            resp = self.session.get(f"{self.base_url}/feed")
            if resp.status_code != 200:
                return

            post_ids = re.findall(r'action="/feed/([^/]+)/like"', resp.text)
            for post_id in post_ids:
                if post_id in self.seen_posts:
                    continue
                self.seen_posts.add(post_id)

                if random.random() < 0.7:
                    self.delay()
                    reaction = random.choice(
                        ["like", "like", "like", "celebrate", "insightful"]
                    )
                    self.session.post(
                        f"{self.base_url}/feed/{post_id}/like",
                        data={"reaction": reaction},
                    )
                    self.log(f"Reacted '{reaction}' to post {post_id[:8]}...")

                if random.random() < 0.2:
                    self.delay()
                    comment = self.get_comment()
                    self.session.post(
                        f"{self.base_url}/feed/{post_id}/comment",
                        data={"content": comment},
                    )
                    self.log(f"Commented: {comment[:50]}...")
        except Exception:
            pass

    def check_notifications(self):
        try:
            resp = self.session.get(f"{self.base_url}/notifications/recent")
            if resp.status_code != 200:
                return

            for notif in resp.json():
                nid = notif.get("id", "")
                if nid in self.seen_notifications or notif.get("read"):
                    continue
                self.seen_notifications.add(nid)
                self.session.post(
                    f"{self.base_url}/notifications/{nid}/read",
                    headers={"Accept": "application/json"},
                )
        except Exception:
            pass

    def run(self):
        if not self.login():
            self.log("Login failed. Stopping.")
            return

        self.log(
            f"Active (style={self.style}, delay={self.delay_min}-{self.delay_max}s)"
        )

        while self.running:
            try:
                self.check_pending_connections()
                self.check_notifications()
                self.check_messages()
                self.check_feed()
                time.sleep(POLL_INTERVAL)
            except KeyboardInterrupt:
                break
            except Exception as e:
                self.log(f"Error: {e}")
                time.sleep(10)

        self.log("Stopped.")


def main():
    bot_filter = None
    args = sys.argv[1:]
    i = 0
    while i < len(args):
        if args[i] == "--bot" and i + 1 < len(args):
            bot_filter = args[i + 1]
            i += 2
        else:
            i += 1

    bot_configs = config.get("bots", [])
    if bot_filter:
        bot_configs = [b for b in bot_configs if bot_filter in b["email"]]

    if not bot_configs:
        print("No bots matched. Available:")
        for b in config.get("bots", []):
            print(f"  {b['email']}")
        return

    print("=== USDA JobPortal Bot Framework ===")
    print(f"Starting {len(bot_configs)} bots...")
    print(f"Poll interval: {POLL_INTERVAL}s")
    print()

    bots = [Bot(bc) for bc in bot_configs]
    threads = []
    for bot in bots:
        t = threading.Thread(target=bot.run, daemon=True, name=bot.name)
        t.start()
        threads.append(t)
        time.sleep(0.5)

    print(f"\n{len(bots)} bots running. Ctrl+C to stop.\n")

    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        print("\nStopping bots...")
        for bot in bots:
            bot.running = False
        time.sleep(2)
        print("Done.")


if __name__ == "__main__":
    main()
