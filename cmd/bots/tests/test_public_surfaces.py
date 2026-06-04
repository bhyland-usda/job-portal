import re

try:
    from .browser_harness import BrowserHarness
except ImportError:
    from browser_harness import BrowserHarness


class PublicSurfaceTests(BrowserHarness):
    def test_landing_page_promotes_sign_in_and_join(self):
        self.use_mobile_page(390, 844)
        self.open_landing()
        self.assert_role_visible("heading", re.compile(r"Connect\. Collaborate\. Grow\."))
        self.assert_role_visible("link", "Join Now")
        self.assert_role_visible("link", "Sign In")

        hero_direction = self.page.locator(".hero").evaluate(
            "el => getComputedStyle(el).flexDirection"
        )
        self.assertIn(hero_direction, ["column", "column-reverse"])

    def test_login_page_renders_credentials_form(self):
        self.open_path("/login", 'form[action="/login"]', "the sign-in form")

        self.assert_role_visible("heading", "Sign In")
        self.assertGreater(
            self.page.locator('form[action="/login"] input[name="csrf_token"]').count(),
            0,
            "expected CSRF hidden field on strict login form",
        )

    def test_profile_page_renders_self_service_actions(self):
        self.open_path("/profile", ".profile-header", "the profile header and actions")

        self.assert_role_visible("heading", re.compile(r".+"))
        self.assert_role_visible("link", "Edit Profile")
        self.assert_role_visible("link", "Print / Export")
        self.assert_role_visible("heading", "Experience")
        self.assert_role_visible("heading", "Education")

    def test_resume_page_renders_upload_and_generate_actions(self):
        self.open_path("/resume", ".resume-layout", "the resume manager")

        self.assert_role_visible("heading", "My Resumes")
        self.assert_role_visible("link", "Generate from Profile")
        upload = self.page.locator('label[for="resume"]')
        self.assertTrue(upload.is_visible())
        self.assertGreater(
            self.page.locator('form[action="/resumes/upload"] input[name="csrf_token"]').count(),
            0,
            "expected CSRF hidden field on resume upload form",
        )
        self.assert_text_visible("PDF only, max 10MB")

    def test_accomplishments_page_renders_filters_and_create_action(self):
        self.open_path(
            "/accomplishments",
            ".accomplishment-layout",
            "the accomplishments workspace",
        )

        self.assert_role_visible("heading", "My Accomplishments")
        self.assert_role_visible("link", "Add Accomplishment")
        self.assert_role_visible("link", "Export")
        self.assert_role_visible("link", "Quarterly")
        self.assert_role_visible("link", "Yearly")

    def test_certifications_page_renders_form_controls(self):
        self.open_path("/certifications", ".cert-layout", "the certifications page")

        self.assert_role_visible("heading", "My Certifications")
        self.assert_role_visible("heading", "Add a Certification")
        self.assert_role_visible("textbox", "Name")
        self.assert_role_visible("textbox", "Issuer")
        self.assert_role_visible("button", "Add Certification")

    def test_digest_page_renders_weekly_sections(self):
        self.open_path("/digest", ".digest-layout", "the weekly digest view")

        self.assert_role_visible("heading", "Your Weekly Digest")
        self.assert_role_visible("heading", "New Connections")
        self.assert_role_visible("heading", "Unread Notifications")
        self.assert_role_visible("heading", "New Postings Matching Your Skills")

    def test_feedback_request_page_renders_subject_form(self):
        self.open_path(
            "/feedback/request",
            ".feedback-layout",
            "the feedback request form",
        )

        self.assert_role_visible("heading", re.compile(r"Request Feedback"))
        self.assert_role_visible("textbox", "Subject")
        submit = self.assert_role_visible("button", "Send Request")
        self.assertTrue(submit.is_enabled())
        self.assert_role_visible("link", "Cancel")

    def test_search_page_renders_search_controls(self):
        self.open_path("/search", ".search-layout", "the people search page")

        search_form = self.page.get_by_role("search").first
        self.assertTrue(search_form.is_visible())
        search_box = self.page.get_by_role("textbox", name=re.compile(r"Search people")).first
        self.assertTrue(search_box.is_visible())
        submit = self.assert_role_visible("button", "Search")
        self.assertTrue(submit.is_enabled())

    def test_onboarding_page_renders_progress_and_completion_actions(self):
        self.open_path("/onboarding", ".posting-layout", "the onboarding checklist")

        self.assert_role_visible("heading", "Getting Started")
        self.assertTrue(self.page.locator(".completeness-bar").first.is_visible())
        self.assert_any_visible(
            "onboarding completion controls",
            ["button:has-text('Done')", ".my-post-item strong"],
        )
