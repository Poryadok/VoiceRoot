package grpcsvc

import "unicode/utf8"

const maxMessageTextCharacters = 4000

func messageTextWithinLimit(text string) bool {
	return utf8.RuneCountInString(text) <= maxMessageTextCharacters
}
