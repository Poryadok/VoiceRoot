package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

var errForbidden = errors.New("forbidden")
var errConflict = errors.New("conflict")
var errInvalid = errors.New("invalid_request")

type Permission struct {
	AccountID    string   `json:"account_id"`
	ProfileID    string   `json:"profile_id"`
	ResourceID   string   `json:"resource_id"`
	SessionEpoch int64    `json:"session_epoch"`
	Actions      []string `json:"actions"`
}
type Snapshot struct {
	Version     int          `json:"version"`
	Complete    bool         `json:"complete"`
	PageCount   int          `json:"page_count"`
	Revision    int64        `json:"revision"`
	ValidUntil  int64        `json:"valid_until"`
	Permissions []Permission `json:"permissions"`
}

func canonicalID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id.String() == s && id != uuid.Nil
}
func (s Snapshot) validate(now time.Time) error {
	if s.Version != 1 || !s.Complete || s.PageCount != 1 || s.Revision < 1 || s.ValidUntil <= now.UnixMilli() || s.ValidUntil > now.Add(5*time.Second).UnixMilli() || s.Permissions == nil {
		return errInvalid
	}
	for _, p := range s.Permissions {
		if !canonicalID(p.AccountID) || !canonicalID(p.ProfileID) || !canonicalID(p.ResourceID) || p.SessionEpoch < 1 || len(p.Actions) == 0 {
			return errInvalid
		}
		for _, a := range p.Actions {
			if a != "read" && a != "write" && a != "subscribe" && a != "media" {
				return errInvalid
			}
		}
	}
	return nil
}

type Claims struct {
	Version     int       `json:"version"`
	Kind        string    `json:"kind"`
	Issuer      string    `json:"issuer"`
	Audience    string    `json:"audience"`
	Environment string    `json:"environment"`
	NodeID      string    `json:"node_id"`
	SpaceID     string    `json:"space_id"`
	Generation  int64     `json:"generation"`
	Epoch       int64     `json:"epoch"`
	Revision    int64     `json:"revision"`
	IssuedAt    int64     `json:"issued_at"`
	ExpiresAt   int64     `json:"expires_at"`
	Hash        string    `json:"hash"`
	Snapshot    *Snapshot `json:"snapshot,omitempty"`
}
type Envelope struct {
	KeyID     string `json:"key_id"`
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

func signEnvelope(key ed25519.PrivateKey, kid string, c Claims) (Envelope, error) {
	if len(key) != ed25519.PrivateKeySize || kid == "" {
		return Envelope{}, errInvalid
	}
	b, err := json.Marshal(c)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{kid, base64.RawURLEncoding.EncodeToString(b), base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, b))}, nil
}
func strictJSON(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return errInvalid
	}
	if d.Decode(new(any)) != io.EOF {
		return errInvalid
	}
	return nil
}
func verifyEnvelope(key ed25519.PublicKey, e Envelope, expected Claims, now time.Time) (Claims, error) {
	var c Claims
	b, err := base64.RawURLEncoding.DecodeString(e.Payload)
	if err != nil {
		return c, errForbidden
	}
	sig, err := base64.RawURLEncoding.DecodeString(e.Signature)
	if err != nil || len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, b, sig) || strictJSON(b, &c) != nil {
		return c, errForbidden
	}
	if c.Version != 1 || c.Kind != expected.Kind || c.Issuer != expected.Issuer || c.Audience != expected.Audience || c.Environment != expected.Environment || c.NodeID != expected.NodeID || c.SpaceID != expected.SpaceID || c.Generation != expected.Generation || c.Epoch != expected.Epoch || c.Revision != expected.Revision || c.Hash != expected.Hash || c.IssuedAt > now.UnixMilli() || c.ExpiresAt <= now.UnixMilli() || c.ExpiresAt <= c.IssuedAt {
		return c, errForbidden
	}
	if c.Kind == "lease" && (c.ExpiresAt-c.IssuedAt > 2000 || c.Snapshot != nil) {
		return c, errForbidden
	}
	if c.Kind != "lease" && c.Kind != "snapshot" {
		return c, errForbidden
	}
	if c.Kind == "snapshot" {
		if c.Snapshot == nil || c.Snapshot.validate(now) != nil || c.Snapshot.Revision != c.Revision || c.Snapshot.ValidUntil != c.ExpiresAt {
			return c, errForbidden
		}
		raw, _ := json.Marshal(c.Snapshot)
		if digest(raw) != c.Hash {
			return c, errForbidden
		}
	}
	return c, nil
}
func digest(b []byte) string { v := sha256.Sum256(b); return hex.EncodeToString(v[:]) }
func peerFingerprint(r *http.Request, now time.Time) (string, error) {
	if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || len(r.TLS.PeerCertificates) == 0 {
		return "", errForbidden
	}
	cert := r.TLS.PeerCertificates[0]
	if !currentVerifiedChain(r.TLS.VerifiedChains, cert, now) {
		return "", errForbidden
	}
	return digest(cert.Raw), nil
}

// TLS verifies the chain when a connection is established. HTTP keep-alive can
// outlive a CA or intermediate certificate, so recheck every certificate in a
// handshake-verified path each time the request is authorized.
func currentVerifiedChain(chains [][]*x509.Certificate, leaf *x509.Certificate, now time.Time) bool {
	for _, chain := range chains {
		if len(chain) == 0 || !bytes.Equal(chain[0].Raw, leaf.Raw) {
			continue
		}
		valid := true
		for i, cert := range chain {
			if cert == nil || now.Before(cert.NotBefore) || !now.Before(cert.NotAfter) {
				valid = false
				break
			}
			if i+1 < len(chain) && cert.CheckSignatureFrom(chain[i+1]) != nil {
				valid = false
				break
			}
		}
		if valid {
			return true
		}
	}
	return false
}
