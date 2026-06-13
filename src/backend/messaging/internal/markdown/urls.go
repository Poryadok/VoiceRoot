package markdown

import (
	"regexp"
	"strings"
)

// ExtractedURL is a link found in message content (markdown or autolink).
type ExtractedURL struct {
	URL   string
	Title string // markdown link label; empty for bare autolinks
}

var (
	reMarkdownLinkURL = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^)\s]+)\)`)
	reAutolinkURL     = regexp.MustCompile(`https?://[^\s<>\[\]()]+`)
)

// ExtractURLs returns URLs in document order (markdown links first per segment, then autolinks).
func ExtractURLs(content string) []ExtractedURL {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	var out []ExtractedURL
	seen := map[string]struct{}{}
	remaining := content
	for len(remaining) > 0 {
		md := reMarkdownLinkURL.FindStringSubmatchIndex(remaining)
		auto := reAutolinkURL.FindStringIndex(remaining)
		if md == nil && auto == nil {
			break
		}
		useMarkdown := md != nil && (auto == nil || md[0] <= auto[0])
		if useMarkdown {
			label := remaining[md[2]:md[3]]
			url := remaining[md[4]:md[5]]
			remaining = remaining[md[1]:]
			if _, ok := seen[url]; ok {
				continue
			}
			seen[url] = struct{}{}
			out = append(out, ExtractedURL{URL: url, Title: label})
			continue
		}
		url := remaining[auto[0]:auto[1]]
		remaining = remaining[auto[1]:]
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		out = append(out, ExtractedURL{URL: url})
	}
	return out
}
