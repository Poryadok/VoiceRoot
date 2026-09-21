// Package searchnormalization defines versioned search-key normalization shared by Go services.
package searchnormalization

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const Version1 = 1

// Normalizer is a stable, versioned producer of search keys.
type Normalizer struct{ version int }

func (n Normalizer) Version() int { return n.version }

// V1 is the currently supported NFKC and confusable-folding normalizer.
var V1 = Normalizer{version: Version1}

var v1Confusables = map[rune]rune{
	'а': 'a', 'е': 'e', 'о': 'o', 'р': 'p', 'с': 'c', 'у': 'y', 'х': 'x',
	'А': 'a', 'В': 'b', 'Е': 'e', 'К': 'k', 'М': 'm', 'Н': 'h', 'О': 'o',
	'Р': 'p', 'С': 'c', 'Т': 't', 'Х': 'x',
	'Α': 'a', 'Β': 'b', 'Ε': 'e', 'Η': 'h', 'Ι': 'i', 'Κ': 'k', 'Μ': 'm',
	'Ν': 'n', 'Ο': 'o', 'Ρ': 'p', 'Τ': 't', 'Υ': 'y', 'Χ': 'x',
	'α': 'a', 'β': 'b', 'ε': 'e', 'η': 'h', 'ι': 'i', 'κ': 'k', 'μ': 'm',
	'ν': 'n', 'ο': 'o', 'ρ': 'p', 'τ': 't', 'υ': 'y', 'χ': 'x',
}

// Normalize returns a search key without relying on database collation or functions.
func (n Normalizer) Normalize(value string) string {
	value = norm.NFKC.String(strings.ToLower(strings.TrimSpace(value)))
	var out strings.Builder
	for _, r := range value {
		if folded, ok := v1Confusables[r]; ok {
			out.WriteRune(folded)
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out.WriteRune(r)
		}
	}
	return out.String()
}
