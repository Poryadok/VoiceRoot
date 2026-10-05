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
	composeFcmCandidateLimit     = 16
	composeFcmWindow             = 5 * time.Second
	composeFcmKeyByteLimit       = 128
	composeFcmAdmissionCountLimit = 255
)

type composeFcmAdmissionKind uint8

const (
	composeFcmAdmissionInvalid composeFcmAdmissionKind = iota
	composeFcmAdmissionWindow
	composeFcmAdmissionTuple
	composeFcmAdmissionIdentity
	composeFcmAdmissionKindCount
)

type composeFcmControlState uint8

const (
	composeFcmControlUnread composeFcmControlState = iota
	composeFcmControlInvalid
	composeFcmControlValid
)

type composeFcmControl struct {
	MessageID       string `json:"message_id"`
	ChatID          string `json:"chat_id"`
	SenderProfileID string `json:"sender_profile_id"`
	RecipientID     string `json:"recipient_profile_id"`
}

type composeFcmObserver struct {
	path               string
	mu                 sync.Mutex
	candidates         map[string]*composeFcmTrace
	control            composeFcmControl
	controlState       composeFcmControlState
	preControlStarted  time.Time
	postControlStarted time.Time
	emitted            bool
	overflow           bool
	ambiguous          bool
	admissionCounts    [composeFcmAdmissionKindCount]uint8
	admissionSeen      bool
	admissionOverflow  bool
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
	provisional       bool
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
	if o == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if eventID == "" || messageID == "" || chatID == "" || senderID == "" ||
		len(eventID) > composeFcmKeyByteLimit || len(messageID) > composeFcmKeyByteLimit ||
		len(chatID) > composeFcmKeyByteLimit || len(senderID) > composeFcmKeyByteLimit {
		o.recordAdmissionLocked(composeFcmAdmissionIdentity)
		return nil
	}
	if o.controlState == composeFcmControlInvalid {
		o.recordAdmissionLocked(composeFcmAdmissionInvalid)
		return nil
	}
	key := messageID + "\x00" + eventID
	existing := o.candidates[key]
	if existing != nil && (existing.chatID != chatID || existing.senderID != senderID) {
		o.ambiguous = true
		o.recordAdmissionLocked(composeFcmAdmissionTuple)
		return nil
	}
	c := o.control
	if o.controlState == composeFcmControlValid && (c.ChatID != chatID || c.SenderProfileID != senderID || c.MessageID != "" && c.MessageID != messageID) {
		o.recordAdmissionLocked(composeFcmAdmissionTuple)
		return nil
	}
	now := time.Now()
	provisional := o.controlState == composeFcmControlUnread
	if provisional {
		if o.preControlStarted.IsZero() {
			o.preControlStarted = now
		}
		if now.Sub(o.preControlStarted) >= composeFcmWindow {
			o.recordAdmissionLocked(composeFcmAdmissionWindow)
			return nil
		}
	} else {
		if o.postControlStarted.IsZero() {
			o.postControlStarted = now
		}
		if now.Sub(o.postControlStarted) >= composeFcmWindow {
			o.recordAdmissionLocked(composeFcmAdmissionWindow)
			return nil
		}
	}
	if existing != nil {
		if existing.attempts < 9999 {
			existing.attempts++
		}
		return existing
	}
	if len(o.candidates) >= composeFcmCandidateLimit {
		o.overflow = true
		o.admissionSeen = true
		o.admissionOverflow = true
		return nil
	}
	target := ""
	if !provisional {
		target = c.RecipientID
	}
	trace := &composeFcmTrace{owner: o, messageID: messageID, eventID: eventID, chatID: chatID, senderID: senderID, target: target, attempts: 1, provisional: provisional,
		memberOK: "unknown", present: "unknown", inbox: "unknown", basePush: "unknown", finalPush: "unknown", presence: "unknown", policy: "unknown", route: "unknown"}
	o.candidates[key] = trace
	return trace
}

// recordAdmissionLocked stores only bounded counts of why observer admission was rejected.
// It is never an event or recipient outcome; callers hold o.mu.
func (o *composeFcmObserver) recordAdmissionLocked(kind composeFcmAdmissionKind) {
	if kind >= composeFcmAdmissionKindCount {
		o.admissionOverflow = true
		o.admissionSeen = true
		return
	}
	o.admissionSeen = true
	if o.admissionCounts[kind] == composeFcmAdmissionCountLimit {
		o.admissionOverflow = true
		return
	}
	o.admissionCounts[kind]++
}

func (o *composeFcmObserver) admissionClassLocked() string {
	if o.admissionOverflow || o.overflow {
		return "overflow"
	}
	if !o.admissionSeen {
		return "none"
	}
	classification := ""
	for kind, count := range o.admissionCounts {
		if count == 0 {
			continue
		}
		if classification != "" {
			return "mixed"
		}
		switch composeFcmAdmissionKind(kind) {
		case composeFcmAdmissionInvalid:
			classification = "invalid"
		case composeFcmAdmissionWindow:
			classification = "window"
		case composeFcmAdmissionTuple:
			classification = "tuple"
		case composeFcmAdmissionIdentity:
			classification = "identity"
		}
	}
	if classification == "" {
		return "overflow"
	}
	return classification
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
	if t == nil || t.provisional {
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
	if t == nil || t.provisional {
		return
	}
	t.owner.mu.Lock()
	defer t.owner.mu.Unlock()
	t.basePush = boolWord(decision.Push)
}

func (t *composeFcmTrace) baseFor(decisions map[string]delivery.DeliveryDecision) {
	if t == nil || t.provisional {
		return
	}
	decision, ok := decisions[t.target]
	if ok {
		t.base(decision)
	}
}

func (t *composeFcmTrace) finalFor(decisions map[string]delivery.DeliveryDecision) {
	if t == nil || t.provisional {
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
	if t == nil || t.provisional || profile.String() != t.recipientID() {
		return
	}
	t.owner.mu.Lock()
	t.basePush, t.finalPush = boolWord(basePush), boolWord(finalPush)
	t.presence, t.policy = safeWord(presence, "online", "offline", "unknown"), safeWord(policy, "ok", "error", "unknown")
	t.owner.mu.Unlock()
}

func (t *composeFcmTrace) FinalDecision(profile uuid.UUID, finalPush bool) {
	if t == nil || t.provisional || profile.String() != t.recipientID() {
		return
	}
	t.owner.mu.Lock()
	t.finalPush = boolWord(finalPush)
	t.owner.mu.Unlock()
}

func (t *composeFcmTrace) Tokens(profile uuid.UUID, rows, fcmEligible int, outcome string) {
	if t == nil || t.provisional || profile.String() != t.recipientID() {
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
	if t == nil || t.provisional || profile.String() != t.recipientID() || service != "fcm" {
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
		now := time.Now()
		o.mu.Lock()
		if o.expireLocked(now) {
			o.mu.Unlock()
			return
		}
		o.mu.Unlock()

		c, ok := o.readControl()
		sampledAt := time.Now()
		o.mu.Lock()
		continueCollecting := o.sampleControlLocked(c, ok, sampledAt)
		o.mu.Unlock()
		if !continueCollecting {
			return
		}
	}
}

// sampleControlLocked applies one collector read; callers hold o.mu.
func (o *composeFcmObserver) sampleControlLocked(c composeFcmControl, ok bool, sampledAt time.Time) bool {
	if !ok {
		o.control = composeFcmControl{}
		o.controlState = composeFcmControlInvalid
		// Any candidates observed across a sampled-invalid control cannot be
		// uniquely attributed. A later valid sample starts a fresh window.
		clear(o.candidates)
		o.preControlStarted = time.Time{}
		o.postControlStarted = time.Time{}
		o.overflow = false
		o.ambiguous = false
		return true
	}
	if o.controlState == composeFcmControlUnread && !o.preControlStarted.IsZero() && sampledAt.Sub(o.preControlStarted) >= composeFcmWindow {
		o.printUnknownLocked("expired")
		return false
	}
	previousState := o.controlState
	o.control, o.controlState = c, composeFcmControlValid
	if previousState != composeFcmControlValid {
		o.preControlStarted = time.Time{}
		if o.postControlStarted.IsZero() {
			o.postControlStarted = sampledAt
		}
	}
	return !o.expireLocked(sampledAt)
}

// expireLocked enforces independent bounded windows for initial-unread
// buffering and correlation after the first valid control sample.
func (o *composeFcmObserver) expireLocked(now time.Time) bool {
	if o.controlState == composeFcmControlUnread && !o.preControlStarted.IsZero() && now.Sub(o.preControlStarted) >= composeFcmWindow {
		o.printUnknownLocked("expired")
		return true
	}
	if o.controlState != composeFcmControlValid || o.postControlStarted.IsZero() || now.Sub(o.postControlStarted) < composeFcmWindow {
		return false
	}
	if len(o.candidates) == 0 {
		return true
	}
	if o.overflow {
		o.printUnknownLocked("overflow")
		return true
	}
	if o.ambiguous {
		o.printUnknownLocked("ambiguous")
		return true
	}
	if o.control.MessageID == "" {
		o.emitUnknownLocked()
		return true
	}
	found, ambiguous := o.candidateForMessage(o.control.MessageID)
	if ambiguous {
		o.printUnknownLocked("ambiguous")
	} else if found != nil && found.finished && !found.provisional {
		o.printTraceLocked(found)
	} else if found != nil && found.provisional {
		o.printUnknownLocked("incomplete")
	} else {
		o.emitUnknownLocked()
	}
	return true
}

// candidateForMessage runs with o.mu held and attributes only one distinct EventId.
// Repeated delivery of the same (message_id, EventId) pair shares one trace.
func (o *composeFcmObserver) candidateForMessage(messageID string) (*composeFcmTrace, bool) {
	if o.ambiguous {
		return nil, true
	}
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
	if found != nil && (found.chatID != o.control.ChatID || found.senderID != o.control.SenderProfileID) {
		return nil, false
	}
	return found, false
}

func (o *composeFcmObserver) emitUnknownLocked() { o.printUnknownLocked("unknown") }

func (o *composeFcmObserver) printUnknownLocked(reason string) {
	if o.emitted {
		return
	}
	o.emitted = true
	fmt.Printf("compose_fcm_diag valid=false reason=%s admission=%s candidates=%d attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown\n", safeWord(reason, "unknown", "ambiguous", "overflow", "expired", "incomplete"), o.admissionClassLocked(), boundedCount(len(o.candidates)))
	clear(o.candidates)
}

func (o *composeFcmObserver) printTraceLocked(t *composeFcmTrace) {
	if o.emitted {
		return
	}
	if t.provisional {
		o.printUnknownLocked("incomplete")
		return
	}
	if o.admissionClassLocked() != "none" {
		o.printUnknownLocked("unknown")
		return
	}
	o.emitted = true
	fmt.Printf("compose_fcm_diag valid=true reason=matched admission=%s candidates=1 attempts=%d member_result=%s member_count=%d recipient_present=%s inbox=%s base_push=%s final_push=%s presence=%s policy=%s token_rows=%d fcm_tokens=%d dispatcher_returns=%d route=%s\n",
		o.admissionClassLocked(), boundedCount(t.attempts), safeWord(t.memberOK, "ok", "error", "unknown"), boundedCount(t.memberRows), safeWord(t.present, "true", "false", "unknown"), safeWord(t.inbox, "main", "requests", "unknown"), safeWord(t.basePush, "true", "false", "unknown"), safeWord(t.finalPush, "true", "false", "unknown"), safeWord(t.presence, "online", "offline", "unknown"), safeWord(t.policy, "ok", "error", "unknown"), boundedCount(t.tokenRows), boundedCount(t.fcmTokens), boundedCount(t.dispatcherReturns), safeWord(t.route, "ack", "nak", "unknown"))
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
