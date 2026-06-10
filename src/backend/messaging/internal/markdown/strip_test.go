package markdown

import "testing"

func TestStripForPreview_basicEmphasis(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"bold", "**bold**", "bold"},
		{"italic", "*italic*", "italic"},
		{"underline", "__underline__", "underline"},
		{"strikethrough", "~~strike~~", "strike"},
		{"spoiler", "||secret||", "secret"},
		{"inline code", "`code`", "code"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := StripForPreview(tt.in); got != tt.want {
				t.Fatalf("StripForPreview(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestStripForPreview_links(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"markdown link label", "[Voice](https://voice.app)", "Voice"},
		{"autolink url", "see https://voice.app/docs", "see https://voice.app/docs"},
		{"mixed", "read [docs](https://voice.app) or https://voice.app", "read docs or https://voice.app"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := StripForPreview(tt.in); got != tt.want {
				t.Fatalf("StripForPreview(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestStripForPreview_blocks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"fenced code",
			"```go\nfmt.Println(\"hi\")\n```",
			"fmt.Println(\"hi\")",
		},
		{
			"header",
			"# Title\nbody",
			"Title\nbody",
		},
		{
			"blockquote",
			"> quoted line",
			"quoted line",
		},
		{
			"bullet list",
			"- one\n- two",
			"one\ntwo",
		},
		{
			"numbered list",
			"1. first\n2. second",
			"first\nsecond",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := StripForPreview(tt.in); got != tt.want {
				t.Fatalf("StripForPreview(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestStripForPreview_nested(t *testing.T) {
	t.Parallel()
	in := "**bold *and* bold**"
	want := "bold and bold"
	if got := StripForPreview(in); got != want {
		t.Fatalf("StripForPreview(%q) = %q, want %q", in, got, want)
	}
}

func TestStripForPreview_plainPassthrough(t *testing.T) {
	t.Parallel()
	in := "hello world"
	if got := StripForPreview(in); got != in {
		t.Fatalf("StripForPreview(%q) = %q, want unchanged", in, got)
	}
}

func TestStripForPreview_unsafeLinkKeptAsText(t *testing.T) {
	t.Parallel()
	in := "[click](javascript:alert(1))"
	want := "click"
	if got := StripForPreview(in); got != want {
		t.Fatalf("StripForPreview(%q) = %q, want %q", in, got, want)
	}
}
