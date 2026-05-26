package middleware

import "net/http"

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
	UserID string
	Nav    UserInfo
}

// NewBaseData creates a BaseData from the request context.
func NewBaseData(r *http.Request) BaseData {
	info := GetUserInfo(r.Context())
	return BaseData{
		UserID: info.ID,
		Nav:    info,
	}
}
