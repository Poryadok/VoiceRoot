package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
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
	profileID  string
	fanout     chan fanoutEnvelope
	chats      map[string]struct{}
}

type wsHub struct {
	mu        sync.RWMutex
	byChat    map[string]map[*connReg]struct{}
	byProfile map[string]map[*connReg]struct{}
}

func newWSHub() *wsHub {
	return &wsHub{
		byChat:    make(map[string]map[*connReg]struct{}),
		byProfile: make(map[string]map[*connReg]struct{}),
	}
}

func (h *wsHub) attachConn(instanceID, connID, profileID string, fanoutBuf int) *connReg {
	reg := &connReg{
		instanceID: instanceID,
		connID:     connID,
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

func (h *wsHub) addChat(reg *connReg, chatID string) {
	if reg == nil || chatID == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.byChat[chatID] == nil {
		h.byChat[chatID] = make(map[*connReg]struct{})
	}
	h.byChat[chatID][reg] = struct{}{}
	reg.chats[chatID] = struct{}{}
}

func (h *wsHub) removeChat(reg *connReg, chatID string) {
	if reg == nil || chatID == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if m, ok := h.byChat[chatID]; ok {
		delete(m, reg)
		if len(m) == 0 {
			delete(h.byChat, chatID)
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
		select {
		case reg.fanout <- env:
		default:
			// Ephemeral typing: drop under backpressure.
		}
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
		select {
		case reg.fanout <- env:
		default:
		}
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
		select {
		case reg.fanout <- env:
		default:
		}
	}
}

// broadcastPresenceInChatExcept delivers op "presence_update" to connections subscribed to chatID,
// excluding the sender connection and excluding other tabs of the same profile (those get profile-scope sync).
func (h *wsHub) broadcastPresenceInChatExcept(chatID, senderProfileID, excludeInstance, excludeConn string, d json.RawMessage) {
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
		select {
		case reg.fanout <- env:
		default:
		}
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
		select {
		case reg.fanout <- env:
		default:
			// Drop under backpressure; client catches up via Messaging REST.
		}
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
		if profileFanoutBlocks(env.Op) {
			reg.fanout <- env
			continue
		}
		select {
		case reg.fanout <- env:
		default:
			// Ephemeral fan-out; clients reconcile through REST.
		}
	}
}
