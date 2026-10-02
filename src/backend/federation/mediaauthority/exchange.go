package mediaauthority

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"
)

type ExchangeRequest struct {
	Credential string `json:"credential"`
	RoomName   string `json:"room_name"`
	ProfileID  string `json:"profile_id"`
}

type ExchangeResult struct {
	JWT        string `json:"jwt"`
	LivekitURL string `json:"livekit_url"`
	ExpiresAt  int64  `json:"expires_at"`
}

// Exchange holds only node-local LiveKit credentials and pinned master public
// trust. A broad master access/refresh token is never accepted at this edge.
type Exchange struct {
	registry                 *Registry
	apiKey, secret, mediaURL string
	now                      func() time.Time
}

func NewExchange(registry *Registry, apiKey, secret, mediaURL string) (*Exchange, error) {
	u, err := url.Parse(mediaURL)
	if registry == nil || !safeText(apiKey, 128) || !safeText(secret, 4096) || err != nil || u.Scheme != "wss" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, ErrDenied
	}
	return &Exchange{registry: registry, apiKey: apiKey, secret: secret, mediaURL: mediaURL, now: time.Now}, nil
}

func (e *Exchange) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.Path != "/v1/media/token" || r.URL.RawQuery != "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if e == nil || r.TLS == nil || r.TLS.Version < 0x0303 {
		http.Error(w, "media denied", http.StatusForbidden)
		return
	}
	var input ExchangeRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxTokenBytes+1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	now := e.now()
	admission, err := e.registry.Admit(input.Credential, input.RoomName, input.ProfileID, now)
	if err != nil {
		http.Error(w, "media denied", http.StatusForbidden)
		return
	}
	grant := admission.grant
	// JWT NumericDate is integral seconds. Round DOWN so this wrapper cannot
	// outlive the signed master admission, and deny a sub-second remainder.
	expiry := time.UnixMilli(grant.ExpiresAt).Unix()
	if expiry <= now.Unix() {
		http.Error(w, "media denied", http.StatusForbidden)
		return
	}
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	claims, err := json.Marshal(map[string]any{
		"iss": e.apiKey, "sub": grant.ProfileID, "iat": now.Unix(), "nbf": now.Unix(), "exp": expiry,
		"video":    map[string]any{"roomJoin": true, "room": grant.RoomName, "canPublish": grant.CanPublish, "canSubscribe": true, "canPublishData": false},
		GrantClaim: input.Credential,
	})
	if err != nil || e.registry.Check(admission, e.now()) != nil {
		http.Error(w, "media denied", http.StatusForbidden)
		return
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, []byte(e.secret))
	_, _ = mac.Write([]byte(unsigned))
	result := ExchangeResult{JWT: unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), LivekitURL: e.mediaURL, ExpiresAt: expiry * 1000}
	_ = json.NewEncoder(w).Encode(result)
}
