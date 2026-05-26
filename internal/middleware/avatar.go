package middleware

import "strings"

// NormalizeAvatarURL ensures uploaded avatars resolve through the app's avatar
// endpoint while preserving already-normalized or external URLs.
func NormalizeAvatarURL(userID, avatarURL string) string {
	avatarURL = strings.TrimSpace(avatarURL)
	if avatarURL == "" {
		return ""
	}
	if strings.HasPrefix(avatarURL, "/avatar/") ||
		strings.HasPrefix(avatarURL, "http://") ||
		strings.HasPrefix(avatarURL, "https://") ||
		strings.HasPrefix(avatarURL, "data:") {
		return avatarURL
	}
	return "/avatar/" + userID
}
