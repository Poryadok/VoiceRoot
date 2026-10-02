package nodecache

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/federation/protocol"
)

type participantList []MediaParticipant

func (p participantList) ListFederatedParticipants(context.Context) ([]MediaParticipant, error) {
	return p, nil
}

type retryingParticipantList struct {
	calls  int
	cancel context.CancelFunc
}

func (p *retryingParticipantList) ListFederatedParticipants(context.Context) ([]MediaParticipant, error) {
	p.calls++
	if p.calls == 1 {
		return nil, errors.New("temporary LiveKit list failure")
	}
	p.cancel()
	return nil, nil
}

type participantEjectLog struct{ ejected []string }

func (e *participantEjectLog) EjectFederatedParticipant(_ context.Context, room, identity string) error {
	e.ejected = append(e.ejected, room+"/"+identity)
	return nil
}

func TestMediaWatchdogEjectsParticipantWhenSignedAuthorityExpires(t *testing.T) {
	pub, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Millisecond)
	scope := Scope{Issuer: "master", Environment: "sandbox", NodeID: uuid.NewString(), SpaceID: uuid.NewString(), Generation: 1, Epoch: 1}
	cache := New(scope, map[string]ed25519.PublicKey{"authority-1": pub})
	accountID, profileID, resourceID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	policy := protocol.Snapshot{Version: 1, Complete: true, PageCount: 1, Revision: 1, ValidUntil: now.Add(time.Second).UnixMilli(), Permissions: []protocol.Permission{{
		AccountID: accountID, ProfileID: profileID, ResourceID: resourceID, SessionEpoch: 3, Actions: []string{"media"},
	}}}
	applySnapshot(t, cache, private, scope, policy, now)
	lease, err := protocol.SignEnvelope(private, "authority-1", protocol.Claims{
		Version: 1, Kind: "lease", Issuer: scope.Issuer, Audience: "voice-node", Environment: scope.Environment,
		NodeID: scope.NodeID, SpaceID: scope.SpaceID, Generation: scope.Generation, Epoch: scope.Epoch,
		Revision: policy.Revision, IssuedAt: now.UnixMilli(), ExpiresAt: policy.ValidUntil, Hash: protocol.SnapshotDigest(policy),
	})
	require.NoError(t, err)
	require.NoError(t, cache.AcceptLease(lease, now))
	ejector := &participantEjectLog{}
	watchdog := &MediaWatchdog{
		Policy: cache, Participants: participantList{{RoomName: "space-room", Identity: profileID, AccountID: accountID, ProfileID: profileID, ResourceID: resourceID, SessionEpoch: 3}},
		Ejector: ejector, Interval: 200 * time.Millisecond, ClockSkew: 250 * time.Millisecond,
		OperationTTL: time.Second, Now: func() time.Time { return now.Add(800 * time.Millisecond) },
	}
	require.NoError(t, watchdog.Sweep(context.Background()))
	require.Equal(t, []string{"space-room/" + profileID}, ejector.ejected)
}

func TestMediaWatchdogRejectsUnboundedSweepConfiguration(t *testing.T) {
	watchdog := &MediaWatchdog{Interval: 251 * time.Millisecond}
	require.ErrorIs(t, watchdog.Sweep(context.Background()), ErrMediaWatchdogUnavailable)
}

func TestMediaWatchdogRunFailsFastWhenRequiredDependenciesAreMissing(t *testing.T) {
	watchdog := &MediaWatchdog{Interval: 5 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	err := watchdog.Run(ctx)

	require.ErrorIs(t, err, ErrMediaWatchdogUnavailable)
}

func TestMediaWatchdogRunReportsTransientSweepFailureAndKeepsRetrying(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	participants := &retryingParticipantList{cancel: cancel}
	reported := make(chan error, 1)
	watchdog := &MediaWatchdog{
		Policy: New(Scope{}, nil), Participants: participants, Ejector: &participantEjectLog{},
		Interval: 5 * time.Millisecond, OperationTTL: time.Second, Now: time.Now,
		OnError: func(err error) { reported <- err },
	}

	err := watchdog.Run(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 2, participants.calls, "a transient sweep error must not permanently stop revocation enforcement")
	require.ErrorContains(t, <-reported, "temporary LiveKit list failure")
}
