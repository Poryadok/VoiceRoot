package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"testing"
	"voice/backend/realtime/internal/lifecycle"
)

type fanoutAdmissionStore struct {
	pending int
	deny    bool
}

func (s *fanoutAdmissionStore) Begin(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error) {
	if s.deny {
		return uuid.Nil, lifecycle.ErrDenied
	}
	s.pending++
	return uuid.New(), nil
}
func (s *fanoutAdmissionStore) Settle(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	s.pending--
	return nil
}

func TestChatAndPersonalNotificationSinksUseOneAdmissionAndDenyLateSend(t *testing.T) {
	chat, profile := uuid.NewString(), uuid.NewString()
	db := &fanoutAdmissionStore{}
	hub := newWSHub()
	hub.lifecycle = lifecycle.NewExecutor(db, uuid.New())
	first := hub.attachConn("i", "first", profile, 8)
	second := hub.attachConn("i", "second", profile, 8)
	hub.addChat(first, chat)
	hub.addChat(second, chat)
	seen := 0
	guard := func() bool {
		seen++
		if db.pending != 1 {
			t.Errorf("sink not inside one durable admission: %d", db.pending)
		}
		return true
	}
	first.setWriteGuard(guard)
	second.setWriteGuard(guard)
	payload := json.RawMessage(`{"chat_id":"` + chat + `"}`)
	hub.broadcastToChat(chat, fanoutEnvelope{Op: "message_create", D: payload}, nil, "")
	hub.broadcastToProfile(profile, fanoutEnvelope{Op: "notification", D: json.RawMessage(`{"type":"mention","chat_id":"` + chat + `"}`)}, nil, "")
	if seen != 4 || db.pending != 0 {
		t.Fatalf("seen=%d pending=%d", seen, db.pending)
	}
	db.deny = true
	hub.broadcastToChat(chat, fanoutEnvelope{Op: "message_create", D: payload}, nil, "")
	hub.broadcastToProfile(profile, fanoutEnvelope{Op: "archive_activity", D: payload}, nil, "")
	hub.broadcastMarkReadSameProfileExcept(profile, "", "", payload)
	if seen != 4 {
		t.Fatalf("denied lifecycle reached sink: %d", seen)
	}
}

func TestMissingInvalidOriginDeniedAndNonChatControlPreserved(t *testing.T) {
	db := &fanoutAdmissionStore{}
	hub := newWSHub()
	hub.lifecycle = lifecycle.NewExecutor(db, uuid.New())
	profile := uuid.NewString()
	reg := hub.attachConn("i", "c", profile, 8)
	hub.broadcastToProfile(profile, fanoutEnvelope{Op: "archive_activity", D: json.RawMessage(`{}`)}, nil, "")
	hub.broadcastToProfile(profile, fanoutEnvelope{Op: "notification", D: json.RawMessage(`{"type":"new_message","chat_id":"invalid"}`)}, nil, "")
	if len(reg.fanout) != 0 {
		t.Fatal("missing/invalid chat origin enqueued")
	}
	hub.broadcastToProfile(profile, fanoutEnvelope{Op: "notification", D: json.RawMessage(`{"type":"friend_request"}`)}, nil, "")
	if len(reg.fanout) != 1 {
		t.Fatal("unrelated profile control was dropped")
	}
}

func TestChatLookupFailureCannotKeepSenderPreallow(t *testing.T) {
	hub := newWSHub()
	hub.subscriptionChecker = &failedMessagePolicy{}
	allowed := sharedChatRecipientAllowSet(hub, uuid.NewString(), "sender", []string{"sender"}, nil)
	if allowed["sender"] {
		t.Fatal("sender shortcut survived authoritative lookup failure")
	}
}

type failedMessagePolicy struct{}

func (*failedMessagePolicy) AuthorizeChat(context.Context, string, string, string) error {
	return errors.New("unavailable")
}
func (*failedMessagePolicy) MessageSenderAccount(context.Context, string) (string, error) {
	return "", errors.New("unavailable")
}
func (*failedMessagePolicy) MessageChatIsShared(context.Context, string, string, string) (bool, error) {
	return false, errors.New("unavailable")
}
func (*failedMessagePolicy) MessageRecipientAllowed(context.Context, string, string, string) (bool, error) {
	return false, errors.New("unavailable")
}
