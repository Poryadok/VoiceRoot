package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type authorityAPI struct {
	Store     *authorityStore
	Operators map[string]bool
}

func validPin(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == 32 && strings.ToLower(s) == s
}
func decodeRequest(w http.ResponseWriter, r *http.Request, v any) error {
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		return errInvalid
	}
	return strictJSON(b, v)
}
func (a *authorityAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	result, err := a.handle(w, r)
	if err != nil {
		code := http.StatusServiceUnavailable
		reason := "unavailable"
		switch {
		case errors.Is(err, errInvalid):
			code = http.StatusBadRequest
			reason = "invalid_request"
		case errors.Is(err, errForbidden):
			code = http.StatusForbidden
			reason = "forbidden"
		case errors.Is(err, errConflict):
			code = http.StatusConflict
			reason = "conflict"
		}
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": reason})
		return
	}
	if result == nil {
		result = map[string]string{"status": "ok"}
	}
	_ = json.NewEncoder(w).Encode(result)
}
func (a *authorityAPI) handle(w http.ResponseWriter, r *http.Request) (any, error) {
	pin, err := peerFingerprint(r, time.Now())
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "v1" || parts[1] != "nodes" {
		return nil, errInvalid
	}
	operator := a.Operators[pin]
	if len(parts) == 2 && r.Method == http.MethodPost {
		if !operator {
			return nil, errForbidden
		}
		var req struct {
			NodeID     string `json:"node_id"`
			OperatorID string `json:"operator_id"`
			Endpoint   string `json:"endpoint"`
			Pin        string `json:"certificate_sha256"`
		}
		if decodeRequest(w, r, &req) != nil || !canonicalID(req.NodeID) || !canonicalID(req.OperatorID) || !validPin(req.Pin) || a.Operators[req.Pin] {
			return nil, errInvalid
		}
		u, err := url.Parse(req.Endpoint)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || len(req.Endpoint) > 2048 {
			return nil, errInvalid
		}
		return nil, a.Store.enroll(r.Context(), req.NodeID, req.OperatorID, req.Endpoint, req.Pin)
	}
	if len(parts) < 4 || !canonicalID(parts[2]) {
		return nil, errInvalid
	}
	node := parts[2]
	if len(parts) == 4 && r.Method == http.MethodPost {
		if !operator {
			return nil, errForbidden
		}
		action := parts[3]
		var pin string
		switch action {
		case "approve":
			var req struct {
				Verified bool `json:"ownership_verified"`
			}
			if decodeRequest(w, r, &req) != nil || !req.Verified {
				return nil, errInvalid
			}
		case "rotate":
			var req struct {
				Pin string `json:"certificate_sha256"`
			}
			if decodeRequest(w, r, &req) != nil || !validPin(req.Pin) || a.Operators[req.Pin] {
				return nil, errInvalid
			}
			pin = req.Pin
		case "suspend", "defederate":
			if decodeRequest(w, r, &struct{}{}) != nil {
				return nil, errInvalid
			}
		default:
			return nil, errInvalid
		}
		result, err := a.Store.changeNode(r.Context(), node, action, pin, pinOrActor(r))
		if err != nil {
			return nil, err
		}
		if action == "approve" || action == "rotate" {
			return result, nil
		}
		return nil, nil
	}
	if len(parts) < 5 || parts[3] != "spaces" || !canonicalID(parts[4]) {
		return nil, errInvalid
	}
	space := parts[4]
	if len(parts) == 5 && r.Method == http.MethodPost {
		if !operator {
			return nil, errForbidden
		}
		if decodeRequest(w, r, &struct{}{}) != nil {
			return nil, errInvalid
		}
		return nil, a.Store.place(r.Context(), node, space)
	}
	if len(parts) != 6 {
		return nil, errInvalid
	}
	if parts[5] == "snapshot" && r.Method == http.MethodPost {
		if !operator {
			return nil, errForbidden
		}
		var snap Snapshot
		if decodeRequest(w, r, &snap) != nil {
			return nil, errInvalid
		}
		return nil, a.Store.publish(r.Context(), node, space, snap)
	}
	if operator {
		return nil, errForbidden
	}
	authorization := r.Header.Get("Authorization")
	if !strings.HasPrefix(authorization, "Bearer ") {
		return nil, errForbidden
	}
	secret := strings.TrimPrefix(authorization, "Bearer ")
	decodedSecret, err := base64.RawURLEncoding.DecodeString(secret)
	if err != nil || len(decodedSecret) != nodeCredentialEntropyBytes || base64.RawURLEncoding.EncodeToString(decodedSecret) != secret {
		return nil, errForbidden
	}
	if parts[5] == "snapshot" && r.Method == http.MethodGet {
		return a.Store.issue(r.Context(), node, space, pin, secret, nil)
	}
	if parts[5] == "lease" && r.Method == http.MethodPost {
		var ack leaseRequest
		if decodeRequest(w, r, &ack) != nil {
			return nil, errInvalid
		}
		return a.Store.issue(r.Context(), node, space, pin, secret, &ack)
	}
	return nil, errInvalid
}
func pinOrActor(r *http.Request) string { pin, _ := peerFingerprint(r, time.Now()); return pin }
