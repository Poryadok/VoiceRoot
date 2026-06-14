package store

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// homoglyphMap maps confusable Cyrillic/Greek chars to Latin for spoof detection.
var homoglyphMap = map[rune]rune{
	'а': 'a', 'е': 'e', 'о': 'o', 'р': 'p', 'с': 'c', 'у': 'y', 'х': 'x',
	'А': 'a', 'В': 'b', 'Е': 'e', 'К': 'k', 'М': 'm', 'Н': 'h', 'О': 'o',
	'Р': 'p', 'С': 'c', 'Т': 't', 'Х': 'x',
}

// NormalizeUsernameKey applies NFKC and homoglyph folding for uniqueness/spoof checks.
func NormalizeUsernameKey(username string) string {
	s := norm.NFKC.String(strings.ToLower(strings.TrimSpace(username)))
	var b strings.Builder
	for _, r := range s {
		if mapped, ok := homoglyphMap[r]; ok {
			b.WriteRune(mapped)
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
