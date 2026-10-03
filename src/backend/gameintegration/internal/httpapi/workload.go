package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidWorkloadProof = errors.New("invalid workload proof")
	ErrWorkloadUnavailable  = errors.New("workload proof unavailable")
)

type NonceStore interface {
	Use(context.Context, string, time.Duration) (bool, error)
}

type WorkloadVerifier struct {
	Key              []byte
	PreviousKey      []byte
	PreviousKeyUntil time.Time
	Principal        string
	Now              func() time.Time
	Nonces           NonceStore
}

// workloadProofFailure exposes only a bounded verification stage for internal
// diagnostics. It never carries request data, signatures, or credential bytes.
type workloadProofFailure string

const (
	workloadFailureHeader    workloadProofFailure = "header"
	workloadFailureBody      workloadProofFailure = "body"
	workloadFailureTimestamp workloadProofFailure = "timestamp"
	workloadFailureNonce     workloadProofFailure = "nonce"
	workloadFailureSignature workloadProofFailure = "signature"
	workloadFailureReplay    workloadProofFailure = "replay"
)

func (e workloadProofFailure) Error() string { return ErrInvalidWorkloadProof.Error() }
func (e workloadProofFailure) Unwrap() error { return ErrInvalidWorkloadProof }

func workloadProofFailureStage(err error) string {
	var failure workloadProofFailure
	if errors.As(err, &failure) {
		return string(failure)
	}
	if errors.Is(err, ErrWorkloadUnavailable) {
		return "unavailable"
	}
	return "invalid"
}

func workloadMessage(method, path, timestamp, nonce string) string {
	empty := sha256.Sum256(nil)
	return "v1\n" + method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + hex.EncodeToString(empty[:])
}

func workloadSignature(key []byte, method, path, timestamp, nonce string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(workloadMessage(method, path, timestamp, nonce)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func bodyWorkloadMessage(method, path, timestamp, nonce string, body []byte) string {
	digest := sha256.Sum256(body)
	return "v1\n" + method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + hex.EncodeToString(digest[:])
}

func bodyWorkloadSignature(key []byte, method, path, timestamp, nonce string, body []byte) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(bodyWorkloadMessage(method, path, timestamp, nonce, body)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// SignBodyWorkloadRequest is the test/controlled-client counterpart for
// authenticated internal POSTs that have a request body but no device assertion.
func SignBodyWorkloadRequest(r *http.Request, key []byte, now time.Time, nonce string, body []byte) {
	SignBodyWorkloadRequestForPrincipal(r, key, now, nonce, body, "auth")
}

func SignBodyWorkloadRequestForPrincipal(r *http.Request, key []byte, now time.Time, nonce string, body []byte, principal string) {
	timestamp := strconv.FormatInt(now.Unix(), 10)
	r.Header.Set("X-Voice-Workload", principal)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Voice-Timestamp", timestamp)
	r.Header.Set("X-Voice-Nonce", nonce)
	r.Header.Set("X-Voice-Signature", bodyWorkloadSignature(key, r.Method, r.URL.EscapedPath(), timestamp, nonce, body))
}

func assertionBoundWorkloadMessage(method, path, timestamp, nonce string, body, assertion []byte) string {
	bodyDigest := sha256.Sum256(body)
	assertionDigest := sha256.Sum256(assertion)
	return "v2\n" + method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" +
		hex.EncodeToString(bodyDigest[:]) + "\n" + hex.EncodeToString(assertionDigest[:])
}

func assertionBoundWorkloadSignature(key []byte, method, path, timestamp, nonce string, body, assertion []byte) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(assertionBoundWorkloadMessage(method, path, timestamp, nonce, body, assertion)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// SignAssertionBoundWorkloadRequest is the test/controlled-client counterpart
// for Auth→GIS authority calls. Version 2 additionally binds the exact raw Auth
// assertion header while leaving the established v1 Bot proof unchanged.
func SignAssertionBoundWorkloadRequest(r *http.Request, key []byte, now time.Time, nonce string, body, assertion []byte) {
	timestamp := strconv.FormatInt(now.Unix(), 10)
	r.Header.Set("X-Voice-Workload", "auth")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Voice-Workload-Version", "2")
	r.Header.Set("X-Voice-Timestamp", timestamp)
	r.Header.Set("X-Voice-Nonce", nonce)
	r.Header.Set("X-Voice-Signature", assertionBoundWorkloadSignature(key, r.Method, r.URL.EscapedPath(), timestamp, nonce, body, assertion))
}

// responseSignature binds a successful policy body to the authenticated request.
// Auth verifies the exact received bytes before parsing or trusting the policy.
func responseSignature(key []byte, status int, path, timestamp, nonce string, body []byte) string {
	digest := sha256.Sum256(body)
	message := "v1\n" + strconv.Itoa(status) + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + hex.EncodeToString(digest[:])
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// SignWorkloadRequest is the test/controlled-client counterpart of the Auth
// workload wire; production Auth implements the same canonical input in Java.
func SignWorkloadRequest(r *http.Request, key []byte, now time.Time, nonce string) {
	timestamp := strconv.FormatInt(now.Unix(), 10)
	r.Header.Set("X-Voice-Workload", "auth")
	r.Header.Set("X-Voice-Timestamp", timestamp)
	r.Header.Set("X-Voice-Nonce", nonce)
	r.Header.Set("X-Voice-Signature", workloadSignature(key, r.Method, r.URL.EscapedPath(), timestamp, nonce))
}

func (v WorkloadVerifier) Verify(r *http.Request) error {
	if len(v.Key) != 32 || v.Nonces == nil || v.Now == nil {
		return ErrWorkloadUnavailable
	}
	if r == nil || r.Method != http.MethodGet || r.URL.RawQuery != "" || r.ContentLength != 0 ||
		r.Header.Get("X-Voice-Workload") != "auth" {
		return ErrInvalidWorkloadProof
	}
	for _, name := range []string{"X-Voice-Workload", "X-Voice-Timestamp", "X-Voice-Nonce", "X-Voice-Signature"} {
		if len(r.Header.Values(name)) != 1 {
			return ErrInvalidWorkloadProof
		}
	}
	timestamp := r.Header.Get("X-Voice-Timestamp")
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || strconv.FormatInt(seconds, 10) != timestamp {
		return ErrInvalidWorkloadProof
	}
	now := v.Now()
	issued := time.Unix(seconds, 0)
	if issued.Before(now.Add(-30*time.Second)) || issued.After(now.Add(30*time.Second)) {
		return ErrInvalidWorkloadProof
	}
	nonce := r.Header.Get("X-Voice-Nonce")
	parsedNonce, err := uuid.Parse(nonce)
	if err != nil || parsedNonce == uuid.Nil || parsedNonce.String() != nonce {
		return ErrInvalidWorkloadProof
	}
	signature := r.Header.Get("X-Voice-Signature")
	if len(signature) != 43 || strings.ContainsAny(signature, "= \t\r\n") {
		return ErrInvalidWorkloadProof
	}
	expected := workloadSignature(v.Key, r.Method, r.URL.EscapedPath(), timestamp, nonce)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return ErrInvalidWorkloadProof
	}
	used, err := v.Nonces.Use(r.Context(), nonce, 60*time.Second)
	if err != nil {
		return ErrWorkloadUnavailable
	}
	if !used {
		return ErrInvalidWorkloadProof
	}
	return nil
}

// VerifyAssertionBound authenticates an Auth POST and binds both its exact
// body bytes and raw device-authority assertion header to the HMAC proof.
func (v WorkloadVerifier) VerifyAssertionBound(r *http.Request) ([]byte, error) {
	if len(v.Key) != 32 || v.Nonces == nil || v.Now == nil {
		return nil, ErrWorkloadUnavailable
	}
	if r == nil || r.Method != http.MethodPost || r.URL.RawQuery != "" || r.URL.EscapedPath() != r.URL.Path ||
		r.Header.Get("X-Voice-Workload") != "auth" || r.Header.Get("X-Voice-Workload-Version") != "2" {
		return nil, workloadFailureHeader
	}
	for _, name := range []string{"X-Voice-Workload", "X-Voice-Workload-Version", "X-Voice-Timestamp", "X-Voice-Nonce", "X-Voice-Signature", "X-Voice-Device-Authority"} {
		if len(r.Header.Values(name)) != 1 {
			return nil, workloadFailureHeader
		}
	}
	if len(r.Header.Values("Content-Type")) != 1 || r.Header.Get("Content-Type") != "application/json" {
		return nil, workloadFailureHeader
	}
	assertion := []byte(r.Header.Get("X-Voice-Device-Authority"))
	if len(assertion) == 0 || len(assertion) > 16<<10 {
		return nil, workloadFailureHeader
	}
	body, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, 2<<10))
	if err != nil {
		return nil, workloadFailureBody
	}
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	if r.ContentLength >= 0 && int64(len(body)) != r.ContentLength {
		return nil, workloadFailureBody
	}
	timestamp := r.Header.Get("X-Voice-Timestamp")
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || strconv.FormatInt(seconds, 10) != timestamp {
		return nil, workloadFailureTimestamp
	}
	now := v.Now()
	issued := time.Unix(seconds, 0)
	if issued.Before(now.Add(-30*time.Second)) || issued.After(now.Add(30*time.Second)) {
		return nil, workloadFailureTimestamp
	}
	nonce := r.Header.Get("X-Voice-Nonce")
	parsedNonce, err := uuid.Parse(nonce)
	if err != nil || parsedNonce == uuid.Nil || parsedNonce.String() != nonce {
		return nil, workloadFailureNonce
	}
	signature := r.Header.Get("X-Voice-Signature")
	if len(signature) != 43 || strings.ContainsAny(signature, "= \t\r\n") {
		return nil, workloadFailureSignature
	}
	expected := assertionBoundWorkloadSignature(v.Key, r.Method, r.URL.EscapedPath(), timestamp, nonce, body, assertion)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return nil, workloadFailureSignature
	}
	used, err := v.Nonces.Use(r.Context(), nonce, 60*time.Second)
	if err != nil {
		return nil, ErrWorkloadUnavailable
	}
	if !used {
		return nil, workloadFailureReplay
	}
	return assertion, nil
}

// VerifyBody authenticates an internal JSON POST using the established v1
// principal/path/body/timestamp/nonce HMAC inputs and Redis replay guard.
func (v WorkloadVerifier) VerifyBody(r *http.Request) ([]byte, error) {
	body, _, err := v.VerifyBodyWithKey(r)
	return body, err
}

// VerifyBodyWithKey verifies WorkloadProof v1 for the configured principal and
// returns the key that authenticated it so the response can use the same key.
func (v WorkloadVerifier) VerifyBodyWithKey(r *http.Request) ([]byte, []byte, error) {
	if len(v.Key) != 32 || v.Nonces == nil || v.Now == nil ||
		(len(v.PreviousKey) != 0 && (len(v.PreviousKey) != 32 || hmac.Equal(v.Key, v.PreviousKey))) ||
		(len(v.PreviousKey) == 32 && v.PreviousKeyUntil.IsZero()) {
		return nil, nil, ErrWorkloadUnavailable
	}
	principal := v.Principal
	if principal == "" {
		principal = "auth"
	}
	if r == nil || r.Method != http.MethodPost || r.URL.RawQuery != "" || r.URL.EscapedPath() != r.URL.Path || r.Header.Get("X-Voice-Workload") != principal {
		return nil, nil, ErrInvalidWorkloadProof
	}
	for _, name := range []string{"X-Voice-Workload", "X-Voice-Timestamp", "X-Voice-Nonce", "X-Voice-Signature"} {
		if len(r.Header.Values(name)) != 1 {
			return nil, nil, ErrInvalidWorkloadProof
		}
	}
	if len(r.Header.Values("Content-Type")) != 1 || r.Header.Get("Content-Type") != "application/json" {
		return nil, nil, ErrInvalidWorkloadProof
	}
	body, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, 2<<10))
	if err != nil {
		return nil, nil, ErrInvalidWorkloadProof
	}
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	if r.ContentLength >= 0 && int64(len(body)) != r.ContentLength {
		return nil, nil, ErrInvalidWorkloadProof
	}
	timestamp := r.Header.Get("X-Voice-Timestamp")
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || strconv.FormatInt(seconds, 10) != timestamp {
		return nil, nil, ErrInvalidWorkloadProof
	}
	now := v.Now()
	issued := time.Unix(seconds, 0)
	if issued.Before(now.Add(-30*time.Second)) || issued.After(now.Add(30*time.Second)) {
		return nil, nil, ErrInvalidWorkloadProof
	}
	nonce := r.Header.Get("X-Voice-Nonce")
	parsedNonce, err := uuid.Parse(nonce)
	if err != nil || parsedNonce == uuid.Nil || parsedNonce.String() != nonce {
		return nil, nil, ErrInvalidWorkloadProof
	}
	signature := r.Header.Get("X-Voice-Signature")
	if len(signature) != 43 || strings.ContainsAny(signature, "= \t\r\n") {
		return nil, nil, ErrInvalidWorkloadProof
	}
	currentExpected := bodyWorkloadSignature(v.Key, r.Method, r.URL.EscapedPath(), timestamp, nonce, body)
	currentValid := hmac.Equal([]byte(signature), []byte(currentExpected))
	previousValid := false
	if len(v.PreviousKey) == 32 {
		previousExpected := bodyWorkloadSignature(v.PreviousKey, r.Method, r.URL.EscapedPath(), timestamp, nonce, body)
		matchesPrevious := hmac.Equal([]byte(signature), []byte(previousExpected))
		previousValid = matchesPrevious && now.Before(v.PreviousKeyUntil)
	}
	if !currentValid && !previousValid {
		return nil, nil, ErrInvalidWorkloadProof
	}
	used, err := v.Nonces.Use(r.Context(), nonce, 60*time.Second)
	if err != nil {
		return nil, nil, ErrWorkloadUnavailable
	}
	if !used {
		return nil, nil, ErrInvalidWorkloadProof
	}
	if currentValid {
		return body, append([]byte(nil), v.Key...), nil
	}
	return body, append([]byte(nil), v.PreviousKey...), nil
}
