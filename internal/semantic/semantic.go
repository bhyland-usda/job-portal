package semantic

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	// Dimensions is the embedding length stored in Postgres vector columns.
	Dimensions = 256

	MatchingModeCookie  = "matching_mode"
	LocationTypesCookie = "preferred_location_types"

	EntityTypeOpportunity = "opportunity"
	EntityTypeWorkspace   = "workspace"
)

type LocationTypePreference struct {
	Remote bool
	Hybrid bool
	Onsite bool
}

func DefaultLocationTypePreference() LocationTypePreference {
	return LocationTypePreference{Remote: true, Hybrid: true, Onsite: true}
}

func (p LocationTypePreference) IsAll() bool {
	return p.Remote && p.Hybrid && p.Onsite
}

func (p LocationTypePreference) Selected() []string {
	selected := make([]string, 0, 3)
	if p.Remote {
		selected = append(selected, "remote")
	}
	if p.Hybrid {
		selected = append(selected, "hybrid")
	}
	if p.Onsite {
		selected = append(selected, "onsite")
	}
	return selected
}

func (p LocationTypePreference) Label() string {
	if p.IsAll() {
		return "All"
	}
	labels := make([]string, 0, 3)
	if p.Remote {
		labels = append(labels, "Remote")
	}
	if p.Hybrid {
		labels = append(labels, "Hybrid")
	}
	if p.Onsite {
		labels = append(labels, "On-Site")
	}
	if len(labels) == 0 {
		return "All"
	}
	return strings.Join(labels, ", ")
}

// Enabled reports whether semantic matching should be attempted.
// Keeping this behind a flag lets existing keyword flows continue unchanged
// until the vector path is explicitly enabled per environment.
func Enabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SEMANTIC_MATCHING_ENABLED"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// EnabledForRequest applies the runtime feature flag first and then a
// per-user cookie override. Default behavior when enabled is "on".
func EnabledForRequest(r *http.Request) bool {
	if !Enabled() {
		return false
	}

	c, err := r.Cookie(MatchingModeCookie)
	if err != nil {
		return true
	}

	switch strings.ToLower(strings.TrimSpace(c.Value)) {
	case "off", "0", "false", "no":
		return false
	default:
		return true
	}
}

// SetPreferenceCookie persists the user's matching-mode preference across
// browser and server restarts.
func SetPreferenceCookie(w http.ResponseWriter, r *http.Request, enabled bool) {
	value := "off"
	if enabled {
		value = "on"
	}

	secure := r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
	http.SetCookie(w, &http.Cookie{
		Name:     MatchingModeCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})
}

// LocationTypePreferenceForRequest resolves the user's persisted location-type
// selections. Missing/invalid values default to all location types.
func LocationTypePreferenceForRequest(r *http.Request) LocationTypePreference {
	defaultPref := DefaultLocationTypePreference()

	c, err := r.Cookie(LocationTypesCookie)
	if err != nil {
		return defaultPref
	}

	raw := strings.ToLower(strings.TrimSpace(c.Value))
	if raw == "" || raw == "all" {
		return defaultPref
	}

	parsed := LocationTypePreference{}
	for _, part := range strings.Split(raw, ",") {
		switch strings.TrimSpace(part) {
		case "remote":
			parsed.Remote = true
		case "hybrid":
			parsed.Hybrid = true
		case "onsite":
			parsed.Onsite = true
		}
	}

	if len(parsed.Selected()) == 0 {
		return defaultPref
	}
	return parsed
}

func SetLocationTypePreferenceCookie(w http.ResponseWriter, r *http.Request, pref LocationTypePreference) {
	value := "all"
	if !pref.IsAll() {
		selected := pref.Selected()
		if len(selected) == 0 {
			pref = DefaultLocationTypePreference()
		} else {
			value = strings.Join(selected, ",")
		}
	}

	secure := r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
	http.SetCookie(w, &http.Cookie{
		Name:     LocationTypesCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})
}

// GenerateEmbedding deterministically projects text into a fixed-size vector.
// It is intentionally local and dependency-free so the vector DB path can be
// adopted before introducing an external embeddings service.
func GenerateEmbedding(text string) []float32 {
	vec := make([]float32, Dimensions)
	tokens := strings.Fields(strings.ToLower(strings.TrimSpace(text)))
	if len(tokens) == 0 {
		return vec
	}

	for _, token := range tokens {
		h := fnv.New64a()
		_, _ = h.Write([]byte(token))
		sum := h.Sum64()

		idx := int(sum % uint64(Dimensions))
		sign := float32(1.0)
		if (sum>>63)&1 == 1 {
			sign = -1.0
		}
		vec[idx] += sign
	}

	// L2 normalize to make cosine distance stable across different lengths.
	norm := float32(0)
	for _, v := range vec {
		norm += v * v
	}
	if norm == 0 {
		return vec
	}
	inv := 1 / float32(math.Sqrt(float64(norm)))
	for i := range vec {
		vec[i] *= inv
	}
	return vec
}

// ToPGVectorLiteral formats an embedding for SQL casts such as $1::vector.
func ToPGVectorLiteral(vec []float32) string {
	if len(vec) == 0 {
		return "[]"
	}

	var b strings.Builder
	b.Grow(len(vec) * 8)
	b.WriteByte('[')
	for i, v := range vec {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(v), 'f', 6, 32))
	}
	b.WriteByte(']')
	return b.String()
}

// UpsertEntityEmbedding writes an embedding for one semantic entity.
func UpsertEntityEmbedding(ctx context.Context, db *sql.DB, entityType, entityID, sourceText string, embedding []float32) error {
	if strings.TrimSpace(entityType) == "" || strings.TrimSpace(entityID) == "" {
		return fmt.Errorf("entity_type and entity_id are required")
	}
	if strings.TrimSpace(sourceText) == "" {
		return fmt.Errorf("source_text is required")
	}

	_, err := db.ExecContext(ctx,
		`INSERT INTO semantic_embeddings (entity_type, entity_id, source_text, embedding, updated_at)
         VALUES ($1, $2, $3, $4::vector, NOW())
         ON CONFLICT (entity_type, entity_id)
         DO UPDATE SET source_text = EXCLUDED.source_text,
                       embedding = EXCLUDED.embedding,
                       updated_at = NOW()`,
		entityType, entityID, sourceText, ToPGVectorLiteral(embedding),
	)
	return err
}

// BuildOpportunitySourceText loads and assembles the text corpus used for an
// opportunity embedding.
func BuildOpportunitySourceText(ctx context.Context, db *sql.DB, opportunityID string) (string, error) {
	var title, description, location, department, learningOutcomes, skills sql.NullString
	err := db.QueryRowContext(ctx,
		`SELECT p.title,
		        p.description,
		        p.location,
		        p.department,
		        p.learning_outcomes,
		        COALESCE(string_agg(ps.skill_name, ' '), '')
		 FROM postings p
		 LEFT JOIN posting_skills ps ON ps.posting_id = p.id
		 WHERE p.id = $1
		 GROUP BY p.id, p.title, p.description, p.location, p.department, p.learning_outcomes`,
		opportunityID,
	).Scan(&title, &description, &location, &department, &learningOutcomes, &skills)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(strings.Join([]string{
		title.String,
		description.String,
		location.String,
		department.String,
		learningOutcomes.String,
		skills.String,
	}, "\n")), nil
}

// BuildWorkspaceSourceText loads and assembles the text corpus used for a
// workspace embedding.
func BuildWorkspaceSourceText(ctx context.Context, db *sql.DB, workspaceID string) (string, error) {
	var name, description, meetingFrequency, primaryAudience, howToJoin sql.NullString
	err := db.QueryRowContext(ctx,
		`SELECT name,
		        COALESCE(description, ''),
		        COALESCE(meeting_frequency, ''),
		        COALESCE(primary_audience, ''),
		        COALESCE(how_to_join, '')
		 FROM workspaces WHERE id = $1`,
		workspaceID,
	).Scan(&name, &description, &meetingFrequency, &primaryAudience, &howToJoin)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(strings.Join([]string{
		name.String,
		description.String,
		meetingFrequency.String,
		primaryAudience.String,
		howToJoin.String,
	}, "\n")), nil
}

// UpsertOpportunityEmbeddingByID regenerates and stores the embedding for one
// opportunity record.
func UpsertOpportunityEmbeddingByID(ctx context.Context, db *sql.DB, opportunityID string) error {
	source, err := BuildOpportunitySourceText(ctx, db, opportunityID)
	if err != nil {
		return err
	}
	if source == "" {
		return nil
	}
	return UpsertEntityEmbedding(ctx, db, EntityTypeOpportunity, opportunityID, source, GenerateEmbedding(source))
}

// UpsertWorkspaceEmbeddingByID regenerates and stores the embedding for one
// workspace record.
func UpsertWorkspaceEmbeddingByID(ctx context.Context, db *sql.DB, workspaceID string) error {
	source, err := BuildWorkspaceSourceText(ctx, db, workspaceID)
	if err != nil {
		return err
	}
	if source == "" {
		return nil
	}
	return UpsertEntityEmbedding(ctx, db, EntityTypeWorkspace, workspaceID, source, GenerateEmbedding(source))
}

// BackfillOpportunityEmbeddings regenerates embeddings for all opportunities.
func BackfillOpportunityEmbeddings(ctx context.Context, db *sql.DB) (int, error) {
	rows, err := db.QueryContext(ctx, `SELECT id FROM postings`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	updated := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return updated, err
		}
		if err := UpsertOpportunityEmbeddingByID(ctx, db, id); err != nil {
			return updated, err
		}
		updated++
	}
	if err := rows.Err(); err != nil {
		return updated, err
	}
	return updated, nil
}

// BackfillWorkspaceEmbeddings regenerates embeddings for all workspaces.
func BackfillWorkspaceEmbeddings(ctx context.Context, db *sql.DB) (int, error) {
	rows, err := db.QueryContext(ctx, `SELECT id FROM workspaces`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	updated := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return updated, err
		}
		if err := UpsertWorkspaceEmbeddingByID(ctx, db, id); err != nil {
			return updated, err
		}
		updated++
	}
	if err := rows.Err(); err != nil {
		return updated, err
	}
	return updated, nil
}
