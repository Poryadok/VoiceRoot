package partysource

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/principal"
)

func TestClientPreservesNativeAdmittedEpochDifferentFromRequestDevice(t *testing.T) {
	s := Subject{uuid.New(), uuid.New(), uuid.New(), 7}
	c := &GRPCClient{Issuer: issuerFunc(func(principal.ServiceInput) (string, error) { return "token", nil }), Client: rpcFunc(func(context.Context, *callsv1.GetMatchmakingPartySnapshotRequest) (*callsv1.GetMatchmakingPartySnapshotResponse, error) {
		r := privateSoloResponse(s)
		r.Kind = callsv1.MatchmakingPartySnapshotKind_MATCHMAKING_PARTY_SNAPSHOT_KIND_VOICE_ROSTER
		r.RoomId = uuid.NewString()
		r.RosterVersion = 2
		r.MemberAuthorities[0].AdmittedSessionEpoch = 3
		return r, nil
	})}
	got, err := c.Snapshot(context.Background(), s)
	require.NoError(t, err)
	require.Equal(t, uint32(1), got.MemberAuthorityVersion)
	require.Len(t, got.MemberAuthorities, 1)
	require.Equal(t, int64(3), got.MemberAuthorities[0].SessionEpoch)
	require.Equal(t, s.AccountID, got.MemberAuthorities[0].AccountID)
	require.Equal(t, [32]byte{2}, got.SourceAuthorityHash)
}
