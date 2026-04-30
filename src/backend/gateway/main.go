package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type gatewayConfig struct {
	versionConfigs     map[string]versionConfig
	forceUpdate        *forceUpdatePolicy
	tokenClaims        map[string]tokenClaims
	rateLimitedGroups  map[string]bool
	restUpstreams      map[string]http.Handler
	realtimeUpstream   http.Handler
	requestIDGenerator func() string
}

type versionConfig struct {
	MinSupportedVersion string
	LatestVersion       string
	UpdateURL           string
	ReleaseNotes        string
	ShorebirdPatch      int
}

type forceUpdatePolicy struct {
	Platform  string
	Version   string
	UpdateURL string
}

type tokenClaims struct {
	UserID           string
	ProfileID        string
	Roles            []string
	SubscriptionTier string
}

type gateway struct {
	config gatewayConfig
}

func handler() http.Handler {
	return newGateway(gatewayConfig{})
}

func newGateway(config gatewayConfig) http.Handler {
	if config.versionConfigs == nil {
		config.versionConfigs = map[string]versionConfig{}
	}
	if config.tokenClaims == nil {
		config.tokenClaims = map[string]tokenClaims{}
	}
	if config.rateLimitedGroups == nil {
		config.rateLimitedGroups = map[string]bool{}
	}
	if config.restUpstreams == nil {
		config.restUpstreams = map[string]http.Handler{}
	}
	if config.requestIDGenerator == nil {
		config.requestIDGenerator = generateRequestID
	}
	return &gateway{config: config}
}

func (g *gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-Id")
	if requestID == "" {
		requestID = g.config.requestIDGenerator()
	}
	w.Header().Set("X-Request-Id", requestID)
	r.Header.Set("X-Request-Id", requestID)

	switch {
	case r.URL.Path == "/health":
		g.handleHealth(w, r)
	case r.URL.Path == "/ws":
		g.handleWebSocket(w, r)
	case r.URL.Path == "/api/v1/version":
		g.handleVersion(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/v1/"):
		g.handleREST(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (g *gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

func (g *gateway) handleVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	platform := r.URL.Query().Get("platform")
	clientVersion := r.URL.Query().Get("version")
	cfg, ok := g.config.versionConfigs[platform]
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown_platform"})
		return
	}

	forceUpdate := compareSemver(clientVersion, cfg.MinSupportedVersion) < 0
	updateAvailable := compareSemver(clientVersion, cfg.LatestVersion) < 0
	writeJSON(w, http.StatusOK, map[string]any{
		"force_update":          forceUpdate,
		"update_available":      updateAvailable,
		"latest_version":        cfg.LatestVersion,
		"min_supported_version": cfg.MinSupportedVersion,
		"update_url":            cfg.UpdateURL,
		"release_notes":         cfg.ReleaseNotes,
		"shorebird_patch":       cfg.ShorebirdPatch,
	})
}

func (g *gateway) handleREST(w http.ResponseWriter, r *http.Request) {
	if g.isForceUpdateBlocked(r) {
		writeJSON(w, http.StatusUpgradeRequired, map[string]string{
			"error":      "client_outdated",
			"update_url": g.config.forceUpdate.UpdateURL,
		})
		return
	}

	namespace := restNamespace(r.URL.Path)
	if namespace == "" || namespace == "federation" {
		http.NotFound(w, r)
		return
	}

	if group := rateLimitGroup(r.Method, r.URL.Path); group != "" && g.config.rateLimitedGroups[group] {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
		return
	}

	var claims tokenClaims
	if !isPublicRESTRoute(r.Method, r.URL.Path) {
		var ok bool
		claims, ok = g.authenticate(r)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		if namespace == "analytics" && !hasRole(claims, "staff") {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		applyClaims(r, claims)
	}

	upstream, ok := g.config.restUpstreams[namespace]
	if !ok {
		http.NotFound(w, r)
		return
	}
	upstream.ServeHTTP(w, r)
}

func (g *gateway) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !isWebSocketUpgrade(r) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "websocket_upgrade_required"})
		return
	}
	claims, ok := g.authenticate(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	applyClaims(r, claims)
	if g.config.realtimeUpstream == nil {
		http.NotFound(w, r)
		return
	}
	g.config.realtimeUpstream.ServeHTTP(w, r)
}

func (g *gateway) isForceUpdateBlocked(r *http.Request) bool {
	if g.config.forceUpdate == nil {
		return false
	}
	return r.Header.Get("X-Voice-Client-Platform") == g.config.forceUpdate.Platform &&
		r.Header.Get("X-Voice-Client-Version") == g.config.forceUpdate.Version
}

func (g *gateway) authenticate(r *http.Request) (tokenClaims, bool) {
	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, prefix) {
		return tokenClaims{}, false
	}
	claims, ok := g.config.tokenClaims[strings.TrimPrefix(auth, prefix)]
	return claims, ok
}

func restNamespace(path string) string {
	rest := strings.TrimPrefix(path, "/api/v1/")
	if rest == "" {
		return ""
	}
	namespace, _, _ := strings.Cut(rest, "/")
	return namespace
}

func isPublicRESTRoute(method, path string) bool {
	if method == http.MethodPost && (path == "/api/v1/auth/login" || path == "/api/v1/auth/register") {
		return true
	}
	return method == http.MethodGet && path == "/api/v1/version"
}

func rateLimitGroup(method, path string) string {
	switch {
	case method == http.MethodPost && (path == "/api/v1/auth/login" || path == "/api/v1/auth/register"):
		return "Auth"
	case method == http.MethodPost && strings.HasPrefix(path, "/api/v1/auth/otp/"):
		return "OTP"
	case method == http.MethodPost && path == "/api/v1/messages/send":
		return "MessagesSend"
	case method == http.MethodPost && path == "/api/v1/files/upload":
		return "FileUpload"
	case method == http.MethodPost && path == "/api/v1/spaces":
		return "SpaceCreation"
	case method == http.MethodPost && strings.HasPrefix(path, "/api/v1/bots/"):
		return "BotAPI"
	default:
		return ""
	}
}

func applyClaims(r *http.Request, claims tokenClaims) {
	r.Header.Set("X-Voice-User-Id", claims.UserID)
	r.Header.Set("X-Voice-Profile-Id", claims.ProfileID)
	r.Header.Set("X-Voice-Roles", strings.Join(claims.Roles, ","))
	r.Header.Set("X-Voice-Subscription-Tier", claims.SubscriptionTier)
}

func hasRole(claims tokenClaims, role string) bool {
	for _, candidate := range claims.Roles {
		if candidate == role {
			return true
		}
	}
	return false
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

func compareSemver(left, right string) int {
	lv := parseSemver(left)
	rv := parseSemver(right)
	for i := 0; i < 3; i++ {
		switch {
		case lv[i] < rv[i]:
			return -1
		case lv[i] > rv[i]:
			return 1
		}
	}
	return 0
}

func parseSemver(version string) [3]int {
	var parsed [3]int
	parts := strings.Split(version, ".")
	for i := 0; i < len(parts) && i < 3; i++ {
		value, err := strconv.Atoi(parts[i])
		if err == nil {
			parsed[i] = value
		}
	}
	return parsed
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func generateRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "request-id-unavailable"
	}
	return hex.EncodeToString(b[:])
}

func main() {
	addr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, handler()))
}
