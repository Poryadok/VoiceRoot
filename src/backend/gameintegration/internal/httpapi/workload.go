package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
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
	Key    []byte
	Now    func() time.Time
	Nonces NonceStore
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
