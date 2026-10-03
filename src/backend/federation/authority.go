package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"voice/backend/federation/protocol"
)

var errForbidden = errors.New("forbidden")
var errConflict = errors.New("conflict")
var errInvalid = errors.New("invalid_request")

type q11DenialError struct {
	Reason string
	Cause  error
}

func (e q11DenialError) Error() string { return e.Cause.Error() }
func (e q11DenialError) Unwrap() error { return e.Cause }

var errQ11NodeCertificateMismatch error = q11DenialError{Reason: "node_certificate_mismatch", Cause: errForbidden}
var errQ11ScopeMismatch error = q11DenialError{Reason: "node_scope_mismatch", Cause: errForbidden}
var errQ11CredentialRevoked error = q11DenialError{Reason: "credential_revoked", Cause: errForbidden}
var errQ11ApprovalConflict error = q11DenialError{Reason: "approval_conflict", Cause: errConflict}

func q11DenialReason(err error) (string, bool) {
	var denial q11DenialError
	if !errors.As(err, &denial) {
		return "", false
	}
	return denial.Reason, true
}

type Permission = protocol.Permission
type Snapshot = protocol.Snapshot
type SnapshotManifest = protocol.SnapshotManifest
type SnapshotPage = protocol.SnapshotPage
type RevisionEvent = protocol.RevisionEvent
type Claims = protocol.Claims
type Envelope = protocol.Envelope

func canonicalID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id.String() == s && id != uuid.Nil
}
func signEnvelope(key ed25519.PrivateKey, kid string, c Claims) (Envelope, error) {
	envelope, err := protocol.SignEnvelope(key, kid, c)
	if err != nil {
		return Envelope{}, errInvalid
	}
	return envelope, nil
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
	c, err := protocol.VerifyEnvelope(key, e, now)
	if err != nil || c.Kind != expected.Kind || c.Issuer != expected.Issuer || c.Audience != expected.Audience || c.Environment != expected.Environment || c.NodeID != expected.NodeID || c.SpaceID != expected.SpaceID || c.Generation != expected.Generation || c.Epoch != expected.Epoch || c.Revision != expected.Revision || c.Hash != expected.Hash {
		return Claims{}, errForbidden
	}
	return c, nil
}
func digest(b []byte) string { return protocol.Digest(b) }
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
