package user

import "time"

type User struct {
	ID        string
	Email     string
	FirstName string
	LastName  string
	Headline  string
	About     string
	AvatarURL string
	Location  string
	CreatedAt time.Time
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
	ID   string
	Name string
}

type ProfileData struct {
	UserID       	 string
	User         	 User
	Experiences  	 []Experience
	Educations   	 []Education
	Skills       	 []Skill
	IsOwnProfile 	 bool
	ConnectionStatus string
}
