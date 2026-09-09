package principal

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	serviceType       = "service"
	delegatedUserType = "delegated_user"
	maxCredentialTTL  = 30 * time.Second
)

type ServiceInput struct {
	Audience    string
	RPC         string
	RequestID   string
	RequestHash string
}

type DelegatedUserInput struct {
	Audience     string
	RPC          string
	RequestID    string
	RequestHash  string
	AccountID    string
	ProfileID    string
	SessionEpoch int64
}

type IssuerConfig struct {
	Issuer     string
	KeyID      string
	PrivateKey *rsa.PrivateKey
	Clock      func() time.Time
}

type Issuer struct {
	issuer     string
	keyID      string
	privateKey *rsa.PrivateKey
	clock      func() time.Time
}

func NewIssuer(config IssuerConfig) (*Issuer, error) {
	if strings.TrimSpace(config.Issuer) == "" || strings.TrimSpace(config.KeyID) == "" || config.PrivateKey == nil {
		return nil, fmt.Errorf("issuer, key id, and private key are required")
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	return &Issuer{issuer: config.Issuer, keyID: config.KeyID, privateKey: config.PrivateKey, clock: config.Clock}, nil
}

func (i *Issuer) IssueService(input ServiceInput) (string, error) {
	if err := validateBinding(input.Audience, input.RPC, input.RequestID, input.RequestHash); err != nil {
		return "", err
	}
	return i.issue(rawClaims{
		Type:        serviceType,
		Issuer:      i.issuer,
		Subject:     "service:" + i.issuer,
		Audience:    input.Audience,
		RPC:         input.RPC,
		RequestID:   input.RequestID,
		RequestHash: input.RequestHash,
	})
}

func (i *Issuer) IssueDelegatedUser(input DelegatedUserInput) (string, error) {
	if err := validateBinding(input.Audience, input.RPC, input.RequestID, input.RequestHash); err != nil {
		return "", err
	}
	if strings.TrimSpace(input.AccountID) == "" || strings.TrimSpace(input.ProfileID) == "" || input.SessionEpoch <= 0 {
		return "", fmt.Errorf("account id, profile id, and positive session epoch are required")
	}
	return i.issue(rawClaims{
		Type:         delegatedUserType,
		Issuer:       i.issuer,
		Subject:      input.AccountID,
		Audience:     input.Audience,
		RPC:          input.RPC,
		RequestID:    input.RequestID,
		RequestHash:  input.RequestHash,
		AccountID:    input.AccountID,
		ProfileID:    input.ProfileID,
		SessionEpoch: input.SessionEpoch,
	})
}

func (i *Issuer) issue(claims rawClaims) (string, error) {
	now := i.clock().UTC()
	claims.IssuedAt = now.Unix()
	claims.NotBefore = now.Unix()
	claims.ExpiresAt = now.Add(maxCredentialTTL).Unix()
	jwtID, err := randomID()
	if err != nil {
		return "", err
	}
	claims.JWTID = jwtID

	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": i.keyID})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, i.privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func randomID() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func validateBinding(audience, rpc, requestID, requestHash string) error {
	if strings.TrimSpace(audience) == "" || strings.TrimSpace(rpc) == "" || strings.TrimSpace(requestID) == "" || strings.TrimSpace(requestHash) == "" {
		return fmt.Errorf("audience, rpc, request id, and request hash are required")
	}
	return nil
}

type rawClaims struct {
	Type         string `json:"principal_type"`
	Issuer       string `json:"iss"`
	Subject      string `json:"sub"`
	Audience     string `json:"aud"`
	RPC          string `json:"rpc"`
	RequestID    string `json:"request_id"`
	RequestHash  string `json:"request_hash"`
	AccountID    string `json:"account_id,omitempty"`
	ProfileID    string `json:"profile_id,omitempty"`
	SessionEpoch int64  `json:"session_epoch,omitempty"`
	IssuedAt     int64  `json:"iat"`
	NotBefore    int64  `json:"nbf"`
	ExpiresAt    int64  `json:"exp"`
	JWTID        string `json:"jti"`
}
