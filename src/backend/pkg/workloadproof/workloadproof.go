// Package workloadproof authenticates narrow internal HTTP calls between
// Voice workloads. It binds the principal, request bytes, response bytes and
// one-time nonce without trusting forwarded user identity metadata.
package workloadproof

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidProof = errors.New("invalid workload proof")
	ErrUnavailable  = errors.New("workload proof unavailable")
)

const (
	workloadHeader        = "X-Voice-Workload"
	audienceHeader        = "X-Voice-Audience"
	timestampHeader       = "X-Voice-Timestamp"
	nonceHeader           = "X-Voice-Nonce"
	signatureHeader       = "X-Voice-Signature"
	responseTimestamp     = "X-Voice-Response-Timestamp"
	responseNonce         = "X-Voice-Response-Nonce"
	responseSignature     = "X-Voice-Response-Signature"
	defaultRequestMaxSkew = 30 * time.Second
	defaultNonceTTL       = 61 * time.Second
)

// NonceStore atomically records a nonce if it has not been used before.
type NonceStore interface {
	Use(context.Context, string, time.Duration) (bool, error)
}

type Verifier struct {
	Key       []byte
	Principal string
	Audience  string
	Now       func() time.Time
	Nonces    NonceStore
}

type Proof struct {
	Principal string
	Audience  string
	Timestamp int64
	Nonce     string
	Body      []byte
}

func (v Verifier) VerifyRequest(r *http.Request, maxBodyBytes int64) (Proof, error) {
	if len(v.Key) != 32 || strings.TrimSpace(v.Principal) == "" || strings.TrimSpace(v.Audience) == "" || v.Now == nil || v.Nonces == nil {
		return Proof{}, ErrUnavailable
	}
	if r == nil || r.Method != http.MethodPost || r.URL == nil || r.URL.RawQuery != "" ||
		r.Header.Get(workloadHeader) != v.Principal || r.Header.Get(audienceHeader) != v.Audience ||
		r.Header.Get("Content-Type") != "application/json" || len(r.Header.Values("Content-Encoding")) != 0 || maxBodyBytes <= 0 {
		return Proof{}, ErrInvalidProof
	}
	for _, name := range []string{workloadHeader, audienceHeader, timestampHeader, nonceHeader, signatureHeader, "Content-Type"} {
		if len(r.Header.Values(name)) != 1 {
			return Proof{}, ErrInvalidProof
		}
	}
	if r.ContentLength > maxBodyBytes || r.Body == nil {
		return Proof{}, ErrInvalidProof
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if err != nil || int64(len(body)) > maxBodyBytes || len(body) == 0 {
		return Proof{}, ErrInvalidProof
	}
	timestamp := r.Header.Get(timestampHeader)
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || strconv.FormatInt(seconds, 10) != timestamp {
		return Proof{}, ErrInvalidProof
	}
	now := v.Now()
	issued := time.Unix(seconds, 0)
	if issued.Before(now.Add(-defaultRequestMaxSkew)) || issued.After(now.Add(defaultRequestMaxSkew)) {
		return Proof{}, ErrInvalidProof
	}
	nonce := r.Header.Get(nonceHeader)
	parsedNonce, err := uuid.Parse(nonce)
	if err != nil || parsedNonce == uuid.Nil || parsedNonce.String() != nonce {
		return Proof{}, ErrInvalidProof
	}
	path := r.URL.EscapedPath()
	signature := r.Header.Get(signatureHeader)
	if !validSignature(signature) || !hmac.Equal([]byte(signature), []byte(requestSignature(v.Key, v.Principal, v.Audience, r.Method, path, timestamp, nonce, body))) {
		return Proof{}, ErrInvalidProof
	}
	used, err := v.Nonces.Use(r.Context(), v.Principal+":"+nonce, defaultNonceTTL)
	if err != nil {
		return Proof{}, ErrUnavailable
	}
	if !used {
		return Proof{}, ErrInvalidProof
	}
	return Proof{Principal: v.Principal, Audience: v.Audience, Timestamp: seconds, Nonce: nonce, Body: body}, nil
}

// SignRequest sets the wire headers for a request whose exact bytes are body.
func SignRequest(r *http.Request, key []byte, principal, audience string, now time.Time, nonce string, body []byte) {
	timestamp := strconv.FormatInt(now.UTC().Unix(), 10)
	r.Header.Set(workloadHeader, principal)
	r.Header.Set(audienceHeader, audience)
	r.Header.Set(timestampHeader, timestamp)
	r.Header.Set(nonceHeader, nonce)
	r.Header.Set(signatureHeader, requestSignature(key, principal, audience, r.Method, r.URL.EscapedPath(), timestamp, nonce, body))
	r.Header.Set("Content-Type", "application/json")
}

func requestSignature(key []byte, principal, audience, method, path, timestamp, nonce string, body []byte) string {
	digest := sha256.Sum256(body)
	message := "v1\n" + principal + "\n" + audience + "\n" + method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + hex.EncodeToString(digest[:])
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func SignResponse(w http.ResponseWriter, key []byte, status int, path, timestamp, nonce string, body []byte) {
	w.Header().Set(responseTimestamp, timestamp)
	w.Header().Set(responseNonce, nonce)
	w.Header().Set(responseSignature, responseMAC(key, status, path, timestamp, nonce, body))
}

func VerifyResponse(key []byte, status int, path, timestamp, nonce string, body []byte, headers http.Header) (Proof, error) {
	if len(key) != 32 || headers == nil {
		return Proof{}, ErrUnavailable
	}
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || strconv.FormatInt(seconds, 10) != timestamp {
		return Proof{}, ErrInvalidProof
	}
	parsedNonce, err := uuid.Parse(nonce)
	if err != nil || parsedNonce == uuid.Nil || parsedNonce.String() != nonce {
		return Proof{}, ErrInvalidProof
	}
	for _, name := range []string{responseTimestamp, responseNonce, responseSignature} {
		if len(headers.Values(name)) != 1 {
			return Proof{}, ErrInvalidProof
		}
	}
	if headers.Get(responseTimestamp) != timestamp || headers.Get(responseNonce) != nonce {
		return Proof{}, ErrInvalidProof
	}
	signature := headers.Get(responseSignature)
	if !validSignature(signature) || !hmac.Equal([]byte(signature), []byte(responseMAC(key, status, path, timestamp, nonce, body))) {
		return Proof{}, ErrInvalidProof
	}
	return Proof{Timestamp: seconds, Nonce: nonce, Body: append([]byte(nil), body...)}, nil
}

func responseMAC(key []byte, status int, path, timestamp, nonce string, body []byte) string {
	digest := sha256.Sum256(body)
	message := fmt.Sprintf("v1\n%d\n%s\n%s\n%s\n%s", status, path, timestamp, nonce, hex.EncodeToString(digest[:]))
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func validSignature(signature string) bool {
	if len(signature) != 43 || strings.ContainsAny(signature, "= \t\r\n") {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(signature)
	return err == nil && len(decoded) == sha256.Size
}
