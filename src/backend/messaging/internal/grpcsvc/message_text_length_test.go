package grpcsvc

import (
	"strings"
	"testing"
)

func TestMessageTextLengthCountsUnicodeCodePoints(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "ASCII at documented limit", content: strings.Repeat("a", 4000), want: true},
		{name: "ASCII over documented limit", content: strings.Repeat("a", 4001), want: false},
		{name: "reported Cyrillic case", content: strings.Repeat("я", 2001), want: true},
		{name: "Cyrillic at documented limit", content: strings.Repeat("я", 4000), want: true},
		{name: "Cyrillic over documented limit", content: strings.Repeat("я", 4001), want: false},
		{name: "non-BMP at documented limit", content: strings.Repeat("😀", 4000), want: true},
		{name: "non-BMP over documented limit", content: strings.Repeat("😀", 4001), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := messageTextWithinLimit(tt.content); got != tt.want {
				t.Fatalf("messageTextWithinLimit() = %t, want %t", got, tt.want)
			}
		})
	}
}
