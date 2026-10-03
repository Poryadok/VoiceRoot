package mediaauthority

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/federation/protocol"
)

func TestBootHeartbeatDirectoryScopesBoundsAndExpiresReceiverRequests(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	verifier := Verifier{Issuer: "master", Environment: "sandbox", NodeID: uuid.NewString(), Keys: map[string]ed25519.PublicKey{"current": public}}
	dir := t.TempDir()
	now := time.Now()
	active := func() ([]string, error) {
		return ActiveBootNonces(dir, verifier.Issuer, verifier.Environment, verifier.NodeID, now)
	}
	create := func(v Verifier) (*Registry, *BootHeartbeat) {
		r, err := NewBootRegistry(v, 0)
		require.NoError(t, err)
		h, err := NewBootHeartbeat(r, dir)
		require.NoError(t, err)
		t.Cleanup(func() { _ = h.Close() })
		require.NoError(t, h.Pulse(now))
		return r, h
	}
	first, heartbeat := create(verifier)
	second, _ := create(verifier)
	expected := []string{first.BootNonce(), second.BootNonce()}
	slices.Sort(expected)
	nonces, err := active()
	require.NoError(t, err)
	require.Equal(t, expected, nonces)
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	require.NoError(t, err)
	require.Len(t, files, 2)
	now = now.Add(time.Second)
	nonces, err = active()
	require.NoError(t, err)
	require.Empty(t, nonces, "dead receivers cannot keep heartbeats current")
	require.NoError(t, heartbeat.Pulse(now))
	nonces, err = active()
	require.NoError(t, err)
	require.Equal(t, []string{first.BootNonce()}, nonces)
	require.NoError(t, heartbeat.Close())
	require.ErrorIs(t, heartbeat.Pulse(now), ErrDenied)
	foreign := verifier
	foreign.NodeID = uuid.NewString()
	_, other := create(foreign)
	_, err = active()
	require.ErrorIs(t, err, ErrDenied, "another node cannot enter this receiver set")
	require.NoError(t, other.Close())
	for i := 0; i <= protocol.MaxReceiverBootNonces; i++ {
		create(verifier)
	}
	_, err = active()
	require.ErrorIs(t, err, ErrDenied, "control-plane work stays bounded")
}

func TestBootHeartbeatDirectoryRefusesMalformedOrUnconfiguredRequests(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	v := Verifier{Issuer: "master", Environment: "sandbox", NodeID: uuid.NewString(), Keys: map[string]ed25519.PublicKey{"current": public}}
	legacy, err := NewRegistry(v, 0)
	require.NoError(t, err)
	_, err = NewBootHeartbeat(legacy, t.TempDir())
	require.ErrorIs(t, err, ErrDenied)
	r, err := NewBootRegistry(v, 0)
	require.NoError(t, err)
	_, err = NewBootHeartbeat(r, "relative")
	require.ErrorIs(t, err, ErrDenied)
	dir := t.TempDir()
	h, err := NewBootHeartbeat(r, dir)
	require.NoError(t, err)
	defer h.Close()
	path := filepath.Join(dir, r.BootNonce()+".json")
	for _, raw := range []string{`{"unknown":true}`, `{} {}`, string(make([]byte, 16385))} {
		require.NoError(t, os.WriteFile(path, []byte(raw), 0600))
		_, err = ActiveBootNonces(dir, v.Issuer, v.Environment, v.NodeID, time.Now())
		require.ErrorIs(t, err, ErrDenied)
	}
}
