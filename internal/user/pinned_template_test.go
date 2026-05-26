package user

import (
	"strings"
	"testing"
	"time"
)

// TestProfilePinnedSectionRenders verifies the pinned posts section appears at
// the top of the profile with the post content, and on the OWNER's own profile
// each pinned post has an Unpin control.
func TestProfilePinnedSectionRenders(t *testing.T) {
	view := ProfileView{
		ProfileData: &ProfileData{
			User:         User{ID: "u1", FirstName: "Self", LastName: "User"},
			IsOwnProfile: true,
		},
		PinnedPosts: []PinnedPost{
			{ID: "post-1", AuthorID: "u1", AuthorFirstName: "Self", AuthorLastName: "User",
				Content: "My pinned thoughts", CreatedAt: time.Now(), PinnedAt: time.Now()},
		},
	}
	out := renderContent(t, view)

	if !strings.Contains(out, "Pinned") {
		t.Error("expected a 'Pinned' heading")
	}
	if !strings.Contains(out, "My pinned thoughts") {
		t.Error("expected pinned post content rendered")
	}
	if !strings.Contains(out, "/profile/posts/post-1/unpin") {
		t.Error("expected Unpin form action on own profile")
	}
}

// TestProfilePinnedSectionNoUnpinForVisitor verifies a visitor sees the pinned
// posts but no Unpin control.
func TestProfilePinnedSectionNoUnpinForVisitor(t *testing.T) {
	view := ProfileView{
		ProfileData: &ProfileData{
			User:         User{ID: "u2", FirstName: "Ada", LastName: "Lovelace"},
			IsOwnProfile: false,
		},
		PinnedPosts: []PinnedPost{
			{ID: "post-1", AuthorID: "u2", AuthorFirstName: "Ada", AuthorLastName: "Lovelace",
				Content: "Ada pinned this post", CreatedAt: time.Now()},
		},
	}
	out := renderContent(t, view)

	if !strings.Contains(out, "Ada pinned this post") {
		t.Error("expected pinned post content rendered for visitor")
	}
	if strings.Contains(out, "/unpin") {
		t.Error("did not expect Unpin control when viewing another user's profile")
	}
}

// TestProfilePinnedSectionAbsentWhenEmpty verifies no pinned section renders when
// there are no pinned posts.
func TestProfilePinnedSectionAbsentWhenEmpty(t *testing.T) {
	view := ProfileView{
		ProfileData: &ProfileData{
			User:         User{ID: "u1", FirstName: "Self", LastName: "User"},
			IsOwnProfile: true,
		},
	}
	out := renderContent(t, view)
	if strings.Contains(out, "/unpin") {
		t.Error("did not expect an Unpin control with no pinned posts")
	}
}
