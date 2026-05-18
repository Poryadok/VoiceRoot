package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	voicejwt "voice/backend/pkg/jwt"
)

type tokenValidator interface {
	Validate(r *http.Request) (voicejwt.Claims, string)
}

func newServiceHandler(service string, tv tokenValidator) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/health", healthOnly(service))
	mux.Handle("/ws", newWSHandler(tv))
	return mux
}

func newWSHandler(tv tokenValidator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tv == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "realtime_auth_unconfigured")
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		if !isWebSocketUpgrade(r) {
			writeJSONError(w, http.StatusBadRequest, "websocket_upgrade_required")
			return
		}
		claims, code := tv.Validate(r)
		if code != "" {
			writeJSONError(w, http.StatusUnauthorized, "invalid_token")
			return
		}
		if claims.ProfileID == "" {
			writeJSONError(w, http.StatusUnauthorized, "invalid_token")
			return
		}
		profileID, ok := pickActiveProfileID(r)
		if !ok || profileID == "" {
			writeJSONError(w, http.StatusUnauthorized, "invalid_token")
			return
		}
		if profileID != claims.ProfileID {
			writeJSONError(w, http.StatusUnauthorized, "invalid_token")
			return
		}

		up := websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(_ *http.Request) bool {
				// Origins are enforced at API Gateway / edge ingress.
				return true
			},
		}
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("ws upgrade: %v", err)
			return
		}
		go runWSConn(conn)
	})
}

func pickActiveProfileID(r *http.Request) (string, bool) {
	xp := strings.TrimSpace(r.Header.Get("X-Profile-Id"))
	xv := strings.TrimSpace(r.Header.Get("X-Voice-Profile-Id"))
	if xp != "" && xv != "" && xp != xv {
		return "", false
	}
	if xp != "" {
		return xp, true
	}
	if xv != "" {
		return xv, true
	}
	return "", false
}

func isWebSocketUpgrade(r *http.Request) bool {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return false
	}
	for _, value := range strings.Split(r.Header.Get("Connection"), ",") {
		if strings.EqualFold(strings.TrimSpace(value), "upgrade") {
			return true
		}
	}
	return false
}

func writeJSONError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

type wsOutbound struct {
	Op string          `json:"op"`
	D  json.RawMessage `json:"d,omitempty"`
	S  int64           `json:"s"`
}

type wsInbound struct {
	Op string          `json:"op"`
	D  json.RawMessage `json:"d"`
}

func runWSConn(c *websocket.Conn) {
	defer func() { _ = c.Close() }()

	var mu sync.Mutex
	var seq int64

	write := func(op string, d json.RawMessage) error {
		mu.Lock()
		defer mu.Unlock()
		seq++
		msg := wsOutbound{Op: op, D: d, S: seq}
		_ = c.SetWriteDeadline(time.Now().Add(10 * time.Second))
		return c.WriteJSON(msg)
	}

	helloD, _ := json.Marshal(map[string]any{})
	if err := write("hello", helloD); err != nil {
		return
	}

	for {
		_ = c.SetReadDeadline(time.Now().Add(90 * time.Second))
		var in wsInbound
		if err := c.ReadJSON(&in); err != nil {
			return
		}
		switch in.Op {
		case "heartbeat":
			ackD, _ := json.Marshal(map[string]any{})
			if err := write("heartbeat_ack", ackD); err != nil {
				return
			}
		case "resume":
			// last_s is for client-side reconnect bookkeeping; message catch-up is via Messaging REST.
			_ = in.D
		default:
			// ignore unknown ops for now
		}
	}
}
