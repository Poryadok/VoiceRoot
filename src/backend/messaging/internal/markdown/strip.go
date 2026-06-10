package markdown

import (
	"regexp"
	"strings"
)

var (
	reFencedCode   = regexp.MustCompile("(?s)```[\\w-]*\\n?([\\s\\S]*?)```")
	reInlineCode   = regexp.MustCompile("`([^`]+)`")
	reMarkdownLink = regexp.MustCompile(`\[([^\]]+)\]\((?:[^()]|\([^()]*\))*\)`)
	reSpoiler      = regexp.MustCompile(`\|\|([^|]+)\|\|`)
	reBold         = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reItalic       = regexp.MustCompile(`\*([^*]+)\*`)
	reUnderline    = regexp.MustCompile(`__([^_]+)__`)
	reStrike       = regexp.MustCompile(`~~([^~]+)~~`)
	reHeader       = regexp.MustCompile(`(?m)^#{1,3}\s+(.+)$`)
	reBlockquote   = regexp.MustCompile(`(?m)^>\s?(.+)$`)
	reBullet       = regexp.MustCompile(`(?m)^-\s+(.+)$`)
	reNumbered     = regexp.MustCompile(`(?m)^\d+\.\s+(.+)$`)
)

// StripForPreview removes markdown markers for chat list previews and search snippets.
func StripForPreview(s string) string {
	if s == "" {
		return s
	}
	out := s
	out = reFencedCode.ReplaceAllString(out, "$1")
	out = reInlineCode.ReplaceAllString(out, "$1")
	out = reMarkdownLink.ReplaceAllString(out, "$1")
	out = reSpoiler.ReplaceAllString(out, "$1")
	for i := 0; i < 3; i++ {
		next := reBold.ReplaceAllString(out, "$1")
		next = reItalic.ReplaceAllString(next, "$1")
		next = reUnderline.ReplaceAllString(next, "$1")
		next = reStrike.ReplaceAllString(next, "$1")
		if next == out {
			break
		}
		out = next
	}
	out = reHeader.ReplaceAllString(out, "$1")
	out = reBlockquote.ReplaceAllString(out, "$1")
	out = reBullet.ReplaceAllString(out, "$1")
	out = reNumbered.ReplaceAllString(out, "$1")
	out = strings.TrimSpace(out)
	return out
}
