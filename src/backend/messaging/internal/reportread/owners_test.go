package reportread

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	socialv1 "voice.app/voice/social/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type spaceCall func(context.Context, *spacev1.ResolveSpaceReportReadContextRequest) (*spacev1.ResolveSpaceReportReadContextResponse, error)

func (f spaceCall) ResolveSpaceReportReadContext(ctx context.Context, r *spacev1.ResolveSpaceReportReadContextRequest, _ ...grpc.CallOption) (*spacev1.ResolveSpaceReportReadContextResponse, error) {
	return f(ctx, r)
}

type roleCall func(context.Context, *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error)

func (f roleCall) CheckPermission(ctx context.Context, r *rolev1.CheckPermissionRequest, _ ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	return f(ctx, r)
}

type socialCall func(context.Context, *socialv1.IsProfilePairsBlockedRequest) (*socialv1.IsProfilePairsBlockedResponse, error)

func (f socialCall) IsProfilePairsBlocked(ctx context.Context, r *socialv1.IsProfilePairsBlockedRequest, _ ...grpc.CallOption) (*socialv1.IsProfilePairsBlockedResponse, error) {
	return f(ctx, r)
}

func ownerFixture(t *testing.T) (*Owners, func(context.Context, proto.Message, string, string) principal.Principal) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	i, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	seen := map[string]bool{}
	verify := func(ctx context.Context, request proto.Message, audience, method string) principal.Principal {
		t.Helper()
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		require.Empty(t, md.Get("x-voice-profile-id"))
		require.Empty(t, md.Get("x-voice-user-id"))
		require.Empty(t, md.Get("x-untrusted"))
		require.Len(t, md.Get("authorization"), 1)
		require.Len(t, md.Get("x-request-id"), 1)
		hash, err := principal.RequestHash(request)
		require.NoError(t, err)
		p, err := principal.VerifyService(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{
			ExpectedIssuer: "messaging", ExpectedAudience: audience, ExpectedRPC: method, ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash,
			KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil },
			ReplayGuard: func(_ context.Context, issuer, jti string, _ time.Time) error {
				require.False(t, seen[issuer+jti])
				seen[issuer+jti] = true
				return nil
			},
		})
		require.NoError(t, err)
		require.True(t, canonicalID(p.RequestID))
		require.Equal(t, "service:messaging", p.Subject)
		return p
	}
	return &Owners{Issuer: i}, verify
}
func reportChat() *chatv1.ChatRef {
	c := chatv1.ChatType_CHAT_TYPE_CHANNEL
	return &chatv1.ChatRef{Id: uuid.NewString(), Type: &c}
}
func positiveSpace(r *spacev1.ResolveSpaceReportReadContextRequest, now time.Time) *spacev1.ResolveSpaceReportReadContextResponse {
	return &spacev1.ResolveSpaceReportReadContextResponse{ProtocolVersion: 1, OperationId: r.OperationId, SpaceId: r.SpaceId, ViewerProfileId: r.ViewerProfileId, Chat: proto.Clone(r.Chat).(*chatv1.ChatRef), Eligible: true,
		SpaceAuthorityRevision: 1, LifecycleGeneration: 1, TreeAttachmentIncarnation: uuid.NewString(), MembershipBinding: &rolev1.MembershipBinding{Source: &rolev1.MembershipBinding_Ordinary{Ordinary: &rolev1.OrdinaryMembershipBinding{Legacy: true, PublicationIdentity: uuid.NewString()}}},
		EffectiveEpisodeId: uuid.NewString(), EffectiveEpisodeGeneration: 1, LedgerRevision: 1, EvaluatedAt: timestamppb.New(now), ValidityRemainingMs: 1000}
}
func taintedContext() context.Context {
	c := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer inbound", "x-voice-profile-id", uuid.NewString()))
	return metadata.NewOutgoingContext(c, metadata.Pairs("authorization", "Bearer old-outbound", "x-untrusted", "payload"))
}

func TestReportOwnerSpaceExactEchoUnknownBoundaryAndTransit(t *testing.T) {
	for _, scenario := range []string{"positive UNKNOWN", "positive KNOWN", "echo", "nested unknown", "binding", "history", "lease", "transit", "denied", "denied leak", "error", "space revision overflow", "lifecycle overflow", "episode overflow", "ledger overflow"} {
		t.Run(scenario, func(t *testing.T) {
			o, verify := ownerFixture(t)
			now := time.Now()
			o.Clock = func() time.Time { return now }
			o.Space = spaceCall(func(ctx context.Context, req *spacev1.ResolveSpaceReportReadContextRequest) (*spacev1.ResolveSpaceReportReadContextResponse, error) {
				verify(ctx, req, "space", spacev1.SpaceService_ResolveSpaceReportReadContext_FullMethodName)
				r := positiveSpace(req, now)
				switch scenario {
				case "positive KNOWN":
					r.HistoryKnown = true
					r.HistoryVisibleSince = timestamppb.New(now.Add(-time.Hour))
				case "echo":
					r.ViewerProfileId = uuid.NewString()
				case "nested unknown":
					r.MembershipBinding.GetOrdinary().ProtoReflect().SetUnknown([]byte{0x78, 1})
				case "binding":
					r.MembershipBinding.GetOrdinary().Legacy = false
				case "history":
					r.HistoryVisibleSince = timestamppb.New(now)
				case "lease":
					r.ValidityRemainingMs = 5001
				case "space revision overflow":
					r.SpaceAuthorityRevision = ^uint64(0)
				case "lifecycle overflow":
					r.LifecycleGeneration = ^uint64(0)
				case "episode overflow":
					r.EffectiveEpisodeGeneration = ^uint64(0)
				case "ledger overflow":
					r.LedgerRevision = ^uint64(0)
				case "transit":
					now = now.Add(time.Second)
				case "denied", "denied leak":
					r = &spacev1.ResolveSpaceReportReadContextResponse{ProtocolVersion: 1, OperationId: req.OperationId, SpaceId: req.SpaceId, ViewerProfileId: req.ViewerProfileId, Chat: req.Chat}
					if scenario == "denied leak" {
						r.TreeAttachmentIncarnation = uuid.NewString()
					}
				case "error":
					return nil, status.Error(codes.NotFound, "private dependency detail")
				}
				return r, nil
			})
			before := now
			got, err := o.ReadSpace(taintedContext(), uuid.New(), uuid.New(), reportChat())
			if strings.HasPrefix(scenario, "positive") {
				require.NoError(t, err)
				require.True(t, got.Facts.Eligible)
				require.Equal(t, before.Add(time.Second), got.Deadline)
			} else if scenario == "denied" {
				require.NoError(t, err)
				require.False(t, got.Facts.Eligible)
				require.True(t, got.Deadline.IsZero())
			} else {
				require.Equal(t, codes.Unavailable, status.Code(err))
				require.Nil(t, got)
				require.NotContains(t, err.Error(), "private dependency")
			}
		})
	}
}

func TestReportOwnerRoleExactScopedBitsAndFreshCredentials(t *testing.T) {
	o, verify := ownerFixture(t)
	chat := reportChat()
	space, viewer := uuid.New(), uuid.New()
	calls := 0
	o.Role = roleCall(func(ctx context.Context, req *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		verify(ctx, req, "role", rolev1.RoleService_CheckPermission_FullMethodName)
		calls++
		require.Equal(t, space.String(), req.SpaceId)
		require.Equal(t, viewer.String(), req.ProfileId)
		require.True(t, proto.Equal(chat, req.Chat))
		require.Nil(t, req.VoiceRoomId)
		return &rolev1.CheckPermissionResponse{Allowed: req.PermissionName == "TEXT_CHAT_VIEW"}, nil
	})
	view, err := o.Permission(taintedContext(), space, viewer, chat, "TEXT_CHAT_VIEW")
	require.NoError(t, err)
	require.True(t, view)
	history, err := o.Permission(taintedContext(), space, viewer, chat, "TEXT_CHAT_READ_HISTORY")
	require.NoError(t, err)
	require.False(t, history)
	_, err = o.Permission(taintedContext(), space, viewer, chat, "MANAGE_REPORTS")
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Equal(t, 2, calls)
	o.Role = roleCall(func(context.Context, *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		r := &rolev1.CheckPermissionResponse{Allowed: true}
		r.ProtoReflect().SetUnknown([]byte{0x78, 1})
		return r, nil
	})
	_, err = o.Permission(taintedContext(), space, viewer, chat, "TEXT_CHAT_VIEW")
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestReportOwnerSocialExactFiniteDirectionalBatch(t *testing.T) {
	for _, scenario := range []string{"complete", "duplicate", "missing", "foreign", "viewer", "unknown"} {
		t.Run(scenario, func(t *testing.T) {
			o, verify := ownerFixture(t)
			viewer, a, b := uuid.New(), uuid.New(), uuid.New()
			o.Social = socialCall(func(ctx context.Context, req *socialv1.IsProfilePairsBlockedRequest) (*socialv1.IsProfilePairsBlockedResponse, error) {
				verify(ctx, req, "social", socialv1.SocialService_IsProfilePairsBlocked_FullMethodName)
				require.Equal(t, viewer.String(), req.ViewerProfileId)
				r := &socialv1.IsProfilePairsBlockedResponse{ProtocolVersion: 1, OperationId: req.OperationId, ViewerProfileId: req.ViewerProfileId, Results: []*socialv1.ProfilePairBlockResult{{OtherProfileId: b.String(), Blocked: true}, {OtherProfileId: a.String()}}}
				switch scenario {
				case "duplicate":
					r.Results[1].OtherProfileId = b.String()
				case "missing":
					r.Results = r.Results[:1]
				case "foreign":
					r.Results[1].OtherProfileId = uuid.NewString()
				case "viewer":
					r.ViewerProfileId = uuid.NewString()
				case "unknown":
					r.Results[0].ProtoReflect().SetUnknown([]byte{0x78, 1})
				}
				return r, nil
			})
			got, err := o.Blocked(taintedContext(), viewer, []uuid.UUID{a, b})
			if scenario == "complete" {
				require.NoError(t, err)
				require.Equal(t, map[uuid.UUID]bool{a: false, b: true}, got)
			} else {
				require.Equal(t, codes.Unavailable, status.Code(err))
				require.Nil(t, got)
			}
		})
	}
}
