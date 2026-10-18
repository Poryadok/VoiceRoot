package partysource

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/principal"
)

type rpcFunc func(context.Context, *callsv1.GetMatchmakingPartySnapshotRequest) (*callsv1.GetMatchmakingPartySnapshotResponse, error)

func (f rpcFunc) GetMatchmakingPartySnapshot(ctx context.Context, r *callsv1.GetMatchmakingPartySnapshotRequest, _ ...grpc.CallOption) (*callsv1.GetMatchmakingPartySnapshotResponse, error) {
	return f(ctx, r)
}

type issuerFunc func(principal.ServiceInput) (string, error)

func (f issuerFunc) IssueService(i principal.ServiceInput) (string, error) { return f(i) }
func TestClientBindsFreshServiceCredentialAndDropsCallerMetadata(t *testing.T) {
	s := Subject{uuid.New(), uuid.New(), uuid.New(), 7}
	expectedHash, err := principal.RequestHash(&callsv1.GetMatchmakingPartySnapshotRequest{SearchOperationId: s.SearchOperationID.String(), InitiatorAccountId: s.AccountID.String(), InitiatorProfileId: s.ProfileID.String(), InitiatorSessionEpoch: s.SessionEpoch})
	require.NoError(t, err)
	calls := 0
	tokens := 0
	c := &GRPCClient{Issuer: issuerFunc(func(i principal.ServiceInput) (string, error) {
		tokens++
		require.Equal(t, "voice", i.Audience)
		require.Equal(t, SnapshotMethod, i.RPC)
		require.Equal(t, s.SearchOperationID.String(), i.RequestID)
		require.Equal(t, expectedHash, i.RequestHash)
		return "fresh-token", nil
	}), Client: rpcFunc(func(ctx context.Context, r *callsv1.GetMatchmakingPartySnapshotRequest) (*callsv1.GetMatchmakingPartySnapshotResponse, error) {
		calls++
		md, _ := metadata.FromOutgoingContext(ctx)
		require.Equal(t, metadata.Pairs("authorization", "Bearer fresh-token", "x-request-id", s.SearchOperationID.String()), md)
		require.Equal(t, s.AccountID.String(), r.InitiatorAccountId)
		require.Equal(t, s.ProfileID.String(), r.InitiatorProfileId)
		require.Equal(t, s.SessionEpoch, r.InitiatorSessionEpoch)
		return &callsv1.GetMatchmakingPartySnapshotResponse{ProtocolVersion: 1, Kind: callsv1.MatchmakingPartySnapshotKind_MATCHMAKING_PARTY_SNAPSHOT_KIND_SOLO, MemberProfileIds: []string{s.ProfileID.String()}}, nil
	})}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer caller", "x-profile-id", "spoof"))
	for i := 0; i < 2; i++ {
		snap, err := c.Snapshot(ctx, s)
		require.NoError(t, err)
		require.Equal(t, "SOLO", snap.Kind)
	}
	require.Equal(t, 2, calls)
	require.Equal(t, 2, tokens)
}

func TestClientMalformedAuthorityNeverBecomesChangedOrSolo(t *testing.T) {
	s := Subject{uuid.New(), uuid.New(), uuid.New(), 7}
	for _, tc := range []struct {
		name   string
		mutate func(*callsv1.GetMatchmakingPartySnapshotResponse)
	}{
		{"protocol", func(r *callsv1.GetMatchmakingPartySnapshotResponse) { r.ProtocolVersion = 2 }},
		{"kind", func(r *callsv1.GetMatchmakingPartySnapshotResponse) { r.Kind = 99 }},
		{"unknown fields", func(r *callsv1.GetMatchmakingPartySnapshotResponse) { r.ProtoReflect().SetUnknown([]byte{0x30, 0x01}) }},
		{"empty members", func(r *callsv1.GetMatchmakingPartySnapshotResponse) { r.MemberProfileIds = nil }},
		{"duplicate", func(r *callsv1.GetMatchmakingPartySnapshotResponse) {
			r.MemberProfileIds = append(r.MemberProfileIds, r.MemberProfileIds[0])
		}},
		{"noncanonical", func(r *callsv1.GetMatchmakingPartySnapshotResponse) {
			r.MemberProfileIds[0] = "10000000000040008000000000000001"
		}},
		{"no initiator", func(r *callsv1.GetMatchmakingPartySnapshotResponse) { r.MemberProfileIds[0] = uuid.NewString() }},
		{"solo room", func(r *callsv1.GetMatchmakingPartySnapshotResponse) { r.RoomId = uuid.NewString() }},
		{"solo version", func(r *callsv1.GetMatchmakingPartySnapshotResponse) { r.RosterVersion = 1 }},
		{"roster absent room", func(r *callsv1.GetMatchmakingPartySnapshotResponse) {
			r.Kind = callsv1.MatchmakingPartySnapshotKind_MATCHMAKING_PARTY_SNAPSHOT_KIND_VOICE_ROSTER
			r.RosterVersion = 1
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &GRPCClient{Issuer: issuerFunc(func(principal.ServiceInput) (string, error) { return "token", nil }), Client: rpcFunc(func(context.Context, *callsv1.GetMatchmakingPartySnapshotRequest) (*callsv1.GetMatchmakingPartySnapshotResponse, error) {
				r := &callsv1.GetMatchmakingPartySnapshotResponse{ProtocolVersion: 1, Kind: callsv1.MatchmakingPartySnapshotKind_MATCHMAKING_PARTY_SNAPSHOT_KIND_SOLO, MemberProfileIds: []string{s.ProfileID.String()}}
				tc.mutate(r)
				return r, nil
			})}
			_, err := c.Snapshot(context.Background(), s)
			require.ErrorIs(t, err, ErrUnknown)
			require.NotErrorIs(t, err, ErrIneligible)
		})
	}
}

func TestClientMissingIssuerOrInvalidSubjectNeverCallsRPC(t *testing.T) {
	rpc := rpcFunc(func(context.Context, *callsv1.GetMatchmakingPartySnapshotRequest) (*callsv1.GetMatchmakingPartySnapshotResponse, error) {
		t.Fatal("invalid client reached authority")
		return nil, nil
	})
	_, err := (&GRPCClient{Client: rpc}).Snapshot(context.Background(), Subject{uuid.New(), uuid.New(), uuid.New(), 7})
	require.Error(t, err)
	_, err = (&GRPCClient{Client: rpc, Issuer: issuerFunc(func(principal.ServiceInput) (string, error) {
		t.Fatal("invalid subject reached issuer")
		return "", nil
	})}).Snapshot(context.Background(), Subject{})
	require.Error(t, err)
}
func TestClientOnlyTrustedSubjectReasonsAreDefinitive(t *testing.T) {
	typed, _ := status.New(codes.Unauthenticated, "stale").WithDetails(&errdetails.ErrorInfo{Domain: "voice.party", Reason: "STALE_PARTY_SUBJECT"})
	foreign, _ := status.New(codes.Unauthenticated, "stale").WithDetails(&errdetails.ErrorInfo{Domain: "foreign", Reason: "STALE_PARTY_SUBJECT"})
	for _, tc := range []struct {
		err      error
		definite bool
	}{{typed.Err(), true}, {foreign.Err(), false}, {status.Error(codes.Unauthenticated, "bad service credential"), false}, {status.Error(codes.FailedPrecondition, "unknown failure"), false}, {status.Error(codes.Unavailable, "down"), false}} {
		c := &GRPCClient{Issuer: issuerFunc(func(principal.ServiceInput) (string, error) { return "token", nil }), Client: rpcFunc(func(context.Context, *callsv1.GetMatchmakingPartySnapshotRequest) (*callsv1.GetMatchmakingPartySnapshotResponse, error) {
			return nil, tc.err
		})}
		_, err := c.Snapshot(context.Background(), Subject{uuid.New(), uuid.New(), uuid.New(), 7})
		require.Error(t, err)
		require.Equal(t, tc.definite, errors.Is(err, ErrIneligible))
	}
}
