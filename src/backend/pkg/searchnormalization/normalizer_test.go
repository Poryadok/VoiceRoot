package searchnormalization

import "testing"

func TestV1GoldenVectors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "nfkc and whitespace", in: "  Ａlice  ", want: "alice"},
		{name: "cyrillic confusables", in: "раураl", want: "paypal"},
		{name: "greek confusables", in: "ΡΑΥΡΑL", want: "paypal"},
		{name: "combining mark", in: "e\u0301clair", want: "éclair"},
		{name: "punctuation removed", in: "a.li_ce-1", want: "alice1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := V1.Normalize(tt.in); got != tt.want {
				t.Fatalf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
