package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	voicejwt "voice/backend/pkg/jwt"
)

type tokenValidator interface {
	Validate(r *http.Request) (voicejwt.Claims, string)
}

type presenceUpdater interface {
	UpdatePresence(ctx context.Context, accountID, profileID, status, customStatus string) error
}

func newServiceHandler(service string, tv tokenValidator, lister dmChatLister, hub *wsHub, rf *redisFanout, instanceID string) http.Handler {
	return newServiceHandlerWithPresence(service, tv, lister, hub, rf, instanceID, nil)
}

func newServiceHandlerWithPresence(service string, tv tokenValidator, lister dmChatLister, hub *wsHub, rf *redisFanout, instanceID string, presence presenceUpdater) http.Handler {
	if hub == nil {
		hub = newWSHub()
	}
	if instanceID == "" {
		instanceID = uuid.NewString()
	}
	mux := http.NewServeMux()
	mux.Handle("/health", healthOnly(service))
	mux.Handle("/ws", newWSHandler(tv, lister, hub, rf, instanceID, presence))
	return mux
}

func newWSHandler(tv tokenValidator, lister dmChatLister, hub *wsHub, rf *redisFanout, instanceID string, presence presenceUpdater) http.Handler {
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
		go runWSConn(conn, claims, lister, hub, rf, instanceID, presence)
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

type subscribePayload struct {
	ChatID string `json:"chat_id"`
}

type markReadPayload struct {
	ChatID    string `json:"chat_id"`
	MessageID string `json:"message_id"`
}

type deliveryAckPayload struct {
	ChatID          string `json:"chat_id"`
	MessageID       string `json:"message_id"`
	SenderProfileID string `json:"sender_profile_id"`
}

type presenceUpdateClientPayload struct {
	Status       string `json:"status"`
	CustomStatus string `json:"custom_status"`
}

var (
	typingThrottle    = 3 * time.Second
	typingIdleTimeout = 5 * time.Second
)

type readResult struct {
	in  wsInbound
	err error
}

func runWSConn(c *websocket.Conn, claims voicejwt.Claims, lister dmChatLister, hub *wsHub, rf *redisFanout, instanceID string, presence presenceUpdater) {
	connID := uuid.NewString()
	reg := hub.attachConn(instanceID, connID, claims.ProfileID, 32)
	lastTypingStart := make(map[string]time.Time)
	typingTimers := make(map[string]*time.Timer)

	defer func() {
		for _, timer := range typingTimers {
			timer.Stop()
		}
		hasLocalProfileConn := hub.unregisterConn(reg)
		if rf != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = rf.Unregister(ctx, claims.ProfileID, connID)
			cancel()
		}
		if !hasLocalProfileConn {
			updatePresence(context.Background(), presence, claims, "invisible", "")
		}
		_ = c.Close()
	}()

	if rf != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := rf.Register(ctx, claims.ProfileID, connID); err != nil {
			log.Printf("ws redis register: %v", err)
		}
		cancel()
	}

	var mu sync.Mutex
	var seq int64
	chatSubs := make(map[string]struct{})

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
	updatePresence(context.Background(), presence, claims, "online", "")

	if lister != nil {
		lctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		ids, err := lister.ListDMChatIDs(lctx, claims.UserID, claims.ProfileID)
		cancel()
		degraded := err != nil
		if err != nil {
			log.Printf("ws dm chat list: %v", err)
			ids = nil
		}
		mu.Lock()
		for _, id := range ids {
			chatSubs[id] = struct{}{}
			hub.addChat(reg, id)
		}
		mu.Unlock()
		idsCopy := append([]string(nil), ids...)
		slices.Sort(idsCopy)
		syncD, _ := json.Marshal(map[string]any{
			"scope":    "dm",
			"chat_ids": idsCopy,
			"source":   "chat",
			"degraded": degraded,
		})
		if err := write("subscription_sync", syncD); err != nil {
			return
		}
	}

	readCh := make(chan readResult, 1)
	go func() {
		for {
			_ = c.SetReadDeadline(time.Now().Add(90 * time.Second))
			var in wsInbound
			err := c.ReadJSON(&in)
			select {
			case readCh <- readResult{in: in, err: err}:
			default:
				return
			}
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case env := <-reg.fanout:
			if err := write(env.Op, env.D); err != nil {
				return
			}
		case rr := <-readCh:
			if rr.err != nil {
				return
			}
			in := rr.in
			switch in.Op {
			case "heartbeat":
				updatePresence(context.Background(), presence, claims, "online", "")
				ackD, _ := json.Marshal(map[string]any{})
				if err := write("heartbeat_ack", ackD); err != nil {
					return
				}
			case "resume":
				// last_s is for client-side reconnect bookkeeping; message catch-up is via Messaging REST.
				_ = in.D
			case "subscribe":
				var p subscribePayload
				if err := json.Unmarshal(in.D, &p); err != nil || !validRFC4122ChatID(p.ChatID) {
					errD, _ := json.Marshal(map[string]any{
						"code":    "invalid_subscribe",
						"message": "chat_id must be a valid UUID",
					})
					if err := write("error", errD); err != nil {
						return
					}
					continue
				}
				cid := strings.TrimSpace(p.ChatID)
				mu.Lock()
				chatSubs[cid] = struct{}{}
				mu.Unlock()
				hub.addChat(reg, cid)
				ackD, _ := json.Marshal(map[string]any{"chat_id": cid})
				if err := write("subscribe_ack", ackD); err != nil {
					return
				}
			case "unsubscribe":
				var p subscribePayload
				if err := json.Unmarshal(in.D, &p); err != nil || !validRFC4122ChatID(p.ChatID) {
					errD, _ := json.Marshal(map[string]any{
						"code":    "invalid_unsubscribe",
						"message": "chat_id must be a valid UUID",
					})
					if err := write("error", errD); err != nil {
						return
					}
					continue
				}
				cid := strings.TrimSpace(p.ChatID)
				mu.Lock()
				delete(chatSubs, cid)
				mu.Unlock()
				hub.removeChat(reg, cid)
				ackD, _ := json.Marshal(map[string]any{"chat_id": cid})
				if err := write("unsubscribe_ack", ackD); err != nil {
					return
				}
			case "typing_start", "typing_stop":
				kind := "start"
				if in.Op == "typing_stop" {
					kind = "stop"
				}
				var p subscribePayload
				if err := json.Unmarshal(in.D, &p); err != nil || !validRFC4122ChatID(p.ChatID) {
					errD, _ := json.Marshal(map[string]any{
						"code":    "invalid_typing",
						"message": "chat_id must be a valid UUID",
					})
					if err := write("error", errD); err != nil {
						return
					}
					continue
				}
				cid := strings.TrimSpace(p.ChatID)
				mu.Lock()
				_, subscribed := chatSubs[cid]
				mu.Unlock()
				if !subscribed {
					errD, _ := json.Marshal(map[string]any{
						"code":    "invalid_typing",
						"message": "not subscribed to chat",
					})
					if err := write("error", errD); err != nil {
						return
					}
					continue
				}
				broadcastTyping := func(k string) {
					d, _ := json.Marshal(map[string]any{
						"chat_id":    cid,
						"profile_id": claims.ProfileID,
						"kind":       k,
					})
					hub.broadcastTypingExcept(cid, instanceID, connID, d)
					if rf != nil {
						ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
						if err := rf.PublishTyping(ctx, cid, claims.ProfileID, k, connID); err != nil {
							log.Printf("ws redis publish typing: %v", err)
						}
						cancel()
					}
				}
				mu.Lock()
				timer := typingTimers[cid]
				if timer != nil {
					timer.Stop()
					delete(typingTimers, cid)
				}
				mu.Unlock()
				if kind == "stop" {
					mu.Lock()
					delete(lastTypingStart, cid)
					mu.Unlock()
					broadcastTyping("stop")
					continue
				}
				now := time.Now()
				mu.Lock()
				last, ok := lastTypingStart[cid]
				shouldBroadcast := !ok || now.Sub(last) >= typingThrottle
				if shouldBroadcast {
					lastTypingStart[cid] = now
				}
				typingTimers[cid] = time.AfterFunc(typingIdleTimeout, func() {
					mu.Lock()
					delete(lastTypingStart, cid)
					delete(typingTimers, cid)
					mu.Unlock()
					broadcastTyping("stop")
				})
				mu.Unlock()
				if shouldBroadcast {
					broadcastTyping("start")
				}
			case "mark_read":
				var p markReadPayload
				if err := json.Unmarshal(in.D, &p); err != nil || !validRFC4122ChatID(p.ChatID) || !validRFC4122ChatID(p.MessageID) {
					errD, _ := json.Marshal(map[string]any{
						"code":    "invalid_mark_read",
						"message": "chat_id and message_id must be valid UUIDs",
					})
					if err := write("error", errD); err != nil {
						return
					}
					continue
				}
				cid := strings.TrimSpace(p.ChatID)
				mid := strings.TrimSpace(p.MessageID)
				mu.Lock()
				_, subscribed := chatSubs[cid]
				mu.Unlock()
				if !subscribed {
					errD, _ := json.Marshal(map[string]any{
						"code":    "invalid_mark_read",
						"message": "not subscribed to chat",
					})
					if err := write("error", errD); err != nil {
						return
					}
					continue
				}
				d, _ := json.Marshal(map[string]any{
					"chat_id":    cid,
					"message_id": mid,
					"profile_id": claims.ProfileID,
				})
				hub.broadcastMarkReadSameProfileExcept(claims.ProfileID, instanceID, connID, d)
				hub.broadcastToChat(cid, fanoutEnvelope{Op: "message_read", D: d})
				if rf != nil {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					if err := rf.PublishMarkRead(ctx, claims.ProfileID, cid, mid, connID); err != nil {
						log.Printf("ws redis publish mark_read: %v", err)
					}
					cancel()
				}
			case "delivery_ack":
				var p deliveryAckPayload
				if err := json.Unmarshal(in.D, &p); err != nil ||
					!validRFC4122ChatID(p.ChatID) ||
					!validRFC4122ChatID(p.MessageID) ||
					!validRFC4122ChatID(p.SenderProfileID) {
					errD, _ := json.Marshal(map[string]any{
						"code":    "invalid_delivery_ack",
						"message": "chat_id, message_id and sender_profile_id must be valid UUIDs",
					})
					if err := write("error", errD); err != nil {
						return
					}
					continue
				}
				cid := strings.TrimSpace(p.ChatID)
				mid := strings.TrimSpace(p.MessageID)
				senderID := strings.TrimSpace(p.SenderProfileID)
				mu.Lock()
				_, subscribed := chatSubs[cid]
				mu.Unlock()
				if !subscribed {
					errD, _ := json.Marshal(map[string]any{
						"code":    "invalid_delivery_ack",
						"message": "not subscribed to chat",
					})
					if err := write("error", errD); err != nil {
						return
					}
					continue
				}
				d, _ := json.Marshal(map[string]any{
					"chat_id":              cid,
					"message_id":           mid,
					"recipient_profile_id": claims.ProfileID,
				})
				hub.broadcastToProfile(senderID, fanoutEnvelope{Op: "message_delivered", D: d})
			case "presence_update":
				var p presenceUpdateClientPayload
				if err := json.Unmarshal(in.D, &p); err != nil {
					errD, _ := json.Marshal(map[string]any{
						"code":    "invalid_presence",
						"message": "invalid presence payload",
					})
					if err := write("error", errD); err != nil {
						return
					}
					continue
				}
				status := strings.TrimSpace(p.Status)
				if status == "" {
					errD, _ := json.Marshal(map[string]any{
						"code":    "invalid_presence",
						"message": "status is required",
					})
					if err := write("error", errD); err != nil {
						return
					}
					continue
				}
				custom := strings.TrimSpace(p.CustomStatus)
				if len(custom) > 256 {
					custom = custom[:256]
				}
				updatePresence(context.Background(), presence, claims, status, custom)
				dSelf, _ := json.Marshal(map[string]any{
					"profile_id":    claims.ProfileID,
					"status":        status,
					"custom_status": custom,
				})
				hub.broadcastPresenceSameProfileExcept(claims.ProfileID, instanceID, connID, dSelf)
				if rf != nil {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					if err := rf.PublishPresenceProfile(ctx, claims.ProfileID, status, custom, connID); err != nil {
						log.Printf("ws redis publish presence profile: %v", err)
					}
					cancel()
				}
				mu.Lock()
				chatCopy := make([]string, 0, len(chatSubs))
				for c := range chatSubs {
					chatCopy = append(chatCopy, c)
				}
				mu.Unlock()
				for _, c := range chatCopy {
					dChat, _ := json.Marshal(map[string]any{
						"chat_id":       c,
						"profile_id":    claims.ProfileID,
						"status":        status,
						"custom_status": custom,
					})
					hub.broadcastPresenceInChatExcept(c, claims.ProfileID, instanceID, connID, dChat)
					if rf != nil {
						ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
						if err := rf.PublishPresenceChat(ctx, c, claims.ProfileID, status, custom, connID); err != nil {
							log.Printf("ws redis publish presence chat: %v", err)
						}
						cancel()
					}
				}
			default:
				// ignore unknown ops for now
			}
		}
	}
}

func updatePresence(ctx context.Context, presence presenceUpdater, claims voicejwt.Claims, status, customStatus string) {
	if presence == nil {
		return
	}
	if strings.TrimSpace(claims.UserID) == "" || strings.TrimSpace(claims.ProfileID) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := presence.UpdatePresence(ctx, claims.UserID, claims.ProfileID, status, customStatus); err != nil {
		log.Printf("ws user presence update profile=%s status=%s: %v", claims.ProfileID, status, err)
	}
}

func validRFC4122ChatID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	_, err := uuid.Parse(id)
	return err == nil
}
