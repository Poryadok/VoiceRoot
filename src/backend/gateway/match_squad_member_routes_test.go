package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/principal"
)

type publicMatchSquadMemberClient struct {
	callsv1.MatchSquadMemberServiceClient
	verify func(context.Context, any, string, string)
	calls  int
}

func (c *publicMatchSquadMemberClient) JoinMatchSquadRoom(ctx context.Context, req *callsv1.JoinMatchSquadRoomRequest, _ ...grpc.CallOption) (*callsv1.JoinMatchSquadRoomResponse, error) {
	c.calls++
	c.verify(ctx, req, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, req.OperationId)
	return &callsv1.JoinMatchSquadRoomResponse{}, nil
}
func (c *publicMatchSquadMemberClient) GetMatchSquadJoinToken(ctx context.Context, req *callsv1.GetMatchSquadJoinTokenRequest, _ ...grpc.CallOption) (*callsv1.GetMatchSquadJoinTokenResponse, error) {
	c.calls++
	c.verify(ctx, req, callsv1.MatchSquadMemberService_GetMatchSquadJoinToken_FullMethodName, "")
	return &callsv1.GetMatchSquadJoinTokenResponse{}, nil
}
func (c *publicMatchSquadMemberClient) LeaveMatchSquadRoom(ctx context.Context, req *callsv1.LeaveMatchSquadRoomRequest, _ ...grpc.CallOption) (*callsv1.LeaveMatchSquadRoomResponse, error) {
	c.calls++
	c.verify(ctx, req, callsv1.MatchSquadMemberService_LeaveMatchSquadRoom_FullMethodName, req.OperationId)
	return &callsv1.LeaveMatchSquadRoomResponse{}, nil
}

func TestMatchSquadMemberRoutesDelegateExactVerifiedActorAndRequest(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	matchID, roomID, opID, epochID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	accountID, profileID := uuid.NewString(), uuid.NewString()
	client := &publicMatchSquadMemberClient{}
	client.verify = func(ctx context.Context, req any, method, expectedID string) {
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		require.Empty(t, md.Get("x-voice-user-id"))
		require.Empty(t, md.Get("x-voice-profile-id"))
		require.Empty(t, md.Get("x-voice-session-epoch"))
		require.Len(t, md.Get("authorization"), 1)
		require.True(t, strings.HasPrefix(md.Get("authorization")[0], "Bearer "))
		requestID := md.Get("x-request-id")[0]
		if expectedID == "" {
			require.True(t, canonicalLifecycleUUID(requestID))
		} else {
			require.Equal(t, expectedID, requestID)
		}
		var hash string
		switch q := req.(type) {
		case *callsv1.JoinMatchSquadRoomRequest:
			require.Equal(t, uint32(1), q.ProtocolVersion)
			require.Equal(t, matchID, q.MatchId)
			require.Equal(t, roomID, q.RoomId)
			hash, err = principal.RequestHash(q)
		case *callsv1.GetMatchSquadJoinTokenRequest:
			require.Equal(t, uint32(1), q.ProtocolVersion)
			require.Equal(t, matchID, q.MatchId)
			require.Equal(t, roomID, q.RoomId)
			require.Equal(t, epochID, q.MediaEpoch)
			hash, err = principal.RequestHash(q)
		case *callsv1.LeaveMatchSquadRoomRequest:
			require.Equal(t, uint32(1), q.ProtocolVersion)
			require.Equal(t, matchID, q.MatchId)
			require.Equal(t, roomID, q.RoomId)
			require.Equal(t, epochID, q.ExpectedMediaEpoch)
			hash, err = principal.RequestHash(q)
		default:
			t.Fatalf("unexpected request type %T", req)
		}
		require.NoError(t, err)
		principalValue, err := principal.VerifyDelegatedUser(context.Background(), strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{
			ExpectedIssuer: "gateway", ExpectedAudience: "voice", ExpectedRPC: method,
			ExpectedRequestID: requestID, ExpectedRequestHash: hash,
			KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil },
			ReplayGuard: func(context.Context, string, string, time.Time) error { return nil },
			SessionEpochChecker: func(_ context.Context, id string, epoch int64) error {
				require.Equal(t, accountID, id)
				require.Equal(t, int64(4), epoch)
				return nil
			},
		})
		require.NoError(t, err)
		require.Equal(t, accountID, principalValue.AccountID)
		require.Equal(t, profileID, principalValue.ProfileID)
	}

	h := newGateway(gatewayConfig{
		tokenClaims: map[string]tokenClaims{"user": {UserID: accountID, ProfileID: profileID, SessionEpoch: 4, AccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)}},
		transcoder:  &transcoder{clients: grpcClients{matchSquadMember: client}, matchSquadMemberIssuer: issuer},
	})
	for _, tc := range []struct{ suffix, body string }{
		{"join", `{"protocol_version":1,"operation_id":"` + opID + `","room_id":"` + roomID + `"}`},
		{"token", `{"protocol_version":1,"room_id":"` + roomID + `","media_epoch":"` + epochID + `"}`},
		{"leave", `{"protocol_version":1,"operation_id":"` + opID + `","room_id":"` + roomID + `","expected_media_epoch":"` + epochID + `"}`},
	} {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/matchmaking/matches/"+matchID+"/voice/"+tc.suffix, strings.NewReader(tc.body))
		r.Header.Set("Authorization", "Bearer user")
		r.Header.Set("X-Voice-User-Id", uuid.NewString())
		r.Header.Set("X-Voice-Profile-Id", uuid.NewString())
		r.Header.Set("X-Voice-Session-Epoch", "99")
		r.Header.Set("X-Voice-Account-Type", "regular")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}
	require.Equal(t, 3, client.calls)
}

func TestMatchSquadMemberRoutesRejectInvalidBindingBeforeUpstream(t *testing.T) {
	matchID, otherMatchID, roomID, opID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	client := &publicMatchSquadMemberClient{}
	h := newGateway(gatewayConfig{
		tokenClaims: map[string]tokenClaims{"user": {UserID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 1, AccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)}},
		transcoder:  &transcoder{clients: grpcClients{matchSquadMember: client}},
	})
	for _, tc := range []struct{ path, body string }{
		{matchID, `{"protocol_version":1,"operation_id":"` + opID + `","match_id":"` + otherMatchID + `","room_id":"` + roomID + `"}`},
		{matchID, `{"protocol_version":2,"operation_id":"` + opID + `","room_id":"` + roomID + `"}`},
		{"not-a-uuid", `{"protocol_version":1,"operation_id":"` + opID + `","room_id":"` + roomID + `"}`},
	} {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/matchmaking/matches/"+tc.path+"/voice/join", strings.NewReader(tc.body))
		r.Header.Set("Authorization", "Bearer user")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		require.Equal(t, http.StatusBadRequest, w.Code)
	}
	require.Zero(t, client.calls)
}
