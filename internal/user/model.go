package user

import (
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type User struct {
	ID                string
	Email             string
	FirstName         string
	LastName          string
	Headline          string
	About             string
	AvatarURL         string
	Location          string
	CreatedAt         time.Time
	Birthday          *time.Time
	HireDate          *time.Time
	WorkStatus        string
	WorkStatusUntil   *time.Time
	ProfileVisibility string
}

// VisibilityOptions is the set of valid profile_visibility values, in display
// order. It is the single source of truth used both to validate form input and
// to render the edit <select>.
var VisibilityOptions = []VisibilityOption{
	{Value: "everyone", Label: "Everyone"},
	{Value: "connections", Label: "Connections only"},
	{Value: "private", Label: "Private (only me)"},
}

// VisibilityOption is one selectable profile-visibility value, used to populate
// the edit form <select> options.
type VisibilityOption struct {
	Value string
	Label string
}

// IsValidVisibility reports whether s is one of the allowed profile_visibility
// values.
func IsValidVisibility(s string) bool {
	for _, o := range VisibilityOptions {
		if o.Value == s {
			return true
		}
	}
	return false
}

// CanViewFullProfile reports whether a viewer may see the full profile of a
// profile owner given the owner's visibility setting and the relationship
// between the two.
//
//   - The owner always sees their own full profile.
//   - Admins (role "admin") may see any full profile.
//   - "everyone": anyone may see the full profile.
//   - "connections": only an accepted connection (connStatus == "connected")
//     may see the full profile.
//   - "private": no one but the owner (and admins) may see the full profile.
//
// connStatus is the value returned by connection.GetConnectionStatus
// ("connected" for an accepted connection).
func CanViewFullProfile(visibility string, isOwner bool, viewerRole, connStatus string) bool {
	if isOwner || viewerRole == "admin" {
		return true
	}
	switch visibility {
	case "connections":
		return connStatus == "connected"
	case "private":
		return false
	default: // "everyone" and any unknown value fall back to fully visible
		return true
	}
}

// LimitedReason returns the human-readable explanation shown on the limited
// profile view for a visibility setting, or "" when the full profile should be
// shown.
func LimitedReason(visibility string) string {
	switch visibility {
	case "connections":
		return "This profile is visible to connections only."
	case "private":
		return "This profile is private."
	default:
		return ""
	}
}

// WorkStatuses is the set of valid work_status values, in display order. It is
// the single source of truth used both to validate form input and to render the
// edit <select>.
var WorkStatuses = []WorkStatusOption{
	{Value: "in_office", Label: "In Office"},
	{Value: "telework", Label: "Telework"},
	{Value: "tdy", Label: "TDY"},
	{Value: "leave", Label: "On Leave"},
}

// WorkStatusOption is one selectable work status, used to populate the edit form
// <select> options.
type WorkStatusOption struct {
	Value string
	Label string
}

// IsValidWorkStatus reports whether s is one of the allowed work_status values.
func IsValidWorkStatus(s string) bool {
	for _, o := range WorkStatuses {
		if o.Value == s {
			return true
		}
	}
	return false
}

// WorkStatusBadge returns an emoji + label for the user's current work status,
// e.g. "🏠 Telework". Defaults to the in-office badge for unknown/empty values.
func (u User) WorkStatusBadge() string {
	switch u.WorkStatus {
	case "telework":
		return "\U0001F3E0 Telework"
	case "tdy":
		return "✈️ TDY"
	case "leave":
		return "\U0001F334 On Leave"
	default:
		return "\U0001F3E2 In Office"
	}
}

// WorkStatusUntilLabel returns the formatted "until" date (e.g. "Jun 3") or ""
// when no end date is set.
func (u User) WorkStatusUntilLabel() string {
	if u.WorkStatusUntil == nil {
		return ""
	}
	return u.WorkStatusUntil.Format("Jan 2")
}

// WorkStatusUntilInput returns the "until" date formatted for an
// <input type="date"> (YYYY-MM-DD), or "" when unset.
func (u User) WorkStatusUntilInput() string {
	if u.WorkStatusUntil == nil {
		return ""
	}
	return u.WorkStatusUntil.Format("2006-01-02")
}

type Experience struct {
	ID          string
	Title       string
	Company     string
	Location    string
	StartDate   time.Time
	EndDate     *time.Time
	Description string
}

type Education struct {
	ID           string
	School       string
	Degree       string
	FieldOfStudy string
	StartYear    int
	EndYear      *int
}

type Skill struct {
	ID               string
	Name             string
	EndorsementCount int
	EndorsedByUser   bool
}

type ProfileData struct {
	middleware.BaseData
	User             User
	Experiences      []Experience
	Educations       []Education
	Skills           []Skill
	IsOwnProfile     bool
	ConnectionStatus string
	Completeness     *ProfileCompleteness
	// Restricted is true when the viewer is not allowed to see the full
	// profile (per the owner's profile_visibility). When set, the template
	// renders a limited view (name + LimitedReason) and details are not
	// loaded.
	Restricted    bool
	LimitedReason string
}

// WorkStatuses exposes the valid work-status options to the edit template so the
// <select> stays in sync with IsValidWorkStatus.
func (d ProfileData) WorkStatuses() []WorkStatusOption {
	return WorkStatuses
}

// Visibilities exposes the valid profile-visibility options to the edit template
// so the <select> stays in sync with IsValidVisibility.
func (d ProfileData) Visibilities() []VisibilityOption {
	return VisibilityOptions
}

// PinnedPost is a view-only representation of a post the profile owner has
// pinned to the top of their profile. It carries the post content plus the
// author display fields, mirroring the columns the feed loads for a post.
type PinnedPost struct {
	ID              string
	AuthorID        string
	AuthorFirstName string
	AuthorLastName  string
	AuthorAvatarURL string
	Content         string
	CreatedAt       time.Time
	PinnedAt        time.Time
}

// Celebration is a view-only entry for the "Celebrations this week" list shown
// on the owner's own profile: a connection whose birthday or work anniversary
// falls in the current week.
type Celebration struct {
	UserID    string
	FirstName string
	LastName  string
	AvatarURL string
	Kind      string // "birthday" or "anniversary"
	Years     int    // only meaningful for anniversary
}

// BirthdayLabel returns the month + day of the user's birthday (no year), e.g.
// "January 5". Empty string when no birthday is set.
func (u User) BirthdayLabel() string {
	if u.Birthday == nil {
		return ""
	}
	return u.Birthday.Format("January 2")
}

// AnniversaryYears returns the number of full years since the user's hire date,
// relative to now. Zero when no hire date is set or the hire date is in the
// future.
func (u User) AnniversaryYears() int {
	if u.HireDate == nil {
		return 0
	}
	now := time.Now()
	years := now.Year() - u.HireDate.Year()
	// Subtract one if this year's anniversary hasn't happened yet.
	anniv := time.Date(now.Year(), u.HireDate.Month(), u.HireDate.Day(), 0, 0, 0, 0, time.UTC)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if today.Before(anniv) {
		years--
	}
	if years < 0 {
		return 0
	}
	return years
}

// HireYear returns the four-digit year of the hire date, or 0 when unset.
func (u User) HireYear() int {
	if u.HireDate == nil {
		return 0
	}
	return u.HireDate.Year()
}

// BirthdayInput returns the birthday formatted for an <input type="date">
// (YYYY-MM-DD), or "" when unset.
func (u User) BirthdayInput() string {
	if u.Birthday == nil {
		return ""
	}
	return u.Birthday.Format("2006-01-02")
}

// HireDateInput returns the hire date formatted for an <input type="date">
// (YYYY-MM-DD), or "" when unset.
func (u User) HireDateInput() string {
	if u.HireDate == nil {
		return ""
	}
	return u.HireDate.Format("2006-01-02")
}

type ExperienceFormData struct {
	middleware.BaseData
	Experience Experience
	Error      string
}

type EducationFormData struct {
	middleware.BaseData
	Education Education
	Error     string
}

type SkillsFormData struct {
	middleware.BaseData
	Skills []Skill
	Error  string
}
