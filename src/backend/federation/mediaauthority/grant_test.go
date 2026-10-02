package mediaauthority

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMediaGrantBindsEveryAuthorityAndRoomDimension(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	grant := Grant{Version: 1, Issuer: "master", Audience: "voice-node-media", Environment: "sandbox",
		NodeID: uuid.NewString(), SpaceID: uuid.NewString(), Generation: 1, AuthorityEpoch: 2,
		AccountID: uuid.NewString(), ProfileID: uuid.NewString(), ResourceID: uuid.NewString(), SessionEpoch: 7,
		RoomName: "explicit-authoritative-room", IssuedAt: now.UnixMilli(), ExpiresAt: now.Add(30 * time.Second).UnixMilli()}
	verifier := Verifier{Issuer: grant.Issuer, Environment: grant.Environment, NodeID: grant.NodeID,
		Keys: map[string]ed25519.PublicKey{"current": public}}
	token, err := Sign(private, "current", grant, now)
	require.NoError(t, err)
	accepted, err := verifier.Verify(token, grant.RoomName, grant.ProfileID, now, 250*time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, grant, accepted)
	for name, change := range map[string]func(*Grant){
		"issuer":             func(g *Grant) { g.Issuer = "foreign" },
		"audience":           func(g *Grant) { g.Audience = "voice-node" },
		"node":               func(g *Grant) { g.NodeID = uuid.NewString() },
		"environment":        func(g *Grant) { g.Environment = "prod" },
		"missing account":    func(g *Grant) { g.AccountID = "" },
		"zero epoch":         func(g *Grant) { g.SessionEpoch = 0 },
		"unbounded validity": func(g *Grant) { g.ExpiresAt = now.Add(time.Minute).UnixMilli() },
	} {
		t.Run(name, func(t *testing.T) {
			changed := grant
			change(&changed)
			other, signErr := Sign(private, "current", changed, now)
			if signErr == nil {
				_, err := verifier.Verify(other, grant.RoomName, grant.ProfileID, now, 0)
				require.ErrorIs(t, err, ErrDenied)
			} else {
				require.ErrorIs(t, signErr, ErrDenied)
			}
		})
	}
	_, err = verifier.Verify(token, "another-room", grant.ProfileID, now, 0)
	require.ErrorIs(t, err, ErrDenied)
	_, err = verifier.Verify(token, grant.RoomName, uuid.NewString(), now, 0)
	require.ErrorIs(t, err, ErrDenied)
	_, err = verifier.Verify(token, grant.RoomName, grant.ProfileID, now.Add(30*time.Second), 0)
	require.ErrorIs(t, err, ErrDenied)
	_, err = verifier.Verify(token, grant.RoomName, grant.ProfileID, now.Add(30*time.Second-200*time.Millisecond), 250*time.Millisecond)
	require.ErrorIs(t, err, ErrDenied)
	_, err = verifier.Verify(token, grant.RoomName, grant.ProfileID, now, 251*time.Millisecond)
	require.ErrorIs(t, err, ErrDenied)
	_, err = verifier.Verify(token+"x", grant.RoomName, grant.ProfileID, now, 0)
	require.ErrorIs(t, err, ErrDenied)
}
