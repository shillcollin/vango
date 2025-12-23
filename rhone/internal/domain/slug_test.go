package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vangoframework/rhone/internal/domain"
)

func TestValidateSlug(t *testing.T) {
	tests := []struct {
		name    string
		slug    string
		wantErr error
	}{
		// Valid slugs
		{"valid simple", "my-app", nil},
		{"valid with numbers", "my-app-123", nil},
		{"valid minimum length", "abc", nil},
		{"valid maximum length", "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz1", nil},
		{"valid single hyphen", "a-b", nil},

		// Invalid: too short
		{"too short 1 char", "a", domain.ErrSlugTooShort},
		{"too short 2 chars", "ab", domain.ErrSlugTooShort},

		// Invalid: too long
		{"too long", "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz12", domain.ErrSlugTooLong},

		// Invalid: wrong start
		{"starts with number", "1my-app", domain.ErrSlugInvalidStart},
		{"starts with hyphen", "-my-app", domain.ErrSlugInvalidStart},
		{"starts with uppercase", "My-app", domain.ErrSlugInvalidStart},

		// Invalid: wrong end
		{"ends with hyphen", "my-app-", domain.ErrSlugInvalidEnd},

		// Invalid: consecutive hyphens
		{"consecutive hyphens", "my--app", domain.ErrSlugConsecutiveHyphens},
		{"triple hyphens", "my---app", domain.ErrSlugConsecutiveHyphens},

		// Invalid: wrong characters
		{"uppercase letters", "MY-APP", domain.ErrSlugInvalidStart},
		{"special characters", "my_app", domain.ErrSlugInvalidChars},
		{"spaces", "my app", domain.ErrSlugInvalidChars},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateSlug(tt.slug)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple name", "My App", "my-app"},
		{"with spaces", "My Cool App", "my-cool-app"},
		{"with underscores", "my_cool_app", "my-cool-app"},
		{"with special chars", "My App! @#$ Test", "my-app-test"},
		{"with numbers", "App 123", "app-123"},
		{"leading numbers", "123 My App", "my-app"},
		{"only numbers", "123456", ""},
		{"consecutive spaces", "My  App", "my-app"},
		{"consecutive hyphens input", "My--App", "my-app"},
		{"leading hyphen", "-My App", "my-app"},
		{"trailing hyphen", "My App-", "my-app"},
		{"short name", "ab", "ab-app"},
		{"single char", "a", "a-app"},
		{"empty string", "", ""},
		{"mixed case", "MyAwesomeApp", "myawesomeapp"},
		{"long name truncated", "This Is A Very Long Application Name That Exceeds The Maximum Length Allowed By The System", "this-is-a-very-long-application-name-that-exceeds-the-maximum-l"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := domain.GenerateSlug(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGeneratedSlugsAreValid(t *testing.T) {
	// Slugs generated from valid names should pass validation
	names := []string{
		"My App",
		"Cool Project",
		"Test 123",
		"Hello World",
		"API Server",
	}

	for _, name := range names {
		slug := domain.GenerateSlug(name)
		if slug == "" {
			continue // Empty slugs are valid for empty/invalid input
		}
		err := domain.ValidateSlug(slug)
		assert.NoError(t, err, "Generated slug %q from %q should be valid", slug, name)
	}
}
