package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	_ "github.com/lib/pq"
)

func main() {
	dbURL := getEnv("DATABASE_URL", "postgres://jobportal:jobportal@localhost:5432/jobportal?sslmode=disable")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	fmt.Println("connected to database")

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Hash the shared password once
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash password: %v", err)
	}
	passwordHash := string(hash)

	// --- Load department IDs ---
	fmt.Println("loading departments...")
	deptIDs, err := loadDepartments(db)
	if err != nil {
		log.Fatalf("failed to load departments: %v", err)
	}
	if len(deptIDs) == 0 {
		log.Fatal("no departments found — run migrations first")
	}
	fmt.Printf("  found %d departments\n", len(deptIDs))

	// --- Create users ---
	fmt.Println("creating users...")
	userIDs, err := seedUsers(db, passwordHash, deptIDs, rng)
	if err != nil {
		log.Fatalf("failed to seed users: %v", err)
	}
	fmt.Printf("  seeded %d users\n", len(userIDs))

	// --- Create skills for users ---
	fmt.Println("creating skills...")
	count, err := seedSkills(db, userIDs, rng)
	if err != nil {
		log.Fatalf("failed to seed skills: %v", err)
	}
	fmt.Printf("  seeded %d skills\n", count)

	// --- Create connections ---
	fmt.Println("creating connections...")
	count, err = seedConnections(db, userIDs, rng)
	if err != nil {
		log.Fatalf("failed to seed connections: %v", err)
	}
	fmt.Printf("  seeded %d connections\n", count)

	// --- Create experience entries ---
	fmt.Println("creating experience entries...")
	count, err = seedExperiences(db, userIDs, rng)
	if err != nil {
		log.Fatalf("failed to seed experiences: %v", err)
	}
	fmt.Printf("  seeded %d experiences\n", count)

	// --- Create education entries ---
	fmt.Println("creating education entries...")
	count, err = seedEducation(db, userIDs, rng)
	if err != nil {
		log.Fatalf("failed to seed education: %v", err)
	}
	fmt.Printf("  seeded %d education entries\n", count)

	// --- Create posts ---
	fmt.Println("creating posts...")
	postIDs, err := seedPosts(db, userIDs, rng)
	if err != nil {
		log.Fatalf("failed to seed posts: %v", err)
	}
	fmt.Printf("  seeded %d posts\n", len(postIDs))

	// --- Create likes ---
	fmt.Println("creating likes...")
	count, err = seedLikes(db, postIDs, userIDs, rng)
	if err != nil {
		log.Fatalf("failed to seed likes: %v", err)
	}
	fmt.Printf("  seeded %d likes\n", count)

	// --- Create comments ---
	fmt.Println("creating comments...")
	count, err = seedComments(db, postIDs, userIDs, rng)
	if err != nil {
		log.Fatalf("failed to seed comments: %v", err)
	}
	fmt.Printf("  seeded %d comments\n", count)

	// --- Create postings ---
	fmt.Println("creating postings...")
	count, err = seedPostings(db, userIDs, rng)
	if err != nil {
		log.Fatalf("failed to seed postings: %v", err)
	}
	fmt.Printf("  seeded %d postings\n", count)

	// --- Create news articles ---
	fmt.Println("creating news articles...")
	count, err = seedNews(db, userIDs, rng)
	if err != nil {
		log.Fatalf("failed to seed news: %v", err)
	}
	fmt.Printf("  seeded %d news articles\n", count)

	// --- Create kudos ---
	fmt.Println("creating kudos...")
	count, err = seedKudos(db, userIDs, rng)
	if err != nil {
		log.Fatalf("failed to seed kudos: %v", err)
	}
	fmt.Printf("  seeded %d kudos\n", count)

	fmt.Println("\ndone! seed data complete.")
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// --- Department loader ---

func loadDepartments(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query(`SELECT id, name FROM departments`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	depts := make(map[string]string) // name -> id
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		depts[name] = id
	}
	return depts, rows.Err()
}

// --- Users ---

type seedUser struct {
	Email      string
	FirstName  string
	LastName   string
	Headline   string
	Location   string
	About      string
	Role       string
	Department string
}

func userList() []seedUser {
	return []seedUser{
		{"maria.garcia@usda.gov", "Maria", "Garcia", "Soil Scientist | NRCS", "Washington, DC", "Soil conservation specialist with 12 years of experience in USDA NRCS programs.", "employee", "NRCS"},
		{"james.wilson@usda.gov", "James", "Wilson", "Forest Ecologist", "Missoula, MT", "Research ecologist focused on wildfire recovery and forest health monitoring.", "employee", "Forest Service"},
		{"sarah.chen@usda.gov", "Sarah", "Chen", "Data Scientist | Agricultural Research", "Beltsville, MD", "Applying machine learning to crop yield prediction and pest detection models.", "employee", "ARS"},
		{"robert.taylor@usda.gov", "Robert", "Taylor", "IT Program Manager", "Fort Collins, CO", "Leading digital transformation initiatives across USDA agencies.", "manager", "NRCS"},
		{"jennifer.martinez@usda.gov", "Jennifer", "Martinez", "Veterinary Epidemiologist", "Ames, IA", "Tracking and preventing animal disease outbreaks across the US.", "employee", "APHIS"},
		{"michael.johnson@usda.gov", "Michael", "Johnson", "Farm Loan Officer", "Des Moines, IA", "Helping farmers and ranchers access USDA financing programs.", "employee", "FSA"},
		{"lisa.brown@usda.gov", "Lisa", "Brown", "Rural Development Specialist", "Little Rock, AR", "Working on broadband expansion and rural infrastructure projects.", "manager", "Rural Development"},
		{"david.lee@usda.gov", "David", "Lee", "GIS Analyst | Remote Sensing", "Salt Lake City, UT", "Satellite imagery analysis for land use change detection.", "employee", "NRCS"},
		{"amanda.davis@usda.gov", "Amanda", "Davis", "Nutritionist | Food Programs", "Alexandria, VA", "Developing nutrition guidelines for SNAP and WIC programs.", "employee", "FNS"},
		{"christopher.moore@usda.gov", "Christopher", "Moore", "Food Safety Inspector", "Atlanta, GA", "Ensuring compliance with federal meat and poultry inspection standards.", "employee", "FSIS"},
		{"patricia.anderson@usda.gov", "Patricia", "Anderson", "Research Geneticist", "Madison, WI", "Developing drought-resistant crop varieties through genomics research.", "employee", "ARS"},
		{"daniel.thomas@usda.gov", "Daniel", "Thomas", "Cybersecurity Analyst", "Kansas City, MO", "Protecting USDA IT infrastructure and responding to security incidents.", "employee", "NRCS"},
		{"rachel.jackson@usda.gov", "Rachel", "Jackson", "Budget Analyst | Strategic Planning", "Washington, DC", "Managing program budgets and performance metrics for USDA operations.", "employee", "FSA"},
		{"kevin.white@usda.gov", "Kevin", "White", "Hydrologist", "Portland, OR", "Watershed modeling and water quality assessment for conservation programs.", "employee", "NRCS"},
		{"stephanie.harris@usda.gov", "Stephanie", "Harris", "Agricultural Economist", "Washington, DC", "Analyzing trade policy impacts on domestic agricultural markets.", "manager", "ARS"},
		{"brian.clark@usda.gov", "Brian", "Clark", "Software Engineer | Cloud Infrastructure", "Raleigh, NC", "Building and maintaining cloud services for USDA digital platforms.", "employee", "NRCS"},
		{"michelle.lewis@usda.gov", "Michelle", "Lewis", "Rangeland Management Specialist", "Boise, ID", "Developing grazing management plans for public lands.", "employee", "Forest Service"},
		{"andrew.walker@usda.gov", "Andrew", "Walker", "Plant Pathologist", "Riverside, CA", "Researching citrus greening disease and biocontrol strategies.", "employee", "APHIS"},
		{"nicole.robinson@usda.gov", "Nicole", "Robinson", "Project Manager | Digital Services", "Denver, CO", "Leading agile delivery of farmer-facing web applications.", "manager", "Rural Development"},
		{"william.king@usda.gov", "William", "King", "Wildlife Biologist", "Lakewood, CO", "Assessing wildlife habitat impacts of conservation reserve programs.", "employee", "NRCS"},
		{"elizabeth.wright@usda.gov", "Elizabeth", "Wright", "Database Administrator", "Kansas City, MO", "Managing enterprise databases and data warehousing for USDA systems.", "employee", "NRCS"},
		{"joseph.young@usda.gov", "Joseph", "Young", "Entomologist | Integrated Pest Management", "Gainesville, FL", "Researching biological control methods for invasive species.", "employee", "ARS"},
		{"ashley.scott@usda.gov", "Ashley", "Scott", "Communications Specialist", "Washington, DC", "Public affairs and stakeholder engagement for USDA programs.", "employee", "FNS"},
		{"matthew.green@usda.gov", "Matthew", "Green", "DevOps Engineer", "Fort Collins, CO", "CI/CD pipelines and infrastructure automation for USDA applications.", "employee", "NRCS"},
		{"laura.adams@usda.gov", "Laura", "Adams", "Conservation Planner", "Lincoln, NE", "Developing conservation plans for producers enrolled in EQIP and CSP.", "employee", "NRCS"},
	}
}

func seedUsers(db *sql.DB, passwordHash string, deptIDs map[string]string, rng *rand.Rand) ([]string, error) {
	users := userList()
	ids := make([]string, 0, len(users))

	for _, u := range users {
		var id string
		deptID, hasDept := deptIDs[u.Department]

		var err error
		if hasDept {
			err = db.QueryRow(`
				INSERT INTO users (id, email, password_hash, first_name, last_name, headline, location, about, role, department_id)
				VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9)
				ON CONFLICT (email) DO UPDATE SET first_name = EXCLUDED.first_name
				RETURNING id`,
				u.Email, passwordHash, u.FirstName, u.LastName, u.Headline, u.Location, u.About, u.Role, deptID,
			).Scan(&id)
		} else {
			err = db.QueryRow(`
				INSERT INTO users (id, email, password_hash, first_name, last_name, headline, location, about, role)
				VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8)
				ON CONFLICT (email) DO UPDATE SET first_name = EXCLUDED.first_name
				RETURNING id`,
				u.Email, passwordHash, u.FirstName, u.LastName, u.Headline, u.Location, u.About, u.Role,
			).Scan(&id)
		}
		if err != nil {
			return nil, fmt.Errorf("user %s: %w", u.Email, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// --- Skills ---

var skillPool = []string{
	"Python", "R", "SQL", "Go", "JavaScript", "AWS", "Azure", "GIS",
	"Remote Sensing", "Machine Learning", "Data Analysis", "PostgreSQL",
	"Docker", "Kubernetes", "Terraform", "Project Management",
	"Agile", "Soil Science", "Plant Pathology", "Entomology",
	"Hydrology", "Conservation Planning", "Budgeting", "Technical Writing",
	"Public Speaking", "Cybersecurity", "Network Security", "USDA Policy",
	"Food Safety", "Nutrition Science", "Genomics", "Bioinformatics",
	"Crop Modeling", "Livestock Management", "Rangeland Ecology",
	"Watershed Management", "Statistics", "Tableau", "Power BI", "Excel",
}

func seedSkills(db *sql.DB, userIDs []string, rng *rand.Rand) (int, error) {
	count := 0
	for _, uid := range userIDs {
		// Each user gets 3-7 random skills
		n := 3 + rng.Intn(5)
		perm := rng.Perm(len(skillPool))
		for i := 0; i < n && i < len(perm); i++ {
			_, err := db.Exec(`
				INSERT INTO skills (id, user_id, name)
				VALUES (gen_random_uuid(), $1, $2)
				ON CONFLICT (user_id, name) DO NOTHING`,
				uid, skillPool[perm[i]],
			)
			if err != nil {
				return 0, fmt.Errorf("skill for user %s: %w", uid, err)
			}
			count++
		}
	}
	return count, nil
}

// --- Connections (~40% of possible pairs, all accepted) ---

func seedConnections(db *sql.DB, userIDs []string, rng *rand.Rand) (int, error) {
	count := 0
	for i := 0; i < len(userIDs); i++ {
		for j := i + 1; j < len(userIDs); j++ {
			if rng.Float64() > 0.40 {
				continue
			}
			_, err := db.Exec(`
				INSERT INTO connections (id, requester_id, addressee_id, status)
				VALUES (gen_random_uuid(), $1, $2, 'accepted')
				ON CONFLICT DO NOTHING`,
				userIDs[i], userIDs[j],
			)
			if err != nil {
				return 0, err
			}
			count++
		}
	}
	return count, nil
}

// --- Experiences ---

func seedExperiences(db *sql.DB, userIDs []string, rng *rand.Rand) (int, error) {
	titles := []struct {
		Title   string
		Company string
	}{
		{"Soil Scientist", "USDA NRCS"},
		{"Research Biologist", "USDA ARS"},
		{"IT Specialist", "USDA OCIO"},
		{"Program Analyst", "USDA FSA"},
		{"Food Inspector", "USDA FSIS"},
		{"Forester", "USDA Forest Service"},
		{"Veterinary Medical Officer", "USDA APHIS"},
		{"Community Development Specialist", "USDA Rural Development"},
		{"Agricultural Statistician", "USDA NASS"},
		{"Software Developer", "Booz Allen Hamilton"},
		{"Data Analyst", "Deloitte"},
		{"GIS Technician", "US Army Corps of Engineers"},
		{"Research Assistant", "Colorado State University"},
		{"Environmental Scientist", "EPA"},
		{"Field Technician", "US Fish and Wildlife Service"},
	}

	locations := []string{
		"Washington, DC", "Fort Collins, CO", "Beltsville, MD",
		"Kansas City, MO", "Portland, OR", "Ames, IA",
	}

	count := 0
	for _, uid := range userIDs {
		// Each user gets 1-3 experiences
		n := 1 + rng.Intn(3)
		perm := rng.Perm(len(titles))
		for i := 0; i < n; i++ {
			t := titles[perm[i]]
			loc := locations[rng.Intn(len(locations))]
			startYear := 2012 + rng.Intn(10)
			startDate := fmt.Sprintf("%d-%02d-01", startYear, 1+rng.Intn(12))

			var endDate *string
			if i > 0 { // first entry is current role (no end date)
				ed := fmt.Sprintf("%d-%02d-01", startYear+1+rng.Intn(3), 1+rng.Intn(12))
				endDate = &ed
			}

			_, err := db.Exec(`
				INSERT INTO experiences (id, user_id, title, company, location, start_date, end_date, description)
				VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7)`,
				uid, t.Title, t.Company, loc, startDate, endDate,
				fmt.Sprintf("Contributed to key projects in %s as %s.", t.Company, strings.ToLower(t.Title)),
			)
			if err != nil {
				return 0, err
			}
			count++
		}
	}
	return count, nil
}

// --- Education ---

func seedEducation(db *sql.DB, userIDs []string, rng *rand.Rand) (int, error) {
	schools := []struct {
		School string
		Degree string
		Field  string
	}{
		{"Colorado State University", "M.S.", "Natural Resources"},
		{"Virginia Tech", "B.S.", "Agricultural Sciences"},
		{"University of Maryland", "Ph.D.", "Plant Science"},
		{"Iowa State University", "M.S.", "Computer Science"},
		{"Texas A&M University", "B.S.", "Animal Science"},
		{"Oregon State University", "M.S.", "Forestry"},
		{"University of Florida", "B.S.", "Entomology"},
		{"Purdue University", "M.S.", "Agricultural Economics"},
		{"University of Georgia", "B.S.", "Food Science"},
		{"North Carolina State University", "M.S.", "Soil Science"},
		{"Cornell University", "B.S.", "Environmental Engineering"},
		{"UC Davis", "Ph.D.", "Data Science"},
	}

	count := 0
	for _, uid := range userIDs {
		// Each user gets 1-2 education entries
		n := 1 + rng.Intn(2)
		perm := rng.Perm(len(schools))
		for i := 0; i < n; i++ {
			s := schools[perm[i]]
			startYear := 2000 + rng.Intn(15)
			endYear := startYear + 2 + rng.Intn(3)
			_, err := db.Exec(`
				INSERT INTO educations (id, user_id, school, degree, field_of_study, start_year, end_year)
				VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6)`,
				uid, s.School, s.Degree, s.Field, startYear, endYear,
			)
			if err != nil {
				return 0, err
			}
			count++
		}
	}
	return count, nil
}

// --- Posts ---

func postContents() []string {
	return []string{
		"Excited to share that our team just completed the 2026 National Resources Inventory update. Three years of field data collection across all 50 states. Proud of everyone involved!",
		"Just wrapped up an amazing training on cloud-native architectures for government systems. If you're interested in modernizing your agency's tech stack, happy to share notes.",
		"Our new crop yield prediction model achieved 94% accuracy on the validation set. Machine learning is transforming how we forecast agricultural output. Paper coming soon!",
		"Reminder: The USDA Innovation Summit is next month in DC. Great opportunity to network and learn about cross-agency initiatives. Who else is attending?",
		"Successfully migrated our legacy Oracle database to PostgreSQL on AWS. 60% cost reduction and better performance. Happy to chat with anyone considering a similar move.",
		"Had a fantastic field day at the Konza Prairie research site. Nothing beats seeing conservation practices in action. The long-term ecological data from this site is invaluable.",
		"Completed my PMP certification this week! Big thanks to my manager and the USDA training program for supporting professional development.",
		"Our APHIS team detected and contained the spotted lanternfly outbreak in the northeast region ahead of schedule. Great example of rapid response and interagency coordination.",
		"Finished building a new dashboard for tracking SNAP participation rates by county. Data visualization makes such a difference for program managers making decisions.",
		"The new USDA cybersecurity framework is rolling out next quarter. All employees should review the updated security awareness materials. Reach out if you have questions.",
		"Proud to announce our Conservation Stewardship Program enrolled 2 million new acres this year. Farmers are embracing sustainable practices like never before.",
		"Just published a peer-reviewed paper on biological control of the emerald ash borer. Open access link in comments. Collaboration between ARS and university partners made this possible.",
		"Hosted a webinar on GIS applications in precision agriculture. Over 200 attendees from across USDA agencies. Recording will be posted on the learning portal next week.",
		"Our DevOps team reduced deployment time from 4 hours to 15 minutes by implementing automated CI/CD pipelines. Small wins add up to big improvements.",
		"Reflecting on 20 years at USDA today. From field technician to branch chief, this agency has given me incredible opportunities to serve American agriculture.",
		"The Rural Broadband Initiative just connected its 500th rural community. Digital equity is essential for modern farming operations and rural quality of life.",
		"New study from our lab shows cover crops reduce soil erosion by up to 90% in test plots. More reason to promote conservation tillage practices.",
		"Attended the Federal IT Modernization conference in Reston. Key takeaway: zero trust architecture is no longer optional for federal agencies.",
	}
}

func seedPosts(db *sql.DB, userIDs []string, rng *rand.Rand) ([]string, error) {
	contents := postContents()
	postIDs := make([]string, 0, len(contents))

	for _, content := range contents {
		authorID := userIDs[rng.Intn(len(userIDs))]
		var id string
		err := db.QueryRow(`
			INSERT INTO posts (id, user_id, content)
			VALUES (gen_random_uuid(), $1, $2)
			RETURNING id`,
			authorID, content,
		).Scan(&id)
		if err != nil {
			return nil, err
		}
		postIDs = append(postIDs, id)
	}
	return postIDs, nil
}

// --- Likes ---

func seedLikes(db *sql.DB, postIDs, userIDs []string, rng *rand.Rand) (int, error) {
	count := 0
	for _, pid := range postIDs {
		// Each post gets 2-8 random likes
		n := 2 + rng.Intn(7)
		perm := rng.Perm(len(userIDs))
		for i := 0; i < n && i < len(perm); i++ {
			_, err := db.Exec(`
				INSERT INTO post_likes (id, post_id, user_id)
				VALUES (gen_random_uuid(), $1, $2)
				ON CONFLICT (post_id, user_id) DO NOTHING`,
				pid, userIDs[perm[i]],
			)
			if err != nil {
				return 0, err
			}
			count++
		}
	}
	return count, nil
}

// --- Comments ---

func seedComments(db *sql.DB, postIDs, userIDs []string, rng *rand.Rand) (int, error) {
	comments := []string{
		"Great work! This is exactly what we need.",
		"Thanks for sharing. Very informative.",
		"Congrats! Well deserved.",
		"Would love to learn more about this. Can we set up a call?",
		"This is impressive. Sharing with my team.",
		"Excellent results! Keep up the great work.",
		"Really interesting findings. How does this compare to last year's data?",
		"Thanks for the update. Looking forward to the next phase.",
		"Our team had a similar experience. Happy to collaborate.",
		"This aligns well with our strategic goals. Nice work!",
		"Amazing progress. Glad to see cross-agency cooperation working.",
		"Can you share the methodology? Would be useful for our program.",
	}

	count := 0
	for _, pid := range postIDs {
		// Each post gets 0-4 comments
		n := rng.Intn(5)
		for i := 0; i < n; i++ {
			commenterID := userIDs[rng.Intn(len(userIDs))]
			commentText := comments[rng.Intn(len(comments))]
			_, err := db.Exec(`
				INSERT INTO comments (id, post_id, user_id, content)
				VALUES (gen_random_uuid(), $1, $2, $3)`,
				pid, commenterID, commentText,
			)
			if err != nil {
				return 0, err
			}
			count++
		}
	}
	return count, nil
}

// --- Postings ---

func seedPostings(db *sql.DB, userIDs []string, rng *rand.Rand) (int, error) {
	// Find manager/admin users for authoring postings
	managerIDs := make([]string, 0)
	for _, uid := range userIDs {
		var role string
		err := db.QueryRow(`SELECT role FROM users WHERE id = $1`, uid).Scan(&role)
		if err != nil {
			continue
		}
		if role == "manager" || role == "admin" {
			managerIDs = append(managerIDs, uid)
		}
	}
	if len(managerIDs) == 0 {
		// Fall back to first few users
		managerIDs = userIDs[:3]
	}

	type postingData struct {
		Title       string
		Description string
		Type        string
		Location    string
		Department  string
		Skills      []string
	}

	postings := []postingData{
		{
			"Cloud Migration Engineer",
			"Looking for an experienced engineer to lead migration of legacy NRCS systems to AWS GovCloud. Must have FedRAMP experience and active security clearance. This is a 12-month detail opportunity.",
			"detail",
			"Fort Collins, CO",
			"NRCS",
			[]string{"AWS", "Docker", "Kubernetes", "Terraform", "PostgreSQL"},
		},
		{
			"Soil Health Monitoring Dashboard",
			"Project to build an interactive dashboard for visualizing soil health metrics across conservation districts. Need GIS and data visualization expertise. 6-month project starting Q2.",
			"project",
			"Washington, DC",
			"NRCS",
			[]string{"GIS", "Python", "Data Analysis", "Tableau"},
		},
		{
			"Data Scientist - Crop Prediction Models",
			"Seeking a data scientist to join our team building next-generation crop yield prediction models using satellite imagery and weather data. Remote-friendly position.",
			"project",
			"Beltsville, MD",
			"ARS",
			[]string{"Machine Learning", "Python", "R", "Remote Sensing", "Statistics"},
		},
		{
			"Rural Broadband Mapping Initiative",
			"Cross-agency project to map broadband coverage gaps in rural communities. Need analysts with GIS and policy experience. Travel to field sites required.",
			"project",
			"Washington, DC",
			"Rural Development",
			[]string{"GIS", "Data Analysis", "Project Management", "SQL"},
		},
		{
			"Cybersecurity Incident Response Lead",
			"Hiring a senior cybersecurity analyst to lead our incident response team. Requires CISSP or equivalent certification and experience with federal IT security frameworks.",
			"detail",
			"Kansas City, MO",
			"NRCS",
			[]string{"Cybersecurity", "Network Security", "Python", "SQL"},
		},
	}

	count := 0
	for _, p := range postings {
		authorID := managerIDs[rng.Intn(len(managerIDs))]
		var postingID string
		err := db.QueryRow(`
			INSERT INTO postings (id, author_id, title, description, type, location, department, status)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, 'active')
			ON CONFLICT DO NOTHING
			RETURNING id`,
			authorID, p.Title, p.Description, p.Type, p.Location, p.Department,
		).Scan(&postingID)
		if err != nil {
			// Might be a duplicate on re-run; skip
			continue
		}

		for _, skill := range p.Skills {
			_, err := db.Exec(`
				INSERT INTO posting_skills (posting_id, skill_name)
				VALUES ($1, $2)
				ON CONFLICT DO NOTHING`,
				postingID, skill,
			)
			if err != nil {
				return 0, err
			}
		}
		count++
	}
	return count, nil
}

// --- News Articles ---

func seedNews(db *sql.DB, userIDs []string, rng *rand.Rand) (int, error) {
	// Find admin users for authoring news
	adminIDs := make([]string, 0)
	for _, uid := range userIDs {
		var role string
		if err := db.QueryRow(`SELECT role FROM users WHERE id = $1`, uid).Scan(&role); err != nil {
			continue
		}
		if role == "admin" {
			adminIDs = append(adminIDs, uid)
		}
	}
	// Also check for any admin that already exists (e.g., seed admin)
	rows, err := db.Query(`SELECT id FROM users WHERE role = 'admin' LIMIT 5`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err == nil {
				adminIDs = append(adminIDs, id)
			}
		}
	}
	if len(adminIDs) == 0 {
		adminIDs = userIDs[:1] // fallback
	}

	articles := []struct {
		Title   string
		Content string
	}{
		{
			"USDA Announces New Climate-Smart Agriculture Initiative",
			"The USDA today announced a comprehensive climate-smart agriculture initiative that will invest $3 billion in conservation practices over the next five years. The program will support farmers and ranchers in adopting cover crops, reduced tillage, and improved nutrient management practices that sequester carbon and reduce greenhouse gas emissions.\n\nSecretary Vilsack emphasized that this initiative builds on existing conservation programs while introducing new incentives for measurable climate outcomes. NRCS field offices will begin accepting applications next month.",
		},
		{
			"IT Modernization Update: New Employee Portal Launching in Q3",
			"The Office of the CIO is pleased to announce that the new USDA Employee Portal will launch in Q3 2026. The portal consolidates access to HR systems, training resources, and collaboration tools into a single modern interface.\n\nKey features include single sign-on, a personalized dashboard, and mobile-friendly design. All employees will receive training access two weeks before launch. For questions, contact the IT Service Desk.",
		},
		{
			"Annual Performance Review Cycle Opens March 15",
			"The FY2026 annual performance review cycle opens on March 15. All supervisors should schedule mid-year check-ins with their direct reports before the end of the month. The new performance management system includes a streamlined self-assessment form and 360-degree feedback option.\n\nReminder: Accomplishment tracking through JobPortal can help document your contributions throughout the year. Start building your record now to make review time easier.",
		},
	}

	count := 0
	for _, a := range articles {
		authorID := adminIDs[rng.Intn(len(adminIDs))]
		_, err := db.Exec(`
			INSERT INTO news_articles (id, author_id, title, content, published)
			VALUES (gen_random_uuid(), $1, $2, $3, true)`,
			authorID, a.Title, a.Content,
		)
		if err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
}

// --- Kudos ---

func seedKudos(db *sql.DB, userIDs []string, rng *rand.Rand) (int, error) {
	messages := []string{
		"Thanks for staying late to help debug the deployment issue last week. Your expertise saved us hours!",
		"Great job leading the soil health workshop. The field staff really appreciated your clear explanations.",
		"Your data visualization work on the quarterly report was outstanding. Made the complex data accessible to everyone.",
		"Appreciate you mentoring the new team members. Your patience and knowledge make a real difference.",
		"Excellent work on the security audit findings. Your thoroughness keeps our systems safe.",
		"Thank you for coordinating the cross-agency meeting so smoothly. Not easy with 50 participants!",
		"Your conservation plan for the Johnson farm was exemplary. The producer was thrilled with the results.",
		"Kudos for getting the CI/CD pipeline working over the weekend. Deployments are so much smoother now.",
	}

	count := 0
	for i := 0; i < 8; i++ {
		senderIdx := rng.Intn(len(userIDs))
		receiverIdx := rng.Intn(len(userIDs))
		for receiverIdx == senderIdx {
			receiverIdx = rng.Intn(len(userIDs))
		}
		_, err := db.Exec(`
			INSERT INTO kudos (id, sender_id, receiver_id, message)
			VALUES (gen_random_uuid(), $1, $2, $3)`,
			userIDs[senderIdx], userIDs[receiverIdx], messages[i],
		)
		if err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
}
