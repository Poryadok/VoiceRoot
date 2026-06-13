package markdown

import "testing"

func TestExtractURLs_markdownAndAutolink(t *testing.T) {
	t.Parallel()
	urls := ExtractURLs("See [Voice](https://voice.app) and https://example.com/docs")
	if len(urls) != 2 {
		t.Fatalf("expected 2 urls, got %d", len(urls))
	}
	if urls[0].URL != "https://voice.app" || urls[0].Title != "Voice" {
		t.Fatalf("markdown link: %+v", urls[0])
	}
	if urls[1].URL != "https://example.com/docs" {
		t.Fatalf("autolink: %+v", urls[1])
	}
}

func TestExtractURLs_dedupes(t *testing.T) {
	t.Parallel()
	urls := ExtractURLs("https://a.com https://a.com")
	if len(urls) != 1 {
		t.Fatalf("expected 1 url, got %d", len(urls))
	}
}
