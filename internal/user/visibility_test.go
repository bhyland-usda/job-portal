package user

import (
	"bytes"
	"html/template"
	"strings"
	"testing"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// TestCanViewFullProfile exercises the visibility decision matrix used by
// showProfile to decide between the full and limited profile views.
func TestCanViewFullProfile(t *testing.T) {
	cases := []struct {
		name       string
		visibility string
		isOwner    bool
		viewerRole string
		connStatus string
		want       bool
	}{
		{"owner sees own private profile", "private", true, "employee", "self", true},
		{"admin sees private profile", "private", false, "admin", "none", true},
		{"admin sees connections-only profile", "connections", false, "admin", "none", true},
		{"everyone visible to stranger", "everyone", false, "employee", "none", true},
		{"unknown value treated as everyone", "", false, "employee", "none", true},
		{"connections-only hidden from non-connection", "connections", false, "employee", "none", false},
		{"connections-only hidden from pending", "connections", false, "employee", "pending_sent", false},
		{"connections-only shown to connection", "connections", false, "employee", "connected", true},
		{"private hidden from connection", "private", false, "employee", "connected", false},
		{"private hidden from stranger", "private", false, "employee", "none", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CanViewFullProfile(tc.visibility, tc.isOwner, tc.viewerRole, tc.connStatus)
			if got != tc.want {
				t.Errorf("CanViewFullProfile(%q, owner=%v, role=%q, conn=%q) = %v, want %v",
					tc.visibility, tc.isOwner, tc.viewerRole, tc.connStatus, got, tc.want)
			}
		})
	}
}

// renderProfileView renders the profile "content" block against a ProfileView,
// mirroring renderContent in handler_test.go but local to this file.
func renderProfileView(t *testing.T, view ProfileView) string {
	t.Helper()
	tmpl, err := template.ParseFiles("../../templates/profile/view.html")
	if err != nil {
		t.Fatalf("failed to parse profile template: %v", err)
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "content", view); err != nil {
		t.Fatalf("failed to render content: %v", err)
	}
	return buf.String()
}

// TestConnectionsOnlyLimitedViewForNonConnection verifies that a
// connections-only profile shows the limited "connections only" view (name +
// reason, no About/Experience details) to a viewer who is not connected.
func TestConnectionsOnlyLimitedViewForNonConnection(t *testing.T) {
	view := ProfileView{
		ProfileData: &ProfileData{
			BaseData: middleware.BaseData{UserID: "viewer", Nav: middleware.UserInfo{ID: "viewer"}},
			User: User{
				ID: "owner", FirstName: "Ada", LastName: "Lovelace",
				About: "TOP SECRET BIO", ProfileVisibility: "connections",
			},
			IsOwnProfile:     false,
			ConnectionStatus: "none",
			Restricted:       true,
			LimitedReason:    LimitedReason("connections"),
		},
	}

	out := renderProfileView(t, view)

	if !strings.Contains(out, "Ada") {
		t.Error("expected the owner's name to be shown on the limited view")
	}
	if !strings.Contains(out, "connections only") {
		t.Errorf("expected the connections-only reason in limited view, got: %s", out)
	}
	if strings.Contains(out, "TOP SECRET BIO") {
		t.Error("limited view must not leak the About/details section")
	}
	if !strings.Contains(out, "/connections/request/owner") {
		t.Error("expected a Connect prompt on the limited view")
	}
}

// TestConnectionsOnlyFullViewForConnection verifies that the same
// connections-only profile shows the full view (with details) to an accepted
// connection.
func TestConnectionsOnlyFullViewForConnection(t *testing.T) {
	view := ProfileView{
		ProfileData: &ProfileData{
			BaseData: middleware.BaseData{UserID: "viewer", Nav: middleware.UserInfo{ID: "viewer"}},
			User: User{
				ID: "owner", FirstName: "Ada", LastName: "Lovelace",
				About: "Visible to connections", ProfileVisibility: "connections",
			},
			IsOwnProfile:     false,
			ConnectionStatus: "connected",
			Restricted:       false,
		},
	}

	out := renderProfileView(t, view)

	if !strings.Contains(out, "Visible to connections") {
		t.Error("expected full About section for an accepted connection")
	}
	if strings.Contains(out, "visible to connections only") {
		t.Error("did not expect the limited-view reason on a full render")
	}
}
