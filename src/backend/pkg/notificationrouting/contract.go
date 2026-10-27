// Package notificationrouting defines the live, event-bound Notification to
// Realtime routing boundary. It is not an inbox or a replay protocol.
package notificationrouting

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"io"
	"strings"
)

const Subject = "notification.routing.message.v1"
const Queue = "notification_message_routing_v1"
const ReplyPrefix = "_INBOX.voice.realtime.notification."
const MaxBytes = 1 << 20

type Target struct {
	ProfileID string `json:"profile_id"`
	Type      string `json:"type"`
}

type Decision struct {
	Version     int      `json:"version"`
	EventSHA256 string   `json:"event_sha256"`
	Allowed     []Target `json:"allowed"`
	Failed      bool     `json:"failed"`
}

func Hash(event []byte) string {
	h := sha256.Sum256(event)
	return hex.EncodeToString(h[:])
}

type Requester interface {
	RequestWithContext(context.Context, string, []byte) (*nats.Msg, error)
}

// Client uses a dedicated reply namespace, so Notification cannot publish into
// arbitrary Realtime request inboxes. It needs no dynamic response grant.
type Client struct{ Connection *nats.Conn }

func (c Client) RequestWithContext(ctx context.Context, subject string, event []byte) (*nats.Msg, error) {
	if c.Connection == nil {
		return nil, errors.New("routing connection unavailable")
	}
	reply := ReplyPrefix + uuid.NewString()
	sub, e := c.Connection.SubscribeSync(reply)
	if e != nil {
		return nil, e
	}
	defer func() { _ = sub.Unsubscribe() }()
	if e = sub.AutoUnsubscribe(1); e != nil {
		return nil, e
	}
	if e = c.Connection.PublishRequest(subject, reply, event); e != nil {
		return nil, e
	}
	return sub.NextMsgWithContext(ctx)
}

func ValidReply(reply string) bool {
	if !strings.HasPrefix(reply, ReplyPrefix) {
		return false
	}
	raw := strings.TrimPrefix(reply, ReplyPrefix)
	id, e := uuid.Parse(raw)
	return e == nil && id != uuid.Nil && id.String() == raw
}

// Request never falls back to subscriptions or partial membership on failure.
func Request(ctx context.Context, nc Requester, event []byte) ([]Target, error) {
	if nc == nil || len(event) == 0 || len(event) > MaxBytes {
		return nil, errors.New("invalid routing request")
	}
	msg, err := nc.RequestWithContext(ctx, Subject, event)
	if err != nil {
		return nil, err
	}
	if msg == nil || len(msg.Data) > MaxBytes {
		return nil, errors.New("invalid routing response")
	}
	decoder := json.NewDecoder(bytes.NewReader(msg.Data))
	decoder.DisallowUnknownFields()
	var decision Decision
	if err := decoder.Decode(&decision); err != nil {
		return nil, err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || decision.Version != 1 || decision.Failed || decision.EventSHA256 != Hash(event) {
		return nil, errors.New("unbound routing decision")
	}
	seen := map[string]bool{}
	for _, target := range decision.Allowed {
		id, err := uuid.Parse(target.ProfileID)
		if err != nil || id == uuid.Nil || id.String() != target.ProfileID || seen[target.ProfileID] {
			return nil, errors.New("invalid routing recipient")
		}
		seen[target.ProfileID] = true
		switch target.Type {
		case "new_message", "message_request", "mention", "reaction":
		default:
			return nil, errors.New("invalid routing type")
		}
	}
	return decision.Allowed, nil
}

// Encode binds even an empty decision to its exact source bytes.
func Encode(event []byte, targets []Target, failed bool) ([]byte, error) {
	return json.Marshal(Decision{Version: 1, EventSHA256: Hash(event), Allowed: targets, Failed: failed})
}
