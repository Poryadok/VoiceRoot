package main

import (
	"encoding/json"
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
	fanout     chan fanoutEnvelope
	chats      map[string]struct{}
}

type wsHub struct {
	mu     sync.RWMutex
	byChat map[string]map[*connReg]struct{}
}

func newWSHub() *wsHub {
	return &wsHub{
		byChat: make(map[string]map[*connReg]struct{}),
	}
}

func (h *wsHub) attachConn(instanceID, connID string, fanoutBuf int) *connReg {
	return &connReg{
		instanceID: instanceID,
		connID:     connID,
		fanout:     make(chan fanoutEnvelope, fanoutBuf),
		chats:      make(map[string]struct{}),
	}
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

func (h *wsHub) unregisterConn(reg *connReg) {
	if reg == nil {
		return
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
