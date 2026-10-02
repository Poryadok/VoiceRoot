package main

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSignedAuthorityRejectsTamperScopeAndExpiry(t *testing.T) {
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Unix(1800000000, 0)
	claims := Claims{Version: 1, Kind: "lease", Issuer: "master", Audience: "voice-node", Environment: "sandbox", NodeID: "00000000-0000-4000-8000-000000000001", SpaceID: "00000000-0000-4000-8000-000000000002", Generation: 1, Epoch: 1, Revision: 2, IssuedAt: now.UnixMilli(), ExpiresAt: now.Add(2 * time.Second).UnixMilli(), Hash: digest([]byte("snapshot"))}
	envelope, err := signEnvelope(key, "key-1", claims)
	require.NoError(t, err)
	_, err = verifyEnvelope(pub, envelope, claims, now)
	require.NoError(t, err)
	for _, field := range []string{"node", "space", "env", "revision", "epoch", "generation", "audience"} {
		t.Run(field, func(t *testing.T) {
			expected := claims
			switch field {
			case "node":
				expected.NodeID = "00000000-0000-4000-8000-000000000003"
			case "space":
				expected.SpaceID = "00000000-0000-4000-8000-000000000004"
			case "env":
				expected.Environment = "prod"
			case "revision":
				expected.Revision++
			case "epoch":
				expected.Epoch++
			case "generation":
				expected.Generation++
			case "audience":
				expected.Audience = "user"
			}
			_, err := verifyEnvelope(pub, envelope, expected, now)
			require.Error(t, err)
		})
	}
	_, err = verifyEnvelope(pub, envelope, claims, now.Add(2*time.Second))
	require.Error(t, err)
	envelope.Payload = base64.RawURLEncoding.EncodeToString([]byte(`{"version":1}`))
	_, err = verifyEnvelope(pub, envelope, claims, now)
	require.Error(t, err)
}

func TestPeerRequiresVerifiedCurrentCertificate(t *testing.T) {
	now := time.Now()
	cert := &x509.Certificate{Raw: []byte("certificate"), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute)}
	r := httptest.NewRequest("GET", "https://master/v1/authority", nil)
	r.Header.Set("X-Client-Cert", "certificate")
	_, err := peerFingerprint(r, now)
	require.Error(t, err)
	r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}}
	_, err = peerFingerprint(r, now)
	require.Error(t, err)
	r.TLS.VerifiedChains = [][]*x509.Certificate{{cert}}
	pin, err := peerFingerprint(r, now)
	require.NoError(t, err)
	require.Len(t, pin, 64)
	_, err = peerFingerprint(r, now.Add(2*time.Minute))
	require.Error(t, err)
}

func TestPeerRevalidatesFullChainAtRequestTime(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	rootKey, err := ecdsaKey(t)
	require.NoError(t, err)
	intermediateKey, err := ecdsaKey(t)
	require.NoError(t, err)
	leafKey, err := ecdsaKey(t)
	require.NoError(t, err)
	rootTmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "root"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(10 * time.Minute), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTmpl, rootTmpl, rootKey.Public(), rootKey)
	require.NoError(t, err)
	root, err := x509.ParseCertificate(rootDER)
	require.NoError(t, err)
	intermediateTmpl := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "intermediate"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Minute), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	intermediateDER, err := x509.CreateCertificate(rand.Reader, intermediateTmpl, root, intermediateKey.Public(), rootKey)
	require.NoError(t, err)
	intermediate, err := x509.ParseCertificate(intermediateDER)
	require.NoError(t, err)
	leafTmpl := &x509.Certificate{SerialNumber: big.NewInt(3), Subject: pkix.Name{CommonName: "node"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(10 * time.Minute), ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, KeyUsage: x509.KeyUsageDigitalSignature}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, intermediate, leafKey.Public(), intermediateKey)
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(leafDER)
	require.NoError(t, err)
	r := httptest.NewRequest("GET", "https://master/v1/authority", nil)
	r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf, intermediate}, VerifiedChains: [][]*x509.Certificate{{leaf, intermediate, root}}}
	_, err = peerFingerprint(r, now)
	require.NoError(t, err)
	_, err = peerFingerprint(r, now.Add(2*time.Minute))
	require.Error(t, err, "the intermediate expired after the TLS connection was established")
}

func ecdsaKey(t *testing.T) (*ecdsa.PrivateKey, error) {
	t.Helper()
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

func TestSnapshotRequiresCompleteKnownSchema(t *testing.T) {
	now := time.Now()
	good := Snapshot{Version: 1, Complete: true, PageCount: 1, Revision: 1, ValidUntil: now.Add(time.Second).UnixMilli(), Permissions: []Permission{}}
	require.NoError(t, good.Validate(now))
	for _, field := range []string{"schema", "complete", "pages", "revision", "expiry", "unbounded", "missing"} {
		t.Run(field, func(t *testing.T) {
			s := good
			switch field {
			case "schema":
				s.Version = 2
			case "complete":
				s.Complete = false
			case "pages":
				s.PageCount = 2
			case "revision":
				s.Revision = 0
			case "expiry":
				s.ValidUntil = now.UnixMilli()
			case "unbounded":
				s.ValidUntil = now.Add(time.Hour).UnixMilli()
			case "missing":
				s.Permissions = nil
			}
			require.Error(t, s.Validate(now))
		})
	}
}
