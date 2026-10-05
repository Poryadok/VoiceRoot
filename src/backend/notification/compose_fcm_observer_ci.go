//go:build voice_compose_fcm_diagnostic

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"voice/backend/notification/internal/chatmembers"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
)

const (
	composeFcmCandidateLimit = 16
	composeFcmWindow         = 5 * time.Second
)

type composeFcmControl struct {
	MessageID       string `json:"message_id"`
	ChatID          string `json:"chat_id"`
	SenderProfileID string `json:"sender_profile_id"`
	RecipientID     string `json:"recipient_profile_id"`
}

type composeFcmObserver struct {
	path       string
	mu         sync.Mutex
	candidates map[string]*composeFcmTrace
	control    composeFcmControl
	controlOK  bool
	started    time.Time
	emitted    bool
	overflow   bool
}

type composeFcmTrace struct {
	owner             *composeFcmObserver
	messageID         string
	eventID           string
	chatID            string
	senderID          string
	target            string
	attempts          int
	memberRows        int
	memberOK          string
	present           string
	inbox             string
	basePush          string
	finalPush         string
	presence          string
	policy            string
	tokenRows         int
	fcmTokens         int
	dispatcherReturns int
	route             string
	finished          bool
}

func newComposeFcmObserver() *composeFcmObserver {
	path := os.Getenv("VOICE_FCM_DIAGNOSTIC_FILE")
	if path == "" {
		return nil
	}
	o := &composeFcmObserver{path: path, candidates: make(map[string]*composeFcmTrace)}
	go o.collect()
	return o
}

func (o *composeFcmObserver) begin(eventID, messageID, chatID, senderID string) *composeFcmTrace {
	if o == nil || eventID == "" || messageID == "" || chatID == "" || senderID == "" {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	c := o.control
	if !o.controlOK || c.ChatID != chatID || c.SenderProfileID != senderID {
		return nil
	}
	now := time.Now()
	if o.started.IsZero() {
		o.started = now
	}
	if now.Sub(o.started) > composeFcmWindow {
		return nil
	}
	key := messageID + "\x00" + eventID
	if existing := o.candidates[key]; existing != nil {
		existing.attempts++
		return existing
	}
	if len(o.candidates) >= composeFcmCandidateLimit {
		o.overflow = true
		return nil
	}
	trace := &composeFcmTrace{owner: o, messageID: messageID, eventID: eventID, chatID: chatID, senderID: senderID, target: c.RecipientID, attempts: 1,
		memberOK: "unknown", present: "unknown", inbox: "unknown", basePush: "unknown", finalPush: "unknown", presence: "unknown", policy: "unknown", route: "unknown"}
	o.candidates[key] = trace
	return trace
}

func (o *composeFcmObserver) finish(trace *composeFcmTrace, routeResult string) {
	if trace == nil || o == nil {
		return
	}
	o.mu.Lock()
	trace.route = routeResult
	trace.finished = true
	o.mu.Unlock()
}

func (t *composeFcmTrace) members(rows []chatmembers.Member, err error) {
	if t == nil {
		return
	}
	t.owner.mu.Lock()
	defer t.owner.mu.Unlock()
	if err != nil {
		t.memberOK = "error"
		return
	}
	t.memberOK = "ok"
	t.memberRows = len(rows)
	for _, row := range rows {
		if row.ProfileID == t.target {
			t.present = "true"
			if row.InboxBucket == "main" || row.InboxBucket == "requests" {
				t.inbox = row.InboxBucket
			}
			return
		}
	}
	t.present = "false"
}

func (t *composeFcmTrace) base(decision delivery.DeliveryDecision) {
	if t == nil {
		return
	}
	t.owner.mu.Lock()
	defer t.owner.mu.Unlock()
	t.basePush = boolWord(decision.Push)
}

func (t *composeFcmTrace) baseFor(decisions map[string]delivery.DeliveryDecision) {
	if t == nil {
		return
	}
	decision, ok := decisions[t.target]
	if ok {
		t.base(decision)
	}
}

func (t *composeFcmTrace) finalFor(decisions map[string]delivery.DeliveryDecision) {
	if t == nil {
		return
	}
	decision, ok := decisions[t.target]
	if !ok {
		return
	}
	t.owner.mu.Lock()
	t.finalPush = boolWord(decision.Push)
	t.owner.mu.Unlock()
}

func (t *composeFcmTrace) context(ctx context.Context) context.Context {
	if t == nil {
		return ctx
	}
	return dispatch.WithMessageDiagnosticObserver(ctx, t)
}

func (t *composeFcmTrace) Decision(profile uuid.UUID, basePush, finalPush bool, presence, policy string) {
	if t == nil || profile.String() != t.recipientID() {
		return
	}
	t.owner.mu.Lock()
	t.basePush, t.finalPush = boolWord(basePush), boolWord(finalPush)
	t.presence, t.policy = safeWord(presence, "online", "offline", "unknown"), safeWord(policy, "ok", "error", "unknown")
	t.owner.mu.Unlock()
}

func (t *composeFcmTrace) FinalDecision(profile uuid.UUID, finalPush bool) {
	if t == nil || profile.String() != t.recipientID() {
		return
	}
	t.owner.mu.Lock()
	t.finalPush = boolWord(finalPush)
	t.owner.mu.Unlock()
}

func (t *composeFcmTrace) Tokens(profile uuid.UUID, rows, fcmEligible int, outcome string) {
	if t == nil || profile.String() != t.recipientID() {
		return
	}
	t.owner.mu.Lock()
	t.tokenRows, t.fcmTokens = boundedCount(rows), boundedCount(fcmEligible)
	if outcome == "error" {
		t.policy = "error"
	}
	t.owner.mu.Unlock()
}

func (t *composeFcmTrace) DispatcherReturned(profile uuid.UUID, service string) {
	if t == nil || profile.String() != t.recipientID() || service != "fcm" {
		return
	}
	t.owner.mu.Lock()
	t.dispatcherReturns = boundedCount(t.dispatcherReturns + 1)
	t.owner.mu.Unlock()
}

// Recipient is set from the pre-send control record when the trace is created.
func (t *composeFcmTrace) recipientID() string { return t.target }

func (o *composeFcmObserver) readControl() (composeFcmControl, bool) {
	info, err := os.Lstat(o.path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() > 4096 {
		return composeFcmControl{}, false
	}
	data, err := os.ReadFile(o.path)
	if err != nil || len(data) > 4096 {
		return composeFcmControl{}, false
	}
	var c composeFcmControl
	if json.Unmarshal(data, &c) != nil || c.MessageID != "" && !validUUID(c.MessageID) || !validUUID(c.ChatID) || !validUUID(c.SenderProfileID) || !validUUID(c.RecipientID) {
		return composeFcmControl{}, false
	}
	return c, true
}

func (o *composeFcmObserver) collect() {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		c, ok := o.readControl()
		o.mu.Lock()
		o.control, o.controlOK = c, ok
		if !o.started.IsZero() && time.Since(o.started) > composeFcmWindow {
			if !ok || c.MessageID == "" {
				o.emitUnknownLocked()
				o.mu.Unlock()
				return
			}
			if o.overflow {
				o.printUnknownLocked("overflow")
				o.mu.Unlock()
				return
			}
			found, ambiguous := o.candidateForMessage(c.MessageID)
			if ambiguous {
				o.printUnknownLocked("ambiguous")
			} else if found != nil && found.finished {
				o.printTraceLocked(found)
			} else {
				o.emitUnknownLocked()
			}
			o.mu.Unlock()
			return
		}
		if !ok || c.MessageID == "" || o.emitted {
			o.mu.Unlock()
			continue
		}
		o.mu.Unlock()
	}
}

// candidateForMessage runs with o.mu held and attributes only one distinct EventId.
// Repeated delivery of the same (message_id, EventId) pair shares one trace.
func (o *composeFcmObserver) candidateForMessage(messageID string) (*composeFcmTrace, bool) {
	var found *composeFcmTrace
	for _, candidate := range o.candidates {
		if candidate.messageID != messageID {
			continue
		}
		if found != nil && found.eventID != candidate.eventID {
			return nil, true
		}
		found = candidate
	}
	return found, false
}

func (o *composeFcmObserver) emitUnknownLocked() { o.printUnknownLocked("unknown") }

func (o *composeFcmObserver) printUnknownLocked(reason string) {
	if o.emitted {
		return
	}
	o.emitted = true
	fmt.Printf("compose_fcm_diag valid=false reason=%s candidates=%d attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown\n", safeWord(reason, "unknown", "ambiguous", "overflow"), boundedCount(len(o.candidates)))
	clear(o.candidates)
}

func (o *composeFcmObserver) printTraceLocked(t *composeFcmTrace) {
	if o.emitted {
		return
	}
	o.emitted = true
	fmt.Printf("compose_fcm_diag valid=true reason=matched candidates=1 attempts=%d member_result=%s member_count=%d recipient_present=%s inbox=%s base_push=%s final_push=%s presence=%s policy=%s token_rows=%d fcm_tokens=%d dispatcher_returns=%d route=%s\n",
		boundedCount(t.attempts), safeWord(t.memberOK, "ok", "error", "unknown"), boundedCount(t.memberRows), safeWord(t.present, "true", "false", "unknown"), safeWord(t.inbox, "main", "requests", "unknown"), safeWord(t.basePush, "true", "false", "unknown"), safeWord(t.finalPush, "true", "false", "unknown"), safeWord(t.presence, "online", "offline", "unknown"), safeWord(t.policy, "ok", "error", "unknown"), boundedCount(t.tokenRows), boundedCount(t.fcmTokens), boundedCount(t.dispatcherReturns), safeWord(t.route, "ack", "nak", "unknown"))
	clear(o.candidates)
}

func validUUID(value string) bool { _, err := uuid.Parse(value); return err == nil }
func boolWord(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
func boundedCount(value int) int {
	if value < 0 {
		return 0
	}
	if value > 9999 {
		return 9999
	}
	return value
}
func safeWord(value string, allowed ...string) string {
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return "unknown"
}
