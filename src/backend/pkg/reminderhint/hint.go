// Package reminderhint defines Notification-authored ephemeral grace hints.
// Broker acceptance is never a durable in-app insertion or client receipt.
package reminderhint

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/google/uuid"
)

const Subject = "notification.subscription_grace_reminder"
const Type = "subscription_grace_reminder"
const Stream = "notification_reminder_hints"

var ErrInvalid = errors.New("invalid subscription reminder hint")
var ErrExpired = errors.New("expired subscription reminder hint")

type Hint struct {
	Version         int    `json:"schema_version"`
	NotificationID  string `json:"notification_id"`
	TargetProfileID string `json:"target_profile_id"`
	AggregateKind   string `json:"aggregate_kind"`
	AggregateID     string `json:"aggregate_id"`
	GraceRevision   string `json:"grace_revision"`
	Day             int    `json:"day"`
	Plan            string `json:"plan"`
	WindowEnd       string `json:"window_end"`
	SpaceID         string `json:"space_id,omitempty"`
}

func canonicalUUID(s string) bool {
	id, e := uuid.Parse(s)
	return e == nil && id != uuid.Nil && id.String() == s
}

// LogicalID matches the BE-118 Notification insertion identity, not a new event ID.
func LogicalID(kind, aggregate, revision string, day int) string {
	k := 1
	if kind == "SPACE" {
		k = 2
	}
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("voice/reminder/%d/%s/%s/%d", k, aggregate, revision, day))).String()
}

// Decode rejects unknown/duplicate fields and trailing values, even when a
// permissive JSON decoder would silently replace the recipient binding.
func Decode(raw []byte, now time.Time) (Hint, error) {
	var h Hint
	if len(raw) > 4096 {
		return h, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return h, ErrInvalid
	}
	seen := map[string]bool{}
	for d.More() {
		token, err = d.Token()
		key, ok := token.(string)
		if err != nil || !ok || seen[key] {
			return h, ErrInvalid
		}
		seen[key] = true
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return h, ErrInvalid
		}
	}
	if _, err = d.Token(); err != nil {
		return h, ErrInvalid
	}
	var tail any
	if d.Decode(&tail) != io.EOF {
		return h, ErrInvalid
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&h) != nil {
		return h, ErrInvalid
	}
	if h.AggregateKind == "PERSONAL" && seen["space_id"] {
		return Hint{}, ErrInvalid
	}
	if err = h.Validate(now); err != nil {
		return Hint{}, err
	}
	return h, nil
}

func (h Hint) Validate(now time.Time) error {
	revision, err := strconv.ParseInt(h.GraceRevision, 10, 64)
	if h.Version != 1 || err != nil || revision < 1 || strconv.FormatInt(revision, 10) != h.GraceRevision ||
		!canonicalUUID(h.TargetProfileID) || !canonicalUUID(h.AggregateID) || !canonicalUUID(h.NotificationID) ||
		(h.Day != 1 && h.Day != 3 && h.Day != 7) {
		return ErrInvalid
	}
	switch h.AggregateKind {
	case "PERSONAL":
		if h.Plan != "premium" || h.SpaceID != "" {
			return ErrInvalid
		}
	case "SPACE":
		if h.Plan != "space_pro" || h.SpaceID != h.AggregateID {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	if h.NotificationID != LogicalID(h.AggregateKind, h.AggregateID, h.GraceRevision, h.Day) {
		return ErrInvalid
	}
	end, err := time.Parse(time.RFC3339Nano, h.WindowEnd)
	if err != nil || end.Location() != time.UTC || end.Format(time.RFC3339Nano) != h.WindowEnd {
		return ErrInvalid
	}
	if !now.Before(end) {
		return ErrExpired
	}
	return nil
}

// ClientData is an explicit allowlist. PERSONAL aggregate_id is a raw account
// ID and must not escape through a whole-envelope marshal or error/log message.
func (h Hint) ClientData() ([]byte, error) {
	return json.Marshal(struct {
		Version         int    `json:"schema_version"`
		Type            string `json:"type"`
		NotificationID  string `json:"notification_id"`
		TargetProfileID string `json:"target_profile_id"`
		Plan            string `json:"plan"`
		Day             int    `json:"day"`
		WindowEnd       string `json:"window_end"`
		SpaceID         string `json:"space_id,omitempty"`
	}{h.Version, Type, h.NotificationID, h.TargetProfileID, h.Plan, h.Day, h.WindowEnd, h.SpaceID})
}
