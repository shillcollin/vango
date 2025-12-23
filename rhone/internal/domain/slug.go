package domain

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
)

// Slug validation errors
var (
	ErrSlugTooShort           = errors.New("slug must be at least 3 characters")
	ErrSlugTooLong            = errors.New("slug must be at most 63 characters")
	ErrSlugInvalidChars       = errors.New("slug must contain only lowercase letters, numbers, and hyphens")
	ErrSlugInvalidStart       = errors.New("slug must start with a lowercase letter")
	ErrSlugInvalidEnd         = errors.New("slug must end with a lowercase letter or number")
	ErrSlugConsecutiveHyphens = errors.New("slug cannot contain consecutive hyphens")
)

// slugRegex validates a properly formatted slug:
// - Starts with lowercase letter
// - Ends with lowercase letter or digit
// - Contains only lowercase letters, digits, and hyphens
var slugRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)

// ValidateSlug validates a slug according to the rules:
// - 3-63 characters
// - Lowercase alphanumeric + hyphens only
// - Must start with a lowercase letter
// - Must end with a lowercase letter or digit
// - No consecutive hyphens
func ValidateSlug(slug string) error {
	if len(slug) < 3 {
		return ErrSlugTooShort
	}
	if len(slug) > 63 {
		return ErrSlugTooLong
	}

	// Check for consecutive hyphens first
	if strings.Contains(slug, "--") {
		return ErrSlugConsecutiveHyphens
	}

	// Check if it matches the regex
	if !slugRegex.MatchString(slug) {
		// Provide more specific error messages
		firstChar := rune(slug[0])
		if !unicode.IsLower(firstChar) || !unicode.IsLetter(firstChar) {
			return ErrSlugInvalidStart
		}

		lastChar := rune(slug[len(slug)-1])
		if !unicode.IsLower(lastChar) && !unicode.IsDigit(lastChar) {
			return ErrSlugInvalidEnd
		}

		return ErrSlugInvalidChars
	}

	return nil
}

// GenerateSlug creates a URL-safe slug from a name.
// Rules:
// - Converts to lowercase
// - Replaces spaces and underscores with hyphens
// - Removes invalid characters
// - Collapses consecutive hyphens
// - Ensures it starts with a letter
// - Truncates to 63 characters
// - Ensures minimum length of 3 characters
func GenerateSlug(name string) string {
	if name == "" {
		return ""
	}

	// Convert to lowercase
	slug := strings.ToLower(name)

	// Replace spaces and underscores with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")

	// Keep only valid characters (lowercase letters, digits, hyphens)
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	slug = result.String()

	// Collapse consecutive hyphens
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	// Trim leading and trailing hyphens
	slug = strings.Trim(slug, "-")

	// Ensure starts with a letter (remove leading digits and hyphens)
	for len(slug) > 0 && !((slug[0] >= 'a' && slug[0] <= 'z')) {
		slug = slug[1:]
	}

	// Trim leading hyphens again after removing digits
	slug = strings.TrimLeft(slug, "-")

	// Truncate to 63 characters
	if len(slug) > 63 {
		slug = slug[:63]
	}

	// Trim trailing hyphen after truncation
	slug = strings.TrimRight(slug, "-")

	// Ensure minimum length by appending "-app" if needed
	if len(slug) < 3 && len(slug) > 0 {
		slug = slug + "-app"
	}

	return slug
}
