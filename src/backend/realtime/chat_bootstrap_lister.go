package main

import "context"

// chatBootstrapLister loads chat IDs for WS auto-subscribe (Chat Service source of truth).
// Optional: when nil, Realtime skips bootstrap sync; clients may still use subscribe/unsubscribe.
type chatBootstrapLister interface {
	ListChatIDs(ctx context.Context, accountID, profileID string) ([]string, error)
}
