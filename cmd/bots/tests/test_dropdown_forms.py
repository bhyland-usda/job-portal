import unittest

try:
    from .browser_harness import BrowserHarness
except ImportError:
    from browser_harness import BrowserHarness


class DropdownFormTests(BrowserHarness):
    def assert_select_changes_value(self, selector: str):
        matches = self.page.locator(selector)
        self.assertGreater(matches.count(), 0, f"expected at least one select matching {selector}")

        select = matches.first
        self.assertTrue(select.is_visible(), f"expected select {selector} to be visible")
        original = select.input_value()

        result = self.page.evaluate(
            """
            (cssSelector) => {
              const el = document.querySelector(cssSelector);
              if (!el) {
                return { changed: false, reason: 'missing' };
              }
              const current = el.value;
              const options = Array.from(el.options).map((opt) => opt.value);

              let target = options.find((value) => value !== current && value !== '');
              if (target === undefined) {
                target = options.find((value) => value !== current);
              }
              if (target === undefined) {
                return { changed: false, reason: 'no_alternative', current, options };
              }

              el.value = target;
              el.dispatchEvent(new Event('input', { bubbles: true }));
              el.dispatchEvent(new Event('change', { bubbles: true }));

              return {
                changed: el.value !== current,
                current,
                newValue: el.value,
                target,
                options,
              };
            }
            """,
            selector,
        )

        self.assertTrue(result["changed"], f"expected select {selector} value to change, got {result}")
        self.assertNotEqual(original, result["newValue"], f"expected select {selector} value to change")
        self.assertEqual(select.input_value(), result["newValue"], f"select {selector} did not persist changed value")

    def test_profile_edit_selects_change_value(self):
        self.open_path("/profile/edit", ".profile-layout", "the edit profile form")
        self.assert_select_changes_value("#work_status")
        self.assert_select_changes_value("#profile_visibility")

    def test_accomplishment_form_select_changes_value(self):
        self.open_path("/accomplishments/add", ".accomplishment-layout", "the accomplishment form")
        self.assert_select_changes_value("#period_type")

    def test_posting_selects_change_value(self):
        self.open_path("/postings/search", ".posting-layout", "the posting search form")
        self.assert_select_changes_value("#posting-search-type")

        self.open_path("/postings/create", ".posting-layout", "the posting create form")
        self.assert_select_changes_value("#type")
        self.assertGreater(
            self.page.locator('form[action="/postings/create"] input[name="csrf_token"]').count(),
            0,
            "expected CSRF hidden field on posting create upload form",
        )

        self.open_path("/postings/posting-1/outcome", ".posting-layout", "the posting outcome form")
        self.assert_select_changes_value("#outcome_status")

        self.open_path("/postings/posting-1/applications", ".posting-layout", "the posting applications form")
        self.assert_select_changes_value("select[name='status']")

    def test_admin_selects_change_value(self):
        self.open_path("/admin/foia", ".foia-layout", "the FOIA search form")
        self.assert_select_changes_value("#type")

        self.open_path("/admin/users", ".admin-users-table", "the admin users form controls")
        self.assert_select_changes_value("select[name='department_id']")
        self.assert_select_changes_value("select[name='role']")

    def test_orgchart_and_spotlight_selects_change_value(self):
        self.open_path("/admin/orgchart", ".orgchart-assign-layout", "the orgchart assignment form")
        self.assert_select_changes_value("#employee_id")
        self.assert_select_changes_value("#manager_id")

        self.open_path("/admin/spotlight", ".spotlight-layout", "the spotlight creation form")
        self.assert_select_changes_value("#user_id")

    def test_leaderboard_filter_select_changes_value(self):
        self.open_path("/analytics/leaderboard", ".lb-layout", "the leaderboard filter form")
        self.assert_select_changes_value("#department")


if __name__ == "__main__":
    unittest.main()
