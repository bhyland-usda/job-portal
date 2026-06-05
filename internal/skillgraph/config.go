package skillgraph

import (
	"os"
	"strings"
)

// Enabled reports whether graph-based skill matching should run.
func Enabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GRAPH_MATCHING_ENABLED"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
