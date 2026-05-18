package main

import "context"

// dmChatLister loads DM chat IDs for the authenticated profile (Chat Service source of truth).
// Optional: when nil, Realtime skips bootstrap sync; clients may still use subscribe/unsubscribe.
type dmChatLister interface {
	ListDMChatIDs(ctx context.Context, accountID, profileID string) ([]string, error)
}
