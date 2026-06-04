try:
    from .browser_harness import BrowserHarness
except ImportError:
    from browser_harness import BrowserHarness


class FeedDraftSurfaceTests(BrowserHarness):
    def test_feed_composer_no_longer_shows_schedule_controls(self):
        self.open_feed("social")

        self.assertEqual(self.page.locator('#composer-content').count(), 1)
        self.assertEqual(self.page.locator('#scheduled_at').count(), 0)
        self.assertEqual(self.page.locator('a[href="/feed/scheduled"]').count(), 0)

    def test_drafts_uses_rich_editor_with_social_type_clarity(self):
        self.open_path("/feed/drafts", ".feed-layout", "the drafts page")

        self.assertTrue(self.page.get_by_text("Create and schedule Social Feed posts from one workspace.").first.is_visible())
        self.assertTrue(self.page.get_by_role("link", name="Create Project/Detail Posting").first.is_visible())

        editors = self.page.locator('[data-rich-editor]')
        self.assertEqual(editors.count(), 1)
        self.assertEqual(self.page.locator('[data-schedule-toggle]').count(), 1)
        self.assertEqual(self.page.locator('[data-schedule-panel]:not([hidden])').count(), 0)

        self.page.locator('[data-schedule-toggle]').click()
        self.assertEqual(self.page.locator('[data-schedule-panel]:not([hidden])').count(), 1)
        self.assertEqual(self.page.locator('[data-schedule-submit]:not([hidden])').count(), 1)

        metrics = self.page.evaluate(
            """
            () => {
              const editor = document.querySelector('.rich-editor-input');
              if (!editor) return null;
              const rect = editor.getBoundingClientRect();
              return {
                height: Math.round(rect.height),
                contentEditable: editor.getAttribute('contenteditable')
              };
            }
            """
        )

        self.assertIsNotNone(metrics)
        self.assertGreaterEqual(metrics["height"], 160)
        self.assertEqual(metrics["contentEditable"], "true")

        def test_drafts_lists_handle_many_items_without_breaking_layout(self):
                self.open_path("/feed/drafts", ".feed-layout", "the drafts page")

                metrics = self.page.evaluate(
                        """
                        () => {
                            const containers = Array.from(document.querySelectorAll('.draft-list-scroll'));
                            containers.forEach((container) => {
                                const seed = container.querySelector('.draft-item-card');
                                if (!seed) return;
                                for (let i = 0; i < 10; i++) {
                                    const clone = seed.cloneNode(true);
                                    clone.querySelectorAll('button').forEach((btn) => btn.removeAttribute('id'));
                                    container.appendChild(clone);
                                }
                            });

                            return containers.map((container) => {
                                const style = getComputedStyle(container);
                                return {
                                    overflowY: style.overflowY,
                                    clientHeight: Math.round(container.clientHeight),
                                    scrollHeight: Math.round(container.scrollHeight),
                                };
                            });
                        }
                        """
                )

                self.assertGreaterEqual(len(metrics), 2)
                for panel in metrics:
                        self.assertIn(panel["overflowY"], ["auto", "scroll"])
                        self.assertGreater(panel["scrollHeight"], panel["clientHeight"])
