package utils

import "strings"

// GenerateSlug produces a URL-safe ASCII slug from a title.
// Arabic/Unicode characters are dropped; spaces and hyphens become a single hyphen.
func GenerateSlug(title string) string {
	var b strings.Builder
	for _, r := range title {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + 32)
		case r == ' ' || r == '-':
			b.WriteByte('-')
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "item"
	}
	return slug
}
