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
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
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
func effectiveRequestID(values []string) string {
	if len(values) == 1 {
		value := values[0]
		id, err := uuid.Parse(value)
		if err == nil && id != uuid.Nil && id.String() == value {
			return value
		}
	}
	return uuid.NewString()
}

func writeAPIError(w http.ResponseWriter, status int, reason string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": reason})
}

func (a *authorityAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	requestID := effectiveRequestID(r.Header.Values("X-Request-ID"))
	w.Header().Set("X-Request-ID", requestID)
	r.Header.Set("X-Request-ID", requestID)
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	result, err := a.handle(w, r)
	if err != nil {
		code := http.StatusServiceUnavailable
		reason := "unavailable"
		switch {
		case errors.Is(err, errInvalid), errors.Is(err, errInvalidHostedResource):
			code = http.StatusBadRequest
			reason = "invalid_request"
		case errors.Is(err, errForbidden):
			code = http.StatusForbidden
			reason = "forbidden"
		case errors.Is(err, errConflict):
			code = http.StatusConflict
			reason = "conflict"
		}
		if pin, pinErr := peerFingerprint(r, time.Now()); pinErr == nil {
			if record, ok := a.q11AuditForRequest(r, pin, err, code); ok {
				if a.Store == nil || a.Store.appendQ11Audit(r.Context(), record) != nil {
					writeAPIError(w, http.StatusServiceUnavailable, "unavailable")
					return
				}
			}
		}
		writeAPIError(w, code, reason)
		return
	}
	if result == nil {
		result = map[string]string{"status": "ok"}
	}
	_ = json.NewEncoder(w).Encode(result)
}

func (a *authorityAPI) q11AuditForRequest(r *http.Request, fingerprint string, err error, status int) (q11AuditRecord, bool) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 || parts[0] != "v1" || parts[1] != "nodes" || !canonicalID(parts[2]) {
		return q11AuditRecord{}, false
	}
	operator := a.Operators[fingerprint]
	record := q11AuditRecord{
		ActorClass:       "node",
		ActorFingerprint: fingerprint,
		NodeID:           parts[2],
		RequestID:        r.Header.Get("X-Request-ID"),
		HTTPStatus:       status,
	}
	if operator {
		record.ActorClass = "operator"
	}
	if (len(parts) == 6 && (parts[5] == "snapshot" || parts[5] == "revisions") ||
		len(parts) == 8 && parts[5] == "snapshot" && parts[6] == "pages") &&
		parts[3] == "spaces" && canonicalID(parts[4]) && r.Method == http.MethodGet {
		record.SpaceID = parts[4]
		record.Action = "node.snapshot.read"
		if operator && errors.Is(err, errForbidden) {
			record.ReasonCode = "certificate_role_mismatch"
		} else if !operator {
			reason, ok := q11DenialReason(err)
			if !ok || !errors.Is(err, errForbidden) {
				return q11AuditRecord{}, false
			}
			record.ReasonCode = reason
		} else {
			return q11AuditRecord{}, false
		}
	} else if len(parts) == 4 && parts[3] == "approve" && r.Method == http.MethodPost {
		record.Action = "node.approve"
		if operator && errors.Is(err, errQ11ApprovalConflict) {
			record.ReasonCode = "approval_conflict"
		} else if !operator && errors.Is(err, errForbidden) {
			record.ReasonCode = "certificate_role_mismatch"
		} else {
			return q11AuditRecord{}, false
		}
	} else {
		return q11AuditRecord{}, false
	}
	if status == http.StatusConflict {
		record.Result = "conflict"
	} else {
		record.Result = "denied"
	}
	return record, true
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
	if len(parts) == 7 && parts[5] == "resources" && canonicalID(parts[6]) && r.Method == http.MethodPost {
		if !operator {
			return nil, errForbidden
		}
		var req struct {
			ResourceType      string   `json:"resource_type"`
			RoutingGeneration int64    `json:"routing_generation"`
			LifecycleState    string   `json:"lifecycle_state"`
			Capabilities      []string `json:"capabilities"`
			RoomName          string   `json:"room_name"`
		}
		if decodeRequest(w, r, &req) != nil {
			return nil, errInvalid
		}
		return a.Store.registerHostedResource(r.Context(), HostedResourceMapping{
			ResourceID: parts[6], ResourceType: req.ResourceType, SpaceID: space, HomeNodeID: node,
			RoutingGeneration: req.RoutingGeneration, LifecycleState: req.LifecycleState,
			Capabilities: req.Capabilities, RoomName: req.RoomName,
		})
	}
	if len(parts) == 6 && parts[5] == "snapshot" && r.Method == http.MethodPost {
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
	if len(parts) == 6 && parts[5] == "snapshot" && r.Method == http.MethodGet {
		return a.Store.issue(r.Context(), node, space, pin, secret, nil)
	}
	if len(parts) == 8 && parts[5] == "snapshot" && parts[6] == "pages" && r.Method == http.MethodGet {
		if len(r.URL.Query()) != 0 {
			return nil, errInvalid
		}
		page, parseErr := strconv.Atoi(parts[7])
		if parseErr != nil || page < 0 {
			return nil, errInvalid
		}
		return a.Store.issuePage(r.Context(), node, space, pin, secret, page)
	}
	if len(parts) == 6 && parts[5] == "revisions" && r.Method == http.MethodGet {
		query := r.URL.Query()
		values, exists := query["after_revision"]
		if !exists || len(values) != 1 || len(query) != 1 {
			return nil, errInvalid
		}
		after, parseErr := strconv.ParseInt(values[0], 10, 64)
		if parseErr != nil || after < 0 {
			return nil, errInvalid
		}
		return a.Store.issueRevisionStream(r.Context(), node, space, pin, secret, after)
	}
	if len(parts) == 6 && parts[5] == "lease" && r.Method == http.MethodPost {
		var ack leaseRequest
		if decodeRequest(w, r, &ack) != nil {
			return nil, errInvalid
		}
		return a.Store.issue(r.Context(), node, space, pin, secret, &ack)
	}
	return nil, errInvalid
}
func pinOrActor(r *http.Request) string { pin, _ := peerFingerprint(r, time.Now()); return pin }
