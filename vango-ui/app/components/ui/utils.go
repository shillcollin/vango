package ui

import "strings"

// CN merges class lists. Future versions may include conflict resolution (tailwind-merge).
// It performs simple string joining and deduplication of exact matches.
func CN(inputs ...string) string {
	var classes []string
	seen := make(map[string]bool)

	for _, input := range inputs {
		// Split by space to handle multiple classes in one string
		parts := strings.Fields(input)
		for _, part := range parts {
			if !seen[part] {
				classes = append(classes, part)
				seen[part] = true
			}
		}
	}
	return strings.Join(classes, " ")
}
