import re

try:
    from .browser_harness import BrowserHarness
except ImportError:
    from browser_harness import BrowserHarness


class SocialSurfaceTests(BrowserHarness):
    def drawer_metrics(self, selector: str):
        return self.page.evaluate(
            """
            (drawerSelector) => {
              const header = document.querySelector('.site-header').getBoundingClientRect();
              const drawer = document.querySelector(drawerSelector).getBoundingClientRect();
              const close = document.querySelector(drawerSelector + ' .drawer-close').getBoundingClientRect();
              return {
                headerBottom: header.bottom,
                drawerTop: drawer.top,
                closeTop: close.top,
                drawerHeight: drawer.height
              };
            }
            """,
            selector,
        )

    def test_shell_primary_nav_stays_compact(self):
        self.open_shell()

        links = self.page.locator(".navbar-app-link")
        self.assertEqual(links.count(), 5)
        self.assertEqual(
            links.all_inner_texts(),
            ["Feed", "Search People", "Network", "Opportunities", "Workspaces"],
        )

    def test_all_tools_drawer_opens_below_header_with_grouped_sections(self):
        self.open_shell()
        self.open_tools_drawer()

        metrics = self.drawer_metrics(".profile-drawer")
        self.assertGreaterEqual(metrics["drawerTop"], metrics["headerBottom"] - 1)
        self.assertGreaterEqual(metrics["closeTop"], metrics["drawerTop"])
        self.assertGreater(metrics["drawerHeight"], 0)

        labels = [
            label.lower()
            for label in self.page.locator(".profile-drawer .drawer-section-label").all_inner_texts()
        ]
        self.assertEqual(
            labels[:6],
            [
                "connect",
                "discover",
                "career",
                "my work",
                "recognition & insights",
                "settings",
            ],
        )

    def test_shell_tools_and_chat_support_keyboard_and_scrim_close(self):
        self.open_shell()
        tools_toggle = self.page.locator("#tools-toggle-btn")

        tools_toggle.focus()
        self.page.keyboard.press("Enter")
        self.page.wait_for_function("document.getElementById('drawer-toggle').checked")
        self.page.locator(".drawer-scrim").click(force=True)
        self.page.wait_for_function("!document.getElementById('drawer-toggle').checked")

        tools_toggle.focus()
        self.page.keyboard.press("Space")
        self.page.wait_for_function("document.getElementById('drawer-toggle').checked")
        self.page.keyboard.press("Escape")
        self.page.wait_for_function("!document.getElementById('drawer-toggle').checked")

        self.page.locator("#chat-toggle-btn").click()
        self.page.wait_for_function("document.getElementById('chat-toggle').checked")
        self.page.locator(".chat-scrim").click(force=True)
        self.page.wait_for_function("!document.getElementById('chat-toggle').checked")

    def test_shell_footer_stays_pinned_while_main_content_scrolls(self):
        self.open_shell()

        metrics = self.page.evaluate(
            """
            () => {
              const main = document.getElementById('main-content');
              const footer = document.querySelector('.site-footer');
              if (!main || !footer) {
                return null;
              }
              const before = footer.getBoundingClientRect().top;
              const beforeWindowScroll = window.scrollY;
              main.scrollTop = 480;
              const after = footer.getBoundingClientRect().top;
              return {
                mainClientHeight: main.clientHeight,
                mainScrollHeight: main.scrollHeight,
                mainScrollTop: main.scrollTop,
                beforeFooterTop: before,
                afterFooterTop: after,
                windowScrollY: window.scrollY,
                beforeWindowScrollY: beforeWindowScroll,
              };
            }
            """
        )

        self.assertIsNotNone(metrics)
        self.assertGreater(metrics["mainScrollHeight"], metrics["mainClientHeight"])
        self.assertGreater(metrics["mainScrollTop"], 0)
        self.assertEqual(metrics["beforeWindowScrollY"], 0)
        self.assertEqual(metrics["windowScrollY"], 0)
        self.assertAlmostEqual(metrics["beforeFooterTop"], metrics["afterFooterTop"], delta=1)

    def test_shell_scroll_region_spans_viewport_while_content_stays_centered(self):
        self.open_shell()

        metrics = self.page.evaluate(
            """
            () => {
              const main = document.getElementById('main-content');
              const content = document.querySelector('.page-content');
              if (!main || !content) {
                return null;
              }
              const mainRect = main.getBoundingClientRect();
              const contentRect = content.getBoundingClientRect();
              return {
                viewportWidth: window.innerWidth,
                mainLeft: Math.round(mainRect.left),
                mainRight: Math.round(mainRect.right),
                mainWidth: Math.round(mainRect.width),
                contentLeft: Math.round(contentRect.left),
                contentWidth: Math.round(contentRect.width),
              };
            }
            """
        )

        self.assertIsNotNone(metrics)
        self.assertEqual(metrics["mainLeft"], 0)
        self.assertEqual(metrics["mainRight"], metrics["viewportWidth"])
        self.assertEqual(metrics["mainWidth"], metrics["viewportWidth"])
        self.assertGreater(metrics["contentLeft"], 0)
        self.assertLess(metrics["contentWidth"], metrics["mainWidth"])

    def test_notifications_dropdown_renders_recent_actions(self):
        self.open_path("/shell?role=employee", ".site-header", "the employee shell fixture")
        self.page.locator("#notif-trigger").click()
        self.page.wait_for_selector(".notif-item")

        badge = self.page.locator("#notif-badge")
        self.assertEqual(badge.count(), 1)
        self.assertGreaterEqual(self.page.locator(".notif-item").count(), 1)
        self.assert_role_visible("button", "Mark notification as read")
        self.assert_role_visible("button", "Delete notification")

        metrics = self.page.evaluate(
            """
            () => {
              const item = document.querySelector('.notif-item').getBoundingClientRect();
              const read = document.querySelector('.notif-read-icon').getBoundingClientRect();
              const del = document.querySelector('.notif-delete-btn').getBoundingClientRect();
              const markAll = document.getElementById('mark-all-read-btn').getBoundingClientRect();
              return {
                itemRight: Math.round(item.right),
                readRight: Math.round(read.right),
                delRight: Math.round(del.right),
                markAllWidth: Math.round(markAll.width),
                markAllHeight: Math.round(markAll.height),
              };
            }
            """
        )
        self.assertLessEqual(metrics["readRight"], metrics["itemRight"])
        self.assertLessEqual(metrics["delRight"], metrics["itemRight"])
        self.assertGreater(metrics["markAllWidth"], 40)
        self.assertGreater(metrics["markAllHeight"], 24)

    def test_notifications_actions_update_badge_and_list(self):
        self.open_path("/shell?role=employee", ".site-header", "the employee shell fixture")
        self.page.locator("#notif-trigger").click()
        self.page.wait_for_selector(".notif-item")

        badge = self.page.locator("#notif-badge")
        self.assertEqual(badge.inner_text(), "2")
        self.assertEqual(self.page.locator(".notif-item").count(), 3)
        self.assertEqual(self.page.locator(".notif-item-unread").count(), 2)

        first_read = self.page.get_by_role("button", name="Mark notification as read").first
        first_read.click()
        self.page.wait_for_function(
            """
            () => {
              const badge = document.getElementById('notif-badge');
              return badge && !badge.hidden && badge.textContent.trim() === '1';
            }
            """
        )
        self.assertEqual(self.page.locator(".notif-item-unread").count(), 1)

        self.page.get_by_role("button", name="Delete notification").first.click()
        self.page.wait_for_function(
            "document.querySelectorAll('.notif-item').length === 2"
        )
        self.assertEqual(self.page.locator(".notif-item").count(), 2)

        self.page.locator("#mark-all-read-btn").click()
        self.page.wait_for_function(
            """
            () => {
              const badge = document.getElementById('notif-badge');
              return badge && badge.hidden;
            }
            """
        )
        self.assertEqual(self.page.locator(".notif-item-unread").count(), 0)

    def test_messaging_drawer_opens_new_conversation_modal(self):
        self.open_shell()
        self.open_chat_drawer()
        self.assert_any_visible(
            "chat list placeholder",
            [".chat-placeholder", "#chat-drawer-list"],
        )

        self.page.locator("#chat-new-btn").click()
        self.page.wait_for_selector("#new-chat-modal.active")

        self.assertTrue(self.page.locator("#new-chat-title").is_visible())
        self.assert_role_visible("radio", "Connections")
        self.assert_role_visible("radio", "All Users")
        self.assertTrue(self.page.locator("#chat-new-search").is_visible())
        self.assertTrue(self.page.locator("#chat-first-message").is_visible())
        self.assert_role_visible("button", "Send")

    def test_messaging_workflow_creates_conversation_and_sends_messages(self):
        self.open_path("/shell?role=employee", ".site-header", "the employee shell fixture")
        self.open_chat_drawer()

        self.page.locator("#chat-new-btn").click()
        self.page.wait_for_selector("#new-chat-modal.active")
        self.page.locator("#chat-new-search").fill("Alex")
        self.page.wait_for_selector('[data-add-user="coworker-1"]')
        self.page.locator('[data-add-user="coworker-1"]').click()
        self.page.locator("#chat-first-message").fill("Workflow hello from Playwright")
        self.page.locator("#chat-new-start").click()

        self.page.wait_for_selector("#chat-conv-view:not(.chat-panel-hidden)")
        self.assertTrue(self.page.locator("#chat-conv-name").is_visible())
        self.assert_text_visible("Workflow hello from Playwright")

        self.page.locator("#chat-send-input").fill("Second workflow message")
        self.page.locator("#chat-send-btn, .chat-send-btn").click()
        self.page.wait_for_function(
            """
            () => {
              const messages = document.querySelectorAll('#chat-conv-messages [data-msg-id]');
              return messages.length >= 2 && document.body.innerText.includes('Second workflow message');
            }
            """
        )

        compose_metrics = self.page.evaluate(
            """
            () => {
              const form = document.getElementById('chat-send-form').getBoundingClientRect();
              const input = document.getElementById('chat-send-input').getBoundingClientRect();
              const attach = document.querySelector('.chat-attach-btn').getBoundingClientRect();
              const send = document.querySelector('.chat-send-btn').getBoundingClientRect();
              return {
                formRight: Math.round(form.right),
                inputRight: Math.round(input.right),
                attachRight: Math.round(attach.right),
                sendRight: Math.round(send.right),
                inputWidth: Math.round(input.width),
              };
            }
            """
        )
        self.assertLessEqual(compose_metrics["inputRight"], compose_metrics["formRight"])
        self.assertLessEqual(compose_metrics["attachRight"], compose_metrics["formRight"])
        self.assertLessEqual(compose_metrics["sendRight"], compose_metrics["formRight"])
        self.assertGreater(compose_metrics["inputWidth"], 120)

        self.page.locator("#chat-back-btn").click()
        self.page.wait_for_selector("#chat-list-view")
        self.page.wait_for_function(
            """
            () => {
              const list = document.getElementById('chat-drawer-list');
              return list && list.innerText.includes('Second workflow message');
            }
            """
        )
        self.assertIn("Second workflow message", self.page.locator("#chat-drawer-list").inner_text())

    def test_social_feed_supports_comment_toggle_and_bookmarks(self):
        self.open_feed()

        active_tab = self.page.locator(".feed-tab.active").first
        self.assertTrue(active_tab.is_visible())
        self.assertEqual(active_tab.inner_text(), "Social")

        toggle = self.page.locator(".comment-toggle").first
        comments = self.page.locator("#post-comments-abcde-1234")
        self.assertEqual(toggle.get_attribute("aria-expanded"), "false")
        self.assertFalse(comments.evaluate("el => el.classList.contains('expanded')"))

        toggle.click()
        self.assertEqual(toggle.get_attribute("aria-expanded"), "true")
        self.assertTrue(comments.evaluate("el => el.classList.contains('expanded')"))
        self.assert_text_visible("Thanks for posting this update.")

        bookmark = self.page.locator(".bookmark-btn").first
        bookmark.click()
        self.page.wait_for_function(
            """
            () => {
              const button = document.querySelector('.bookmark-btn');
              return button && button.getAttribute('aria-label') === 'Saved to your bookmarks';
            }
            """
        )
        self.assertEqual(bookmark.get_attribute("aria-label"), "Saved to your bookmarks")
        self.assertIn("Saved", bookmark.inner_text())

    def test_connections_page_renders_network_actions(self):
        self.open_path("/connections", ".connections-layout", "the network page")

        self.assert_role_visible("heading", re.compile(r"Pending Invitations|Connections"))
        self.assert_any_visible(
            "connection actions",
            [".connection-item", "button[aria-label^='Accept invitation from']"],
        )

    def test_connection_workflows_accept_pending_and_request_new(self):
        self.open_path("/connections", ".connections-layout", "the network page")
        pending_casey = self.page.locator(".connection-item").filter(has_text="Casey Brooks").first
        self.assertTrue(pending_casey.is_visible())
        self.submit_and_wait_for_navigation(
            self.page.get_by_role("button", name=re.compile(r"Accept invitation from Casey Brooks")),
            lambda response: response.request.method == "POST" and "/connections/accept/coworker-2" in response.url,
        )
        self.assertEqual(self.page.locator("text=Pending Invitations").count(), 0)
        accepted_casey = self.page.locator(".connection-item").filter(has_text="Casey Brooks").first
        self.assertTrue(accepted_casey.is_visible())

        self.open_path("/search?q=Jordan", ".search-layout", "the people search page")
        jordan_result = self.page.locator(".connection-item").filter(has_text="Jordan Kim").first
        self.assertTrue(jordan_result.is_visible())
        self.submit_and_wait_for_navigation(
            jordan_result.get_by_role("button", name="Connect"),
            lambda response: response.request.method == "POST" and "/connections/request/mentor-2" in response.url,
        )
        self.assertTrue(jordan_result.get_by_text("Pending").is_visible())

    def test_bookmarks_page_renders_saved_item_controls(self):
        self.open_path("/bookmarks", ".bookmarks-layout", "the saved items page")

        self.assert_role_visible("heading", "Saved Items")
        self.assert_role_visible("link", "Posts")
        self.assert_role_visible("link", "Postings")
        self.assert_role_visible("link", "Articles")
        self.assert_any_visible(
            "bookmark content",
            [".bookmark-item", ".empty-state"],
        )

    def test_bookmarks_workflow_removes_saved_item(self):
        self.open_path("/bookmarks", ".bookmarks-layout", "the saved items page")
        self.assertEqual(self.page.locator(".bookmark-item").count(), 1)
        self.submit_and_wait_for_navigation(
            self.page.get_by_role("button", name="Remove"),
            lambda response: response.request.method == "POST" and "/bookmarks/bookmark-1/delete" in response.url,
        )
        self.assertEqual(self.page.locator(".bookmark-item").count(), 0)
        self.assert_text_visible("No saved items yet.")

    def test_group_page_renders_membership_and_discussion(self):
        self.open_path("/groups/group-1", ".group-view-layout", "the group detail page")

        self.assert_any_visible(
            "group membership controls",
            ["button:has-text('Join Group')", "button:has-text('Leave Group')"],
        )
        self.assert_role_visible("heading", "Discussion")
        self.assert_any_visible("group discussion content", [".profile-card textarea", ".empty-state"])

    def test_group_workflow_posts_leaves_and_rejoins(self):
        self.open_path("/groups/group-1", ".group-view-layout", "the group detail page")

        self.page.locator('textarea[name="content"]').fill("Workflow update from Playwright")
        self.submit_and_wait_for_navigation(
            self.page.get_by_role("button", name="Post"),
            lambda response: response.request.method == "POST" and "/groups/group-1/post" in response.url,
        )
        self.assert_text_visible("Workflow update from Playwright")

        self.submit_and_wait_for_navigation(
            self.page.get_by_role("button", name="Leave Group"),
            lambda response: response.request.method == "POST" and "/groups/group-1/leave" in response.url,
        )
        self.assertTrue(self.page.get_by_role("button", name="Join Group").is_visible())
        self.assertEqual(self.page.locator('textarea[name="content"]').count(), 0)

        self.submit_and_wait_for_navigation(
            self.page.get_by_role("button", name="Join Group"),
            lambda response: response.request.method == "POST" and "/groups/group-1/join" in response.url,
        )
        self.assertTrue(self.page.get_by_role("button", name="Leave Group").is_visible())
        self.assertEqual(self.page.locator('textarea[name="content"]').count(), 1)

    def test_mentorship_find_page_renders_mentor_discovery(self):
        self.open_path(
            "/mentorship?tab=find",
            ".connections-layout",
            "the mentorship finder",
        )

        self.assert_role_visible("heading", "Mentorship")
        self.assert_role_visible("link", "Find a Mentor")
        self.assert_any_visible(
            "mentor discovery content",
            [".mentor-item", ".empty-state"],
        )

    def test_mentorship_workflow_requests_and_accepts_mentor(self):
        self.open_path(
            "/mentorship?tab=find",
            ".connections-layout",
            "the mentorship finder",
        )
        manager_item = self.page.locator(".mentor-item").filter(has_text="Taylor Jordan").first
        self.assertTrue(manager_item.is_visible())
        self.submit_and_wait_for_navigation(
            manager_item.get_by_role("button", name="Request Mentor"),
            lambda response: response.request.method == "POST" and "/mentors/request/manager-1" in response.url,
        )
        self.assertEqual(self.page.locator(".mentor-item").filter(has_text="Taylor Jordan").count(), 0)

        self.open_path("/mentorship?tab=active", ".connections-layout", "the active mentorship page")
        active_tab = self.page.locator(".feed-tab.active").first
        self.assertEqual(active_tab.inner_text(), "Active Mentorships")
        pending_item = self.page.locator(".mentorship-item").filter(has_text="Taylor Jordan").first
        self.assertTrue(pending_item.is_visible())
        self.assertIn("pending", pending_item.inner_text().lower())
        self.assertIn("waiting for response", pending_item.inner_text().lower())

        self.open_path("/mentorship?role=manager", ".connections-layout", "the manager mentorship inbox")
        self.submit_and_wait_for_navigation(
            self.page.get_by_role("button", name="Accept"),
            lambda response: response.request.method == "POST" and "/mentorship/" in response.url and response.url.endswith("/accept"),
        )
        manager_item = self.page.locator(".mentorship-item").filter(has_text="Riley Carter").first
        self.assertTrue(manager_item.is_visible())
        self.assertIn("active", manager_item.inner_text().lower())

        self.open_path("/mentorship?tab=active&role=employee", ".connections-layout", "the employee mentorship page")
        accepted_item = self.page.locator(".mentorship-item").filter(has_text="Taylor Jordan").first
        self.assertTrue(accepted_item.is_visible())
        self.assertIn("active", accepted_item.inner_text().lower())

    def test_department_page_renders_directory_members(self):
        self.open_path(
            "/departments/dep-1",
            ".connections-layout",
            "the department directory detail page",
        )

        self.assert_role_visible("heading", "Employees")
        self.assert_any_visible("department members", [".connection-item", ".empty-state"])

    def test_workspace_page_renders_notes_and_member_controls(self):
        self.open_path(
            "/workspaces/ws-1",
            ".wsv-layout",
            "the workspace detail page",
        )

        self.assert_role_visible("heading", "Add a Note")
        self.assert_role_visible("button", "Post Note")
        self.assert_role_visible("heading", "Add a Member")
        self.assert_role_visible("button", "Add member")

    def test_workspace_workflow_adds_note_and_member(self):
        self.open_path(
            "/workspaces/workspace-1",
            ".wsv-layout",
            "the workspace detail page",
        )

        self.page.locator("#wsv-note-body").fill("Workflow note from Playwright")
        self.submit_and_wait_for_navigation(
            self.page.get_by_role("button", name="Post Note"),
            lambda response: response.request.method == "POST" and "/workspaces/workspace-1/notes" in response.url,
        )
        self.assert_text_visible("Workflow note from Playwright")

        self.page.locator("#wsv-member-id").fill("coworker-2")
        self.submit_and_wait_for_navigation(
            self.page.get_by_role("button", name="Add member"),
            lambda response: response.request.method == "POST" and "/workspaces/workspace-1/members" in response.url,
        )
        member = self.page.locator(".wsv-member").filter(has_text="Casey Brooks").first
        self.assertTrue(member.is_visible())
