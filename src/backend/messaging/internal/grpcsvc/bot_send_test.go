package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"testing"
	"time"
	botv1 "voice.app/voice/bot/v1"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/principal"
)

type botSendOwnerFunc func(context.Context, *botv1.CheckBotMessageSendAuthorityRequest) (*botv1.CheckBotMessageSendAuthorityResponse, error)

func (f botSendOwnerFunc) CheckBotMessageSendAuthority(c context.Context, r *botv1.CheckBotMessageSendAuthorityRequest, _ ...grpc.CallOption) (*botv1.CheckBotMessageSendAuthorityResponse, error) {
	return f(c, r)
}

type botSendSpaceFunc func(context.Context, *spacev1.GetMemberAdmissionAuthorityRequest) (*spacev1.GetMemberAdmissionAuthorityResponse, error)

func (f botSendSpaceFunc) GetMemberAdmissionAuthority(c context.Context, r *spacev1.GetMemberAdmissionAuthorityRequest, _ ...grpc.CallOption) (*spacev1.GetMemberAdmissionAuthorityResponse, error) {
	return f(c, r)
}

type botSendScope struct{ policy *store.ChatThreadPolicy }

func (s botSendScope) Load(context.Context, uuid.UUID) (*store.ChatThreadPolicy, error) {
	return s.policy, nil
}
func botSendBinding() *commonv1.BotMessageSendBinding {
	return &commonv1.BotMessageSendBinding{ProtocolVersion: 1, BotId: uuid.NewString(), ActorProfileId: uuid.NewString(), SpaceId: uuid.NewString(), InstallationId: uuid.NewString(), InstallOperationId: uuid.NewString(), InstallationGeneration: 7, ChatId: uuid.NewString()}
}
func botSendObservation(b *commonv1.BotMessageSendBinding) *spacev1.GetMemberAdmissionAuthorityResponse {
	return &spacev1.GetMemberAdmissionAuthorityResponse{ProtocolVersion: 1, SpaceId: b.SpaceId, ProfileId: b.ActorProfileId, SpaceAuthorityRevision: 9, BotMessageSendBinding: proto.Clone(b).(*commonv1.BotMessageSendBinding), Lifecycle: &spacev1.MemberAdmissionLifecycleAuthority{Generation: 1, State: spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE}, Ownership: &spacev1.MemberAdmissionOwnershipAuthority{Generation: 1, State: spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE}, Ordinary: &spacev1.MemberAdmissionOrdinaryAuthority{HeadGeneration: 3, State: spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE, PublicationKind: spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_BOT_INSTALL, NativePublication: &spacev1.MemberAdmissionNativePublicationProof{Kind: spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_BOT_INSTALL, OperationId: b.InstallOperationId, BindingSha256: make([]byte, 32)}, PublishedBinding: &rolev1.MembershipBinding{Source: &rolev1.MembershipBinding_Ordinary{Ordinary: &rolev1.OrdinaryMembershipBinding{MembershipGeneration: 3, PublicationIdentity: b.InstallOperationId}}}}, TreeRelation: &spacev1.MemberAdmissionTreeRelationAuthority{ChatId: b.ChatId, Present: true, Revision: 1}, ValidityRemaining: durationpb.New(5 * time.Second)}
}
func TestBotSendAuthorityExactIndependentOwners(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	b := botSendBinding()
	space := uuid.MustParse(b.SpaceId)
	observation := botSendObservation(b)
	allowed := true
	ids := map[string]bool{}
	verify := func(ctx context.Context, body proto.Message, audience, rpc string) {
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		require.Len(t, md, 2)
		require.Empty(t, md.Get("x-voice-profile-id"))
		id := md.Get("x-request-id")[0]
		require.False(t, ids[id])
		ids[id] = true
		hash, e := principal.RequestHash(body)
		require.NoError(t, e)
		_, e = principal.VerifyService(ctx, md.Get("authorization")[0][7:], principal.VerifyConfig{ExpectedIssuer: "messaging", ExpectedAudience: audience, ExpectedRPC: rpc, ExpectedRequestID: id, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }})
		require.NoError(t, e)
	}
	a := &BotSendAuthority{Issuer: issuer, Scope: botSendScope{&store.ChatThreadPolicy{SpaceID: &space, ChatType: "group"}}}
	a.Bot = botSendOwnerFunc(func(ctx context.Context, r *botv1.CheckBotMessageSendAuthorityRequest) (*botv1.CheckBotMessageSendAuthorityResponse, error) {
		verify(ctx, r, "bot", botv1.BotService_CheckBotMessageSendAuthority_FullMethodName)
		return &botv1.CheckBotMessageSendAuthorityResponse{Binding: proto.Clone(b).(*commonv1.BotMessageSendBinding), Allowed: allowed}, nil
	})
	a.Space = botSendSpaceFunc(func(ctx context.Context, r *spacev1.GetMemberAdmissionAuthorityRequest) (*spacev1.GetMemberAdmissionAuthorityResponse, error) {
		verify(ctx, r, "space", spacev1.SpaceService_GetMemberAdmissionAuthority_FullMethodName)
		require.Nil(t, r.ExpectedGeneration)
		require.Nil(t, r.ExpectedAdmissionOperationId)
		require.Equal(t, b.ChatId, r.GetChatId())
		return observation, nil
	})
	ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(t.Context(), metadata.Pairs("x-voice-profile-id", "forged")), 5*time.Second)
	defer cancel()
	_, expires, err := a.Check(ctx, b)
	require.NoError(t, err)
	deadline, _ := ctx.Deadline()
	require.False(t, expires.After(deadline))
	for _, name := range []string{"revoked", "reinstall", "foreign_chat", "retired", "frozen", "wrong_native_operation", "missing_native", "wrong_binding", "expired", "unknown"} {
		t.Run(name, func(t *testing.T) {
			observation = botSendObservation(b)
			allowed = true
			switch name {
			case "revoked":
				allowed = false
			case "reinstall":
				observation.BotMessageSendBinding.InstallationGeneration++
			case "foreign_chat":
				observation.TreeRelation.ChatId = uuid.NewString()
			case "retired":
				observation.Ordinary.RetiredThrough = 3
			case "frozen":
				observation.Lifecycle.State = spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_FROZEN
			case "wrong_native_operation":
				observation.Ordinary.NativePublication.OperationId = uuid.NewString()
			case "missing_native":
				observation.Ordinary.NativePublication = nil
			case "wrong_binding":
				observation.ProfileId = uuid.NewString()
			case "expired":
				observation.ValidityRemaining = durationpb.New(0)
			case "unknown":
				observation.Ordinary.ProtoReflect().SetUnknown([]byte{0x80, 0x06, 1})
			}
			_, _, e := a.Check(ctx, b)
			require.Error(t, e)
		})
	}
	_, _, err = a.Check(context.Background(), b)
	require.Error(t, err)
}
