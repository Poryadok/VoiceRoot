// Ping-bot is a minimal polling-mode dev bot for Phase 16 local smoke.
//
// Usage:
//
//	export VOICE_API_BASE_URL=http://127.0.0.1:18080
//	export VOICE_BOT_TOKEN=vb_...
//	go run .
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	base := strings.TrimRight(env("VOICE_API_BASE_URL", "http://127.0.0.1:18080"), "/")
	token := strings.TrimSpace(os.Getenv("VOICE_BOT_TOKEN"))
	if token == "" {
		log.Fatal("VOICE_BOT_TOKEN is required")
	}
	ephemeral := strings.EqualFold(os.Getenv("VOICE_BOT_EPHEMERAL"), "true")
	client := &http.Client{Timeout: 10 * time.Second}
	auth := "Bot " + token
	log.Printf("ping-bot polling %s (ephemeral=%v)", base, ephemeral)

	for {
		req, err := http.NewRequest(http.MethodGet, base+"/api/v1/bots/me/interactions/poll", nil)
		if err != nil {
			log.Printf("poll request: %v", err)
			time.Sleep(250 * time.Millisecond)
			continue
		}
		req.Header.Set("Authorization", auth)
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("poll: %v", err)
			time.Sleep(250 * time.Millisecond)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			log.Printf("poll status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
			time.Sleep(250 * time.Millisecond)
			continue
		}
		var parsed struct {
			Events []struct {
				PayloadJSON string `json:"payload_json"`
			} `json:"events"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		for _, evt := range parsed.Events {
			var payload map[string]any
			_ = json.Unmarshal([]byte(evt.PayloadJSON), &payload)
			tok, _ := payload["interaction_token"].(string)
			if tok == "" {
				continue
			}
			cmd, _ := payload["command_name"].(string)
			content := "pong"
			if cmd != "" && cmd != "ping" {
				content = fmt.Sprintf("unknown command: %s", cmd)
			}
			complete, _ := json.Marshal(map[string]any{
				"interaction_token": tok,
				"content":           content,
				"is_ephemeral":      ephemeral,
			})
			creq, err := http.NewRequest(http.MethodPost, base+"/api/v1/bots/me/interactions/complete", bytes.NewReader(complete))
			if err != nil {
				continue
			}
			creq.Header.Set("Authorization", auth)
			creq.Header.Set("Content-Type", "application/json")
			cresp, err := client.Do(creq)
			if err != nil {
				log.Printf("complete: %v", err)
				continue
			}
			cbody, _ := io.ReadAll(cresp.Body)
			cresp.Body.Close()
			log.Printf("completed %s -> %d %s", tok, cresp.StatusCode, strings.TrimSpace(string(cbody)))
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
