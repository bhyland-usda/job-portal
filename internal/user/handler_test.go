package user

import (
	"bytes"
	"html/template"
	"os"
	"strings"
	"testing"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// loadProfileContent parses the profile view template's "content" block in
// isolation. We only execute the "content" definition (which is what the bug
// fixes touch) so the test does not depend on the layout/navbar templates,
// which live outside this change's scope.
func loadProfileContent(t *testing.T) *template.Template {
	t.Helper()
	tmpl, err := template.ParseFiles("../../templates/profile/view.html")
	if err != nil {
		t.Fatalf("failed to parse profile template: %v", err)
	}
	return tmpl
}

func renderContent(t *testing.T, view ProfileView) string {
	t.Helper()
	tmpl := loadProfileContent(t)
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "content", view); err != nil {
		t.Fatalf("failed to render content: %v", err)
	}
	return buf.String()
}

// TestProfileFullTemplateRenders verifies the full template (layouts + view)
// executes against a ProfileView via the embedded *ProfileData, mirroring how
// the production handler renders it. This catches field-promotion errors that a
// content-only render would miss.
func TestProfileFullTemplateRenders(t *testing.T) {
	tmpl, err := template.ParseFS(
		os.DirFS("../../templates"),
		"layouts/*.html",
		"profile/view.html",
	)
	if err != nil {
		t.Fatalf("failed to parse full template set: %v", err)
	}

	view := ProfileView{
		ProfileData: &ProfileData{
			BaseData:         middleware.BaseData{UserID: "u1", Nav: middleware.UserInfo{ID: "u1"}},
			User:             User{ID: "u2", FirstName: "Ada", LastName: "Lovelace"},
			IsOwnProfile:     false,
			ConnectionStatus: "connected",
		},
		IsFollowing: true,
		SkillViews: []SkillView{
			{Skill: Skill{ID: "s1", Name: "Go", EndorsementCount: 3}, VerificationCount: 2},
		},
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base", view); err != nil {
		t.Fatalf("failed to render full base template: %v", err)
	}
	if !strings.Contains(buf.String(), "/profile/skills/s1/verify") {
		t.Error("expected verify form in full render")
	}
}

// TestProfileVerifyButtonShownForOtherUser verifies BUG-2: a "Verify" button
// and the verification count render when viewing another user's connected
// profile, alongside the existing "Endorse" button.
func TestProfileVerifyButtonShownForOtherUser(t *testing.T) {
	view := ProfileView{
		ProfileData: &ProfileData{
			User:             User{ID: "u2", FirstName: "Ada", LastName: "Lovelace"},
			IsOwnProfile:     false,
			ConnectionStatus: "connected",
		},
		SkillViews: []SkillView{
			{Skill: Skill{ID: "s1", Name: "Go", EndorsementCount: 3}, VerificationCount: 2},
		},
	}

	out := renderContent(t, view)

	if !strings.Contains(out, `/profile/skills/s1/verify`) {
		t.Error("expected Verify form action for other user's profile")
	}
	if !strings.Contains(out, ">Verify<") {
		t.Error("expected a Verify button label")
	}
	if !strings.Contains(out, "Endorse") {
		t.Error("expected Endorse button to remain present")
	}
	if !strings.Contains(out, "2") {
		t.Error("expected verification count to be displayed")
	}
}

// TestProfileVerifyButtonHiddenOnOwnProfile verifies the Verify button is gated
// to other users' profiles only (matching how Endorse is gated).
func TestProfileVerifyButtonHiddenOnOwnProfile(t *testing.T) {
	view := ProfileView{
		ProfileData: &ProfileData{
			User:         User{ID: "u1", FirstName: "Self", LastName: "User"},
			IsOwnProfile: true,
		},
		SkillViews: []SkillView{
			{Skill: Skill{ID: "s1", Name: "Go"}, VerificationCount: 0},
		},
	}

	out := renderContent(t, view)

	if strings.Contains(out, "/verify") {
		t.Error("Verify button should not appear on own profile")
	}
}

// TestProfileFollowButtonReflectsState verifies BUG-3: the follow button shows
// "Following" when already following and "Follow" otherwise.
func TestProfileFollowButtonReflectsState(t *testing.T) {
	base := func(following bool) ProfileView {
		return ProfileView{
			ProfileData: &ProfileData{
				User:             User{ID: "u2", FirstName: "Ada", LastName: "Lovelace"},
				IsOwnProfile:     false,
				ConnectionStatus: "connected",
			},
			IsFollowing: following,
		}
	}

	notFollowing := renderContent(t, base(false))
	if !strings.Contains(notFollowing, "Follow") {
		t.Error("expected 'Follow' label when not following")
	}
	if strings.Contains(notFollowing, "Following") {
		t.Error("did not expect 'Following' label when not following")
	}

	following := renderContent(t, base(true))
	if !strings.Contains(following, "Following") {
		t.Error("expected 'Following' label when already following")
	}
}
