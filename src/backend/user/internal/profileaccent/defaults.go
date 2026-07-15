package profileaccent

import (
	"fmt"
	"regexp"
	"strings"
)

// DefaultPalette matches design/tokens/voice.tokens.json profileAccent.defaults.
var DefaultPalette = []string{
	"#7EC8E3",
	"#9ED9A6",
	"#F0A8A8",
	"#F5E6A3",
	"#FFCC99",
	"#C9B8FF",
	"#FFB3E6",
}

var hexColorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// At returns a palette color for the given zero-based profile index.
func At(index int) string {
	if len(DefaultPalette) == 0 {
		return "#7EC8E3"
	}
	if index < 0 {
		index = 0
	}
	return DefaultPalette[index%len(DefaultPalette)]
}

// Normalize validates and uppercases a #RRGGBB accent color.
func Normalize(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if !hexColorRe.MatchString(s) {
		return "", fmt.Errorf("accent_color must be #RRGGBB")
	}
	return strings.ToUpper(s), nil
}
