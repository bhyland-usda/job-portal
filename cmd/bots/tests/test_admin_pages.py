import unittest
from urllib.parse import parse_qs, urlparse

try:
    from .browser_harness import BrowserHarness
except ImportError:
    from browser_harness import BrowserHarness


class AdminSubsystemPageTests(BrowserHarness):
    def test_admin_dashboard_renders_metrics_and_admin_links(self):
        self.open_path("/admin/dashboard", ".dash-layout", "the admin dashboard")

        self.assert_role_visible("heading", "Admin Dashboard")
        self.assertGreaterEqual(self.page.locator(".dash-card").count(), 6)

        labels = [label.lower() for label in self.page.locator(".dash-card-label").all_inner_texts()]
        for want in [
            "total users",
            "departments",
            "posts this week",
            "new connections (7d)",
            "active postings",
            "pending applications",
        ]:
            self.assertIn(want, labels)

        role_counts = self.page.locator(".dash-card-sub").first.inner_text().lower()
        for want in ["employee:", "manager:", "admin:"]:
            self.assertIn(want, role_counts)

        self.assert_role_visible("heading", "Recent Signups")
        self.assertGreater(self.page.locator(".dash-mini-table").first.locator("tbody tr").count(), 0)
        self.assertGreater(self.page.locator(".dash-mini-table").nth(1).locator("tbody tr").count(), 0)

        self.open_tools_drawer()
        self.assert_text_visible("Administration")
        for want in ["Admin Dashboard", "Audit Log", "Moderation Queue", "FOIA Tool"]:
            self.assert_role_visible("link", want)

    def test_moderation_queue_lists_reports_with_resolution_actions(self):
        self.open_path(
            "/admin/moderation",
            ".moderation-layout",
            "the moderation queue",
        )

        self.assert_role_visible("heading", "Moderation Queue")
        reports = self.page.locator(".report-item")
        initial_count = reports.count()
        self.assertGreater(initial_count, 0)

        first_report = reports.first
        self.assertTrue(first_report.locator(".report-type").is_visible())
        self.assertTrue(first_report.locator(".report-content-id").is_visible())
        self.assertTrue(first_report.locator(".report-meta").is_visible())
        self.assertTrue(first_report.locator(".report-actions").is_visible())
        dismiss_button = first_report.locator(
            "form:has(input[name='action'][value='dismissed']) button[type='submit']"
        )
        self.assertTrue(first_report.get_by_role("button", name="Mark Reviewed").is_visible())
        self.assertTrue(dismiss_button.is_visible())

        response = self.submit_and_wait_for_navigation(
            dismiss_button,
            lambda resp: resp.request.method == "POST"
            and resp.url.endswith("/admin/moderation/report-1/resolve"),
        )
        self.assertIn("action=dismissed", response.request.post_data or "")

        remaining = self.page.locator(".report-item").count()
        if self.page.locator(".empty-state").count() > 0 or remaining < initial_count:
            self.assertLess(remaining, initial_count)
        else:
            self.assertEqual(remaining, initial_count)
            self.assertTrue(
                self.page.locator("form[action='/admin/moderation/report-1/resolve']").first.is_visible()
            )

    def test_foia_search_renders_results_and_export_link(self):
        self.open_path("/admin/foia", ".foia-layout", "the FOIA search page")

        self.assert_role_visible("heading", "FOIA Data Search")
        self.page.locator("#user").fill("Riley")
        self.page.locator("#from").fill("2026-05-01")
        self.page.locator("#to").fill("2026-05-31")
        self.page.locator("#type").select_option("post")

        with self.page.expect_navigation(wait_until="domcontentloaded"):
            self.page.locator("form[action='/admin/foia/search'] button[type='submit']").click()

        self.assertEqual(urlparse(self.page.url).path, "/admin/foia/search")
        self.assertGreater(self.page.locator(".admin-table tbody tr").count(), 0)
        current_params = parse_qs(urlparse(self.page.url).query)
        self.assertEqual(current_params["user"], ["Riley"])
        self.assertEqual(current_params["type"], ["post"])
        self.assertEqual(current_params["from"], ["2026-05-01"])
        self.assertEqual(current_params["to"], ["2026-05-31"])

        export_link = self.page.get_by_role("link", name="Export JSON")
        self.assertTrue(export_link.is_visible())
        href = export_link.get_attribute("href")
        self.assertIsNotNone(href)
        parsed = urlparse(href)
        self.assertEqual(parsed.path, "/admin/foia/export")
        params = parse_qs(parsed.query)
        for key in ["user", "type", "from", "to"]:
            self.assertIn(key, params)
            self.assertEqual(params[key], current_params[key])

        export_response = self.page.context.request.get(f"{self.base_url}{href}")
        self.assertTrue(export_response.ok)
        payload = export_response.json()
        if isinstance(payload, list):
            self.assertGreater(len(payload), 0)
            self.assertEqual(payload[0]["type"], "post")
        else:
            self.assertEqual(payload["status"], "ready")
            self.assertIn("export_id", payload)

    def test_workforce_analytics_shows_summary_and_distribution_tables(self):
        self.open_path(
            "/analytics/workforce",
            ".wf-layout",
            "the workforce analytics dashboard",
        )

        self.assert_role_visible("heading", "Workforce Analytics")
        self.assertGreaterEqual(self.page.locator(".wf-stat").count(), 5)
        self.assertGreater(self.page.locator(".wf-table tbody tr").count(), 0)

        section_titles = self.page.locator(".wf-section h3").all_inner_texts()
        for want in [
            "Headcount by Department",
            "Breakdown by Role",
            "Top Skills Across the Org",
        ]:
            self.assertIn(want, section_titles)

        self.open_tools_drawer()
        self.assert_text_visible("Leadership tools")
        self.assert_role_visible("link", "Workforce Analytics")

    def test_network_page_renders_graph_json_into_svg(self):
        self.open_path(
            "/insights/network",
            "#network-svg",
            "the network graph page",
        )

        self.page.wait_for_function(
            """
            () => {
              return document.querySelectorAll('#network-svg .node').length > 0;
            }
            """
        )

        self.assert_role_visible("heading", "My Network Graph")
        self.assertGreater(self.page.locator("#network-svg .node").count(), 0)

        script_text = "\n".join(self.page.locator("script").all_inner_texts())
        self.assertIn('"nodes"', script_text)
        self.assertIn('"edges"', script_text)

    def test_orgchart_shows_roots_and_admin_assignment_action(self):
        self.open_path("/orgchart", ".orgchart-layout", "the org chart")

        self.assert_role_visible("heading", "Org Chart")
        self.assertGreater(self.page.locator(".orgchart-tree > li").count(), 0)
        self.assertGreater(self.page.locator(".org-node .org-name").count(), 0)
        self.assertTrue(self.page.locator(".orgchart-tree.orgchart-graph").is_visible())
        self.assert_role_visible("link", "Assign Managers")

    def test_orgchart_pages_use_shared_themed_styles(self):
        self.open_path("/orgchart", ".orgchart-layout", "the org chart")
        self.assertEqual(self.page.locator("main#main-content style").count(), 0)

        self.open_path("/admin/orgchart", ".orgchart-assign-layout", "the org chart assignment page")
        self.assertEqual(self.page.locator("main#main-content style").count(), 0)
        back_link = self.page.get_by_role("link", name="← Back to Org Chart")
        self.assertTrue(back_link.is_visible())

    def test_workforce_and_heatmap_pages_use_shared_themed_styles(self):
        self.open_path(
            "/analytics/workforce",
            ".wf-layout",
            "the workforce analytics dashboard",
        )
        self.assertEqual(self.page.locator("main#main-content style").count(), 0)
        self.assertEqual(self.page.locator("main#main-content [style]").count(), 0)

        response = self.page.goto(f"{self.base_url}/insights/heatmap", wait_until="domcontentloaded")
        self.assertIsNotNone(response)
        self.assertLess(response.status, 400)
        self.assertEqual(urlparse(self.page.url).path, "/insights/heatmap")
        self.page.wait_for_selector(".heatmap-layout", timeout=5000)
        self.assertEqual(self.page.locator("main#main-content style").count(), 0)

    def test_data_export_page_renders_download_summary(self):
        self.open_path("/data-export", ".dataexport-layout", "the data export page")

        self.assert_role_visible("heading", "Export My Data")
        self.assertGreaterEqual(self.page.locator(".dataexport-list li").count(), 6)
        self.assert_text_visible("Your password is never included.")

        download = self.page.locator("#data-export-download")
        self.assertTrue(download.is_visible())
        self.assertEqual(download.get_attribute("href"), "/data-export/download")

    def test_aup_page_renders_policy_sections_and_acceptance_affordance(self):
        self.open_path("/aup", ".aup-layout", "the acceptable use policy page")

        self.assert_role_visible("heading", "Acceptable Use Policy")
        self.assertEqual(self.page.locator(".aup-policy h3").count(), 5)

        accept_form = self.page.locator("form[action='/aup/accept']")
        if accept_form.count() > 0:
            self.submit_and_wait_for_navigation(
                accept_form.locator("button[type='submit']"),
                lambda resp: resp.request.method == "POST" and resp.url.endswith("/aup/accept"),
            )

        accepted_note = self.page.locator(".aup-accepted-note")
        self.assertTrue(accepted_note.is_visible())
        continue_link = self.page.get_by_role("link", name="Continue to Feed")
        self.assertTrue(continue_link.is_visible())

        with self.page.expect_navigation(wait_until="domcontentloaded"):
            continue_link.click()
        self.assertEqual(urlparse(self.page.url).path, "/feed")
        self.assertTrue(self.page.locator(".feed-layout").first.is_visible())


if __name__ == "__main__":
    unittest.main()
