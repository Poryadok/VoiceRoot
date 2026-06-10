package mentions

import (
	"encoding/json"
	"regexp"
	"strings"
)

var contentMentionToken = regexp.MustCompile(`(?i)@everyone|@here|@[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// EntriesJSONFromContent builds a mentions_json array from @tokens in message text.
func EntriesJSONFromContent(content string) string {
	matches := contentMentionToken.FindAllString(content, -1)
	if len(matches) == 0 {
		return "[]"
	}
	seen := make(map[string]struct{}, len(matches))
	entries := make([]Entry, 0, len(matches))
	for _, raw := range matches {
		token := strings.ToLower(strings.TrimSpace(raw))
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		switch token {
		case "@everyone":
			entries = append(entries, Entry{Type: "everyone"})
		case "@here":
			entries = append(entries, Entry{Type: "here"})
		default:
			entries = append(entries, Entry{Type: "user", TargetID: strings.TrimPrefix(token, "@")})
		}
	}
	out, err := json.Marshal(entries)
	if err != nil {
		return "[]"
	}
	return string(out)
}
