package utils

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/item"
)

// GrandCharmFingerprint returns a deterministic, restart-safe fingerprint
// for a MAGIC Grand Charm based on its affixes and stats (not UnitID).
func GrandCharmFingerprint(it data.Item) string {
	// Defensive guard
	if it.Name != "GrandCharm" || it.Quality != item.QualityMagic {
		return ""
	}

	var parts []string

	// Base identity
	parts = append(parts, "GC")

	// Identified name (stable across games)
	if it.IdentifiedName != "" {
		parts = append(parts, it.IdentifiedName)
	}

	// Magic affixes (prefixes & suffixes)
	for _, p := range it.Affixes.Magic.Prefixes {
		if p != 0 {
			parts = append(parts, fmt.Sprintf("P%d", p))
		}
	}
	for _, s := range it.Affixes.Magic.Suffixes {
		if s != 0 {
			parts = append(parts, fmt.Sprintf("S%d", s))
		}
	}

	// Serialize stats deterministically
	stats := make([]string, 0, len(it.Stats))
	for _, st := range it.Stats {
		stats = append(stats, fmt.Sprintf(
			"%d:%d:%d",
			st.ID,    // stat ID (THIS WAS THE BUG)
			st.Layer, // layer
			st.Value, // value
		))
	}

	// Order-independent
	sort.Strings(stats)

	for _, s := range stats {
		parts = append(parts, s)
	}

	return strings.Join(parts, "|")
}
