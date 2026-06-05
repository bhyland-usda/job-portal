package middleware

import (
	"net/http"

	"github.com/bhyland-usda/job-portal/internal/semantic"
)

// BaseData should be embedded in every page data struct.
// It provides Nav info for the navbar template.
//
// Example:
//
//	type FeedPage struct {
//	    middleware.BaseData
//	    Posts []Post
//	}
//
// Template accesses: .Nav.FirstName, .Nav.Role, .Nav.AvatarURL, .UserID
type BaseData struct {
	UserID          string
	Nav             UserInfo
	CSRFToken       string
	CurrentPath     string
	MatchingEnabled bool
	LocationPrefAll bool
	LocationRemote  bool
	LocationHybrid  bool
	LocationOnsite  bool
	LocationLabel   string
}

// NewBaseData creates a BaseData from the request context.
func NewBaseData(r *http.Request) BaseData {
	info := GetUserInfo(r.Context())
	locationPref := semantic.LocationTypePreferenceForRequest(r)
	return BaseData{
		UserID:          info.ID,
		Nav:             info,
		CSRFToken:       GetCSRFToken(r.Context()),
		CurrentPath:     r.URL.RequestURI(),
		MatchingEnabled: semantic.EnabledForRequest(r),
		LocationPrefAll: locationPref.IsAll(),
		LocationRemote:  locationPref.Remote,
		LocationHybrid:  locationPref.Hybrid,
		LocationOnsite:  locationPref.Onsite,
		LocationLabel:   locationPref.Label(),
	}
}
