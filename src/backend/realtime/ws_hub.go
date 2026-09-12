package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// fanoutEnvelope is delivered to a WebSocket pump when another instance (or local peer) fans out.
type fanoutEnvelope struct {
	Op string
	D  json.RawMessage
}

// connReg is one authenticated WebSocket registered for cross-connection fan-out.
type connReg struct {
	instanceID string
	connID     string
	accountID  string
	profileID  string
	fanout     chan fanoutEnvelope
	chats      map[string]struct{}
	guardMu    sync.RWMutex
	writeGuard func() bool
}

type wsHub struct {
	mu                  sync.RWMutex
	byChat              map[string]map[*connReg]struct{}
	byProfile           map[string]map[*connReg]struct{}
	memberInboxLister   chatMemberInboxLister
	subscriptionChecker chatSubscriptionChecker
	dmPairByChat        map[string]dmAccountPair
	dmChatsByPair       map[dmAccountPair]*dmPairIndexEntry
}

type dmAccountPair struct {
	first  string
	second string
}

type dmPairIndexEntry struct {
	version  uint64
	inFlight int
	chatIDs  map[string]struct{}
}

func canonicalAccountPair(accountA, accountB string) (dmAccountPair, bool) {
	first, errA := uuid.Parse(strings.TrimSpace(accountA))
	second, errB := uuid.Parse(strings.TrimSpace(accountB))
	if errA != nil || errB != nil || first == second {
		return dmAccountPair{}, false
	}
	a, b := first.String(), second.String()
	if a > b {
		a, b = b, a
	}
	return dmAccountPair{first: a, second: b}, true
}

// canonicalChatID makes UUID-equivalent wire spellings use one subscription
// key. Invalid IDs remain unchanged; client ingress rejects those before it
// reaches the hub, while bootstrap/event callers can still be observed safely.
func canonicalChatID(chatID string) string {
	chatID = strings.TrimSpace(chatID)
	parsed, err := uuid.Parse(chatID)
	if err != nil {
		return chatID
	}
	return parsed.String()
}

func canonicalUUID(value string) string {
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return strings.TrimSpace(value)
	}
	return parsed.String()
}

// hasChat is the sole local authority for subscription-gated client actions.
func (h *wsHub) hasChat(reg *connReg, chatID string) bool {
	chatID = canonicalChatID(chatID)
	if h == nil || reg == nil || chatID == "" {
		return false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := reg.chats[chatID]
	return ok
}

func (h *wsHub) authorizeClientSideEffect(ctx context.Context, reg *connReg, chatID string) bool {
	if !h.hasChat(reg, chatID) {
		return false
	}
	checker, ok := h.subscriptionChecker.(chatSideEffectChecker)
	if !ok {
		return true
	}
	if err := checker.AuthorizeSideEffect(ctx, reg.accountID, chatID); err != nil {
		return false
	}
	// An event or an authoritative blocked read may have revoked the local
	// subscription while the Social request was in flight.
	return h.hasChat(reg, chatID)
}

func (h *wsHub) authorizeAndAddChat(ctx context.Context, reg *connReg, accountID, profileID, chatID string) error {
	checker := h.subscriptionChecker
	if checker == nil {
		return fmt.Errorf("chat subscription checker is not configured")
	}
	if registrar, ok := checker.(chatSubscriptionRegistrar); ok {
		return registrar.AuthorizeAndAddChat(ctx, accountID, profileID, chatID, reg)
	}
	if err := checker.AuthorizeChat(ctx, accountID, profileID, chatID); err != nil {
		return err
	}
	if !h.addChat(reg, chatID) {
		return fmt.Errorf("chat subscription denied")
	}
	return nil
}

func (h *wsHub) chatIDs(reg *connReg) []string {
	if h == nil || reg == nil {
		return nil
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]string, 0, len(reg.chats))
	for chatID := range reg.chats {
		out = append(out, chatID)
	}
	return out
}

// revokeProfileChat removes every local tab of profileID from chatID. Chat
// membership events are authoritative; a later client operation must not rely
// on an obsolete connection-local subscription.
func (h *wsHub) revokeProfileChat(profileID, chatID string) {
	chatID = canonicalChatID(chatID)
	if h == nil || profileID == "" || chatID == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for reg := range h.byProfile[profileID] {
		if members := h.byChat[chatID]; members != nil {
			delete(members, reg)
			if len(members) == 0 {
				delete(h.byChat, chatID)
				h.removeDMChatIndexLocked(chatID)
			}
		}
		delete(reg.chats, chatID)
	}
}

func (h *wsHub) dmAccountPairForLocalChat(accountID, chatID string) (dmAccountPair, bool) {
	if h == nil {
		return dmAccountPair{}, false
	}
	accountID = canonicalUUID(accountID)
	chatID = canonicalChatID(chatID)
	h.mu.RLock()
	defer h.mu.RUnlock()
	pair, ok := h.dmPairByChat[chatID]
	if !ok || (accountID != pair.first && accountID != pair.second) {
		return dmAccountPair{}, false
	}
	return pair, true
}

func (h *wsHub) beginDMPairCheck(pair dmAccountPair) uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	entry := h.dmChatsByPair[pair]
	if entry == nil {
		entry = &dmPairIndexEntry{chatIDs: make(map[string]struct{})}
		h.dmChatsByPair[pair] = entry
	}
	entry.inFlight++
	return entry.version
}

func (h *wsHub) finishDMPairCheck(pair dmAccountPair, version uint64, reg *connReg, chatID string, allow bool) bool {
	chatID = canonicalChatID(chatID)
	h.mu.Lock()
	defer h.mu.Unlock()
	entry := h.dmChatsByPair[pair]
	if entry == nil || entry.inFlight == 0 {
		return false
	}
	entry.inFlight--
	accepted := allow && entry.version == version
	if accepted && reg != nil {
		if existing, ok := h.dmPairByChat[chatID]; ok && existing != pair {
			accepted = false
		} else {
			h.addChatLocked(reg, chatID)
			h.dmPairByChat[chatID] = pair
			entry.chatIDs[chatID] = struct{}{}
		}
	}
	h.cleanupDMPairEntryLocked(pair)
	return accepted
}

func (h *wsHub) cleanupDMPairEntryLocked(pair dmAccountPair) {
	entry := h.dmChatsByPair[pair]
	if entry != nil && entry.inFlight == 0 && len(entry.chatIDs) == 0 {
		delete(h.dmChatsByPair, pair)
	}
}

func (h *wsHub) removeDMChatIndexLocked(chatID string) {
	pair, ok := h.dmPairByChat[chatID]
	if !ok {
		return
	}
	delete(h.dmPairByChat, chatID)
	if entry := h.dmChatsByPair[pair]; entry != nil {
		delete(entry.chatIDs, chatID)
	}
	h.cleanupDMPairEntryLocked(pair)
}

// revokeAccountPairDMChats marks the pair denied before removing every local
// tab on either account. Every Realtime instance applies the same Social event.
func (h *wsHub) revokeAccountPairDMChats(accountA, accountB string) {
	if h == nil {
		return
	}
	pair, ok := canonicalAccountPair(accountA, accountB)
	if !ok {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	entry := h.dmChatsByPair[pair]
	if entry == nil {
		return
	}
	entry.version++
	chatIDs := make([]string, 0, len(entry.chatIDs))
	for chatID := range entry.chatIDs {
		chatIDs = append(chatIDs, chatID)
	}
	for _, chatID := range chatIDs {
		members := h.byChat[chatID]
		for reg := range members {
			if reg.accountID != pair.first && reg.accountID != pair.second {
				continue
			}
			delete(members, reg)
			delete(reg.chats, chatID)
		}
		if len(members) == 0 {
			delete(h.byChat, chatID)
			h.removeDMChatIndexLocked(chatID)
		}
	}
	h.cleanupDMPairEntryLocked(pair)
}

func (r *connReg) setWriteGuard(guard func() bool) {
	r.guardMu.Lock()
	r.writeGuard = guard
	r.guardMu.Unlock()
}

func (r *connReg) enqueue(env fanoutEnvelope, blocking bool) {
	r.guardMu.RLock()
	guard := r.writeGuard
	r.guardMu.RUnlock()
	if guard != nil {
		if !guard() {
			return
		}
	}
	if blocking {
		r.fanout <- env
		return
	}
	select {
	case r.fanout <- env:
	default:
	}
}

func newWSHub() *wsHub {
	return &wsHub{
		byChat:        make(map[string]map[*connReg]struct{}),
		byProfile:     make(map[string]map[*connReg]struct{}),
		dmPairByChat:  make(map[string]dmAccountPair),
		dmChatsByPair: make(map[dmAccountPair]*dmPairIndexEntry),
	}
}

func (h *wsHub) attachConn(instanceID, connID, profileID string, fanoutBuf int) *connReg {
	return h.attachAccountConn(instanceID, connID, "", profileID, fanoutBuf)
}

func (h *wsHub) attachAccountConn(instanceID, connID, accountID, profileID string, fanoutBuf int) *connReg {
	reg := &connReg{
		instanceID: instanceID,
		connID:     connID,
		accountID:  canonicalUUID(accountID),
		profileID:  profileID,
		fanout:     make(chan fanoutEnvelope, fanoutBuf),
		chats:      make(map[string]struct{}),
	}
	if profileID == "" {
		return reg
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.byProfile[profileID] == nil {
		h.byProfile[profileID] = make(map[*connReg]struct{})
	}
	h.byProfile[profileID][reg] = struct{}{}
	return reg
}

func (h *wsHub) addChat(reg *connReg, chatID string) bool {
	chatID = canonicalChatID(chatID)
	if reg == nil || chatID == "" {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.addChatLocked(reg, chatID)
	return true
}

func (h *wsHub) addChatLocked(reg *connReg, chatID string) {
	if h.byChat[chatID] == nil {
		h.byChat[chatID] = make(map[*connReg]struct{})
	}
	h.byChat[chatID][reg] = struct{}{}
	reg.chats[chatID] = struct{}{}
}

func (h *wsHub) removeChat(reg *connReg, chatID string) {
	chatID = canonicalChatID(chatID)
	if reg == nil || chatID == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if m, ok := h.byChat[chatID]; ok {
		delete(m, reg)
		if len(m) == 0 {
			delete(h.byChat, chatID)
			h.removeDMChatIndexLocked(chatID)
		}
	}
	delete(reg.chats, chatID)
}

func (h *wsHub) unregisterConn(reg *connReg) bool {
	if reg == nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for chatID := range reg.chats {
		if m, ok := h.byChat[chatID]; ok {
			delete(m, reg)
			if len(m) == 0 {
				delete(h.byChat, chatID)
				h.removeDMChatIndexLocked(chatID)
			}
		}
	}
	reg.chats = make(map[string]struct{})
	if reg.profileID != "" {
		if m, ok := h.byProfile[reg.profileID]; ok {
			delete(m, reg)
			if len(m) == 0 {
				delete(h.byProfile, reg.profileID)
				return false
			}
			return true
		}
	}
	return false
}

// broadcastTypingExcept delivers op "typing" with payload d to every connection subscribed to chatID
// except the sender identified by (excludeInstance, excludeConn).
func (h *wsHub) broadcastTypingExcept(chatID, excludeInstance, excludeConn string, d json.RawMessage) {
	chatID = canonicalChatID(chatID)
	if chatID == "" {
		return
	}
	h.mu.RLock()
	m := h.byChat[chatID]
	// Copy pointers so we don't hold lock while sending on channels.
	var targets []*connReg
	for reg := range m {
		if reg.instanceID == excludeInstance && reg.connID == excludeConn {
			continue
		}
		targets = append(targets, reg)
	}
	h.mu.RUnlock()

	env := fanoutEnvelope{Op: "typing", D: d}
	for _, reg := range targets {
		reg.enqueue(env, false)
	}
}

// broadcastMarkReadSameProfileExcept delivers op "mark_read" to other connections of the same profile
// (cross-device read sync; see docs/ARCHITECTURE_REQUIREMENTS.md).
func (h *wsHub) broadcastMarkReadSameProfileExcept(profileID, excludeInstance, excludeConn string, d json.RawMessage) {
	if profileID == "" {
		return
	}
	h.mu.RLock()
	m := h.byProfile[profileID]
	var targets []*connReg
	for reg := range m {
		if reg.instanceID == excludeInstance && reg.connID == excludeConn {
			continue
		}
		targets = append(targets, reg)
	}
	h.mu.RUnlock()

	env := fanoutEnvelope{Op: "mark_read", D: d}
	for _, reg := range targets {
		reg.enqueue(env, false)
	}
}

// broadcastPresenceSameProfileExcept delivers op "presence_update" to other connections of the same profile.
func (h *wsHub) broadcastPresenceSameProfileExcept(profileID, excludeInstance, excludeConn string, d json.RawMessage) {
	if profileID == "" {
		return
	}
	h.mu.RLock()
	m := h.byProfile[profileID]
	var targets []*connReg
	for reg := range m {
		if reg.instanceID == excludeInstance && reg.connID == excludeConn {
			continue
		}
		targets = append(targets, reg)
	}
	h.mu.RUnlock()

	env := fanoutEnvelope{Op: "presence_update", D: d}
	for _, reg := range targets {
		reg.enqueue(env, false)
	}
}

// broadcastPresenceInChatExcept delivers op "presence_update" to connections subscribed to chatID,
// excluding the sender connection and excluding other tabs of the same profile (those get profile-scope sync).
func (h *wsHub) broadcastPresenceInChatExcept(chatID, senderProfileID, excludeInstance, excludeConn string, d json.RawMessage) {
	chatID = canonicalChatID(chatID)
	if chatID == "" {
		return
	}
	h.mu.RLock()
	m := h.byChat[chatID]
	var targets []*connReg
	for reg := range m {
		if reg.instanceID == excludeInstance && reg.connID == excludeConn {
			continue
		}
		if senderProfileID != "" && reg.profileID == senderProfileID {
			continue
		}
		targets = append(targets, reg)
	}
	h.mu.RUnlock()

	env := fanoutEnvelope{Op: "presence_update", D: d}
	for _, reg := range targets {
		reg.enqueue(env, false)
	}
}

const fanoutConnIDsLogCap = 8

func fanoutLogAttrs(chatID, profileID, op, requestID string, targets []*connReg) []slog.Attr {
	attrs := []slog.Attr{
		slog.String("event", "ws_fanout"),
		slog.String("op", op),
		slog.Int("recipient_count", len(targets)),
	}
	if chatID != "" {
		attrs = append(attrs, slog.String("chat_id", chatID))
	}
	if profileID != "" {
		attrs = append(attrs, slog.String("profile_id", profileID))
	}
	if requestID != "" {
		attrs = append(attrs, slog.String("request_id", requestID))
	}
	if len(targets) == 0 {
		return attrs
	}
	logged := make([]string, 0, fanoutConnIDsLogCap)
	for i, reg := range targets {
		if i >= fanoutConnIDsLogCap {
			attrs = append(attrs, slog.Int("conn_ids_truncated", len(targets)-fanoutConnIDsLogCap))
			break
		}
		logged = append(logged, reg.connID)
	}
	attrs = append(attrs, slog.Any("conn_ids", logged))
	return attrs
}

// profileIDsSubscribedToChat returns unique non-empty profile IDs with at least one connection subscribed to chatID.
func (h *wsHub) profileIDsSubscribedToChat(chatID string) []string {
	chatID = canonicalChatID(chatID)
	if chatID == "" {
		return nil
	}
	h.mu.RLock()
	m := h.byChat[chatID]
	seen := make(map[string]struct{}, len(m))
	var ids []string
	for reg := range m {
		if reg.profileID == "" {
			continue
		}
		if _, ok := seen[reg.profileID]; ok {
			continue
		}
		seen[reg.profileID] = struct{}{}
		ids = append(ids, reg.profileID)
	}
	h.mu.RUnlock()
	return ids
}

// broadcastToChat delivers a fan-out envelope to every connection subscribed to chatID (local hub only).
func (h *wsHub) broadcastToChat(chatID string, env fanoutEnvelope, logger *slog.Logger, requestID string) {
	chatID = canonicalChatID(chatID)
	if chatID == "" {
		return
	}
	h.mu.RLock()
	m := h.byChat[chatID]
	var targets []*connReg
	for reg := range m {
		targets = append(targets, reg)
	}
	h.mu.RUnlock()
	if logger != nil {
		logger.LogAttrs(context.Background(), slog.LevelDebug, "ws fanout", fanoutLogAttrs(chatID, "", env.Op, requestID, targets)...)
	}
	for _, reg := range targets {
		reg.enqueue(env, false)
	}
}

func profileFanoutBlocks(op string) bool {
	switch op {
	case "call_incoming", "call_accepted", "call_declined", "call_missed", "call_ended",
		"screen_share_started", "screen_share_stopped":
		return true
	default:
		return false
	}
}

// broadcastToProfile delivers a fan-out envelope to every connection for profileID (local hub only).
func (h *wsHub) broadcastToProfile(profileID string, env fanoutEnvelope, logger *slog.Logger, requestID string) {
	if profileID == "" {
		return
	}
	h.mu.RLock()
	m := h.byProfile[profileID]
	var targets []*connReg
	for reg := range m {
		targets = append(targets, reg)
	}
	h.mu.RUnlock()
	if logger != nil {
		logger.LogAttrs(context.Background(), slog.LevelDebug, "ws fanout", fanoutLogAttrs("", profileID, env.Op, requestID, targets)...)
	}
	for _, reg := range targets {
		reg.enqueue(env, profileFanoutBlocks(env.Op))
	}
}
