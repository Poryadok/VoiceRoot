package store

import "errors"

var (
	ErrNotChatMember      = errors.New("not a chat member")
	ErrChatNotFound       = errors.New("chat not found")
	ErrStoreNotConfigured = errors.New("store not configured")
)
