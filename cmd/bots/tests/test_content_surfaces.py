try:
    from .browser_harness import BrowserHarness
except ImportError:
    from browser_harness import BrowserHarness


class ContentSurfaceTests(BrowserHarness):
    def test_feed_postings_tab_shows_create_action(self):
        self.open_feed(tab="postings")

        active_tab = self.page.locator(".feed-tab.active").first
        self.assertTrue(active_tab.is_visible())
        self.assertEqual(active_tab.inner_text(), "Postings")
        self.assert_role_visible("link", "Create Posting")
        self.assert_text_visible("Temporary detail opportunity")

    def test_posting_page_renders_apply_or_manager_actions(self):
        self.open_path("/postings/posting-1?role=manager", ".posting-layout", "the posting detail page")

        self.assert_any_visible(
            "posting primary actions",
            [
                "button:has-text('Apply')",
                "a:has-text('View Applications')",
                "a:has-text('Record Outcome')",
            ],
        )
        self.assert_any_visible(
            "posting bookmark control",
            [".bookmark-btn", ".bookmark-saved"],
        )
        self.assert_any_visible(
            "posting detail content",
            [".posting-body", ".posting-skills", ".posting-outcome"],
        )

        close_form = self.page.locator("form[action='/postings/posting-1/close']")
        if close_form.count() == 0:
            return

        response = self.submit_and_wait_for_navigation(
            close_form.locator("button[type='submit']"),
            lambda resp: resp.request.method == "POST" and resp.url.endswith("/postings/posting-1/close"),
        )
        self.assertTrue(self.page.locator(".posting-layout").first.is_visible())

        status_badge = self.page.locator(".posting-status-closed")
        if status_badge.count() > 0:
            self.assertTrue(status_badge.first.is_visible())
            self.assertEqual(close_form.count(), 0)
        else:
            self.assertEqual(close_form.count(), 1)
            self.assertTrue(self.page.locator(".posting-outcome").first.is_visible())

    def test_article_page_renders_reader_controls(self):
        self.open_path("/articles/article-1", ".article-layout", "the article detail page")

        self.assert_role_visible("link", "Back to Articles")
        self.assert_any_visible(
            "article bookmark control",
            [".bookmark-btn", ".bookmark-saved"],
        )
        self.assertTrue(self.page.locator(".article-body").first.is_visible())

    def test_news_page_renders_story_content(self):
        self.open_path("/news/news-1", ".news-layout", "the news detail page")

        self.assert_any_visible(
            "news header content",
            [".news-meta", ".news-draft-badge"],
        )
        self.assertTrue(self.page.locator(".news-content").first.is_visible())
        self.assert_role_visible("link", "Back to News")

    def test_announcements_page_renders_updates(self):
        self.open_path("/announcements", ".news-layout", "the announcements index")

        self.assert_role_visible("heading", "Announcements")
        self.assert_any_visible(
            "announcement content",
            [".announcement-item", ".empty-state"],
        )

    def test_spotlight_page_renders_current_or_past_spotlights(self):
        self.open_path("/spotlight", ".spotlight-layout", "the spotlight page")

        self.assert_role_visible("heading", "Employee Spotlight")
        self.assert_any_visible(
            "spotlight content",
            [".spotlight-featured", ".spotlight-past-list", ".empty-state"],
        )

    def test_kudos_page_renders_received_tab(self):
        self.open_path("/kudos?tab=received", ".kudos-layout", "the kudos page")

        self.assert_role_visible("heading", "Kudos")
        active_tab = self.page.locator(".feed-tab.active").first
        self.assertTrue(active_tab.is_visible())
        self.assertEqual(active_tab.inner_text(), "Received")
        self.assert_any_visible("kudos content", [".kudos-item", ".empty-state"])

        sent_tab = self.page.locator("a.feed-tab[href='/kudos?tab=sent']")
        with self.page.expect_navigation(wait_until="domcontentloaded"):
            sent_tab.click()

        self.assertTrue(self.page.locator(".kudos-layout").first.is_visible())
        self.assertTrue(self.page.locator("a.feed-tab.active[href='/kudos?tab=sent']").first.is_visible())
        self.assert_any_visible("sent kudos state", [".kudos-item", ".empty-state"])

    def test_polls_page_renders_vote_or_result_controls(self):
        self.open_path("/polls", ".poll-layout", "the polls page")

        self.assert_role_visible("heading", "Polls")
        poll = self.page.locator(".poll-card").first
        self.assertTrue(poll.is_visible())

        vote_form = poll.locator("form[action='/polls/poll-1/vote']")
        if vote_form.count() > 0:
            option = vote_form.locator("input[type='radio']").first
            option.check()
            self.submit_and_wait_for_navigation(
                vote_form.locator("button[type='submit']"),
                lambda resp: resp.request.method == "POST" and resp.url.endswith("/polls/poll-1/vote"),
            )
            self.assertEqual(self.page.locator("form[action='/polls/poll-1/vote']").count(), 0)
            self.assertGreater(self.page.locator(".poll-results .poll-result-row").count(), 0)
        else:
            self.assertTrue(poll.locator(".poll-results").first.is_visible())
            self.assertGreater(poll.locator(".poll-result-row").count(), 0)
            self.assertIn("vote", poll.locator(".poll-meta").inner_text().lower())

    def test_badges_page_renders_earned_or_available_badges(self):
        self.open_path("/badges", ".badge-layout", "the badges page")

        self.assert_any_visible(
            "badge headings",
            ["text=Earned Badges", "text=Available Badges", "text=All Badges"],
        )
        self.assert_any_visible(
            "badge content",
            [".badge-item", ".empty-state"],
        )
        self.assertEqual(self.page.locator(".badge-layout form, .badge-layout button").count(), 0)
