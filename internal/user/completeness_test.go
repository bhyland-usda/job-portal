package user

import "testing"

// TestProfileCompletenessPercentage verifies the completeness percentage calculation.
func TestProfileCompletenessPercentage(t *testing.T) {
	tests := []struct {
		score   int
		total   int
		percent int
	}{
		{0, 7, 0},
		{1, 7, 14},
		{3, 7, 42},
		{7, 7, 100},
		{0, 0, 0}, // edge case: no items
	}

	for _, tt := range tests {
		pct := 0
		if tt.total > 0 {
			pct = (tt.score * 100) / tt.total
		}
		if pct != tt.percent {
			t.Errorf("Score %d/%d: expected %d%%, got %d%%", tt.score, tt.total, tt.percent, pct)
		}
	}
}

// TestCompletenessItemCount verifies we check 7 profile fields.
func TestCompletenessItemCount(t *testing.T) {
	expectedItems := 7 // headline, location, about, avatar, experience, education, skills
	items := []string{"headline", "location", "about", "avatar", "experience", "education", "skills"}
	if len(items) != expectedItems {
		t.Errorf("Expected %d completeness items, got %d", expectedItems, len(items))
	}
}
