package store

import "errors"

var (
	ErrNotChatMember      = errors.New("not a chat member")
	ErrStoreNotConfigured = errors.New("store not configured")
)
