package testclient

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/google/uuid"
)

// SessionSnapshot is the player-scoped GIS response used during reconnect.
type SessionSnapshot struct {
	SessionID      uuid.UUID  `json:"session_id"`
	Kind           string     `json:"kind"`
	RosterRevision int64      `json:"roster_revision"`
	LeaseExpiresAt string     `json:"lease_expires_at"`
	ChatID         *uuid.UUID `json:"chat_id,omitempty"`
}

type SessionReader interface {
	GetPlayerSessionSnapshot(context.Context, uuid.UUID) (SessionSnapshot, error)
}

type MessagePage struct {
	Messages   []json.RawMessage `json:"messages"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

type ChatReader interface {
	ListMessages(context.Context, uuid.UUID, string) (MessagePage, error)
}

// ChatSender accepts caller-stable client IDs so a lost acknowledgement can be
// retried without creating a second message.
type ChatSender interface {
	SendMessage(context.Context, uuid.UUID, uuid.UUID, string) (json.RawMessage, error)
}

type Scope struct {
	ApplicationID  uuid.UUID
	EnvironmentID  uuid.UUID
	AccountID      uuid.UUID
	ProfileID      uuid.UUID
	SessionID      uuid.UUID
	ChatID         uuid.UUID
	RosterRevision int64
}

type CachedPage struct {
	Page   MessagePage
	Cursor string
}

// MessageCursor is scoped to a Messaging chat query. A Realtime `s`/resume
// value must not be converted into this type.
type MessageCursor string

type scopedKey struct {
	applicationID, environmentID, accountID, profileID, sessionID, chatID uuid.UUID
	rosterRevision                                                        int64
}

// PageCache is intentionally in-memory for this conformance client. Its key
// prevents data/cursors crossing apps, environments, accounts, profiles,
// sessions, chats, or roster revisions.
type PageCache struct {
	mu      sync.Mutex
	entries map[scopedKey]CachedPage
}

func NewPageCache() *PageCache { return &PageCache{entries: make(map[scopedKey]CachedPage)} }

func (c *PageCache) Get(scope Scope) (CachedPage, bool) {
	if c == nil {
		return CachedPage{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[keyFor(scope)]
	return entry, ok
}

func (c *PageCache) Put(scope Scope, entry CachedPage) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[keyFor(scope)] = entry
}

func (c *PageCache) ClearProfile(applicationID, environmentID, accountID, profileID uuid.UUID) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.entries {
		if key.applicationID == applicationID && key.environmentID == environmentID && key.accountID == accountID && key.profileID == profileID {
			delete(c.entries, key)
		}
	}
}

func (c *PageCache) ClearSessionExcept(scope Scope) {
	if c == nil {
		return
	}
	current := keyFor(scope)
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.entries {
		if key.applicationID == current.applicationID && key.environmentID == current.environmentID && key.accountID == current.accountID && key.profileID == current.profileID && key.sessionID == current.sessionID && key != current {
			delete(c.entries, key)
		}
	}
}

func keyFor(scope Scope) scopedKey {
	return scopedKey{scope.ApplicationID, scope.EnvironmentID, scope.AccountID, scope.ProfileID, scope.SessionID, scope.ChatID, scope.RosterRevision}
}

type PlayerSessionClient struct {
	Sessions SessionReader
	Chats    ChatReader
	Sender   ChatSender
	Cache    *PageCache
}

// Reconnect revalidates the player session before reading only the selected
// session chat. Realtime resume cursors are not accepted as message cursors.
func (c *PlayerSessionClient) Reconnect(ctx context.Context, scope Scope, messageCursor MessageCursor) (CachedPage, error) {
	if c == nil || c.Sessions == nil || c.Chats == nil || scope.AccountID == uuid.Nil || scope.ProfileID == uuid.Nil || scope.SessionID == uuid.Nil {
		return CachedPage{}, errors.New("player session client is unavailable")
	}
	snapshot, err := c.Sessions.GetPlayerSessionSnapshot(ctx, scope.SessionID)
	if err != nil {
		return CachedPage{}, err
	}
	if snapshot.SessionID != scope.SessionID || snapshot.RosterRevision <= 0 || snapshot.ChatID == nil || *snapshot.ChatID == uuid.Nil {
		return CachedPage{}, errors.New("player session snapshot is incomplete")
	}
	scope.ChatID = *snapshot.ChatID
	scope.RosterRevision = snapshot.RosterRevision
	if c.Cache != nil {
		c.Cache.ClearSessionExcept(scope)
	}
	page, err := c.Chats.ListMessages(ctx, scope.ChatID, string(messageCursor))
	if err != nil {
		return CachedPage{}, err
	}
	result := CachedPage{Page: page, Cursor: page.NextCursor}
	if c.Cache != nil {
		c.Cache.Put(scope, result)
	}
	return result, nil
}

// Send retries must call this with the same clientMessageID after a lost ACK.
func (c *PlayerSessionClient) Send(ctx context.Context, chatID, clientMessageID uuid.UUID, body string) (json.RawMessage, error) {
	if c == nil || c.Sender == nil || chatID == uuid.Nil || clientMessageID == uuid.Nil {
		return nil, errors.New("stable message identity is required")
	}
	return c.Sender.SendMessage(ctx, chatID, clientMessageID, body)
}
