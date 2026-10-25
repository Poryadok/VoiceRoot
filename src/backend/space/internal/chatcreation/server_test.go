package chatcreation

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

var errMissing = errors.New("missing")

func TestGatewayScopeDenialHidesPrivateExpiredOutcomeBeforeStoreRead(t *testing.T) {
	b := bindingFixture()
	st := &fakeOutcomeStore{readErr: errors.New("private result expired")}
	s := &Server{Store: st, Admission: &fakeAdmission{scopeErr: status.Error(codes.NotFound, "Space not found")}}
	ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "delegated_user", Issuer: "gateway", Audience: "space", RPC: spacev1.SpaceService_GetSpaceChatCreationOutcome_FullMethodName, AccountID: b.GetUser().AccountId, ProfileID: b.CreatorProfileId, AccountType: "regular", SessionEpoch: 7})
	out, err := s.GetSpaceChatCreationOutcome(ctx, &spacev1.GetSpaceChatCreationOutcomeRequest{SpaceId: b.SpaceId, OperationId: b.OperationId})
	require.Nil(t, out)
	require.Equal(t, codes.NotFound, status.Code(err))
	require.Zero(t, st.reads, "public denial precedes private result/expiry disclosure")
}

func TestChatPublicReplayChecksCurrentScopeBeforePrivateResult(t *testing.T) {
	b := bindingFixture()
	st := &fakeOutcomeStore{saved: &spacev1.SpaceChatCreationOutcome{Binding: b, State: spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_ABORTED, FailureCode: uint32(codes.PermissionDenied)}}
	a := &fakeAdmission{scopeErr: status.Error(codes.NotFound, "membership or guest access removed")}
	s := &Server{Store: st, Admission: a}
	ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "space", RPC: spacev1.SpaceService_GetSpaceChatCreationOutcome_FullMethodName})
	req := &spacev1.GetSpaceChatCreationOutcomeRequest{SpaceId: b.SpaceId, OperationId: b.OperationId, PublicActor: &spacev1.SpaceChatCreationPublicActor{ProfileId: b.CreatorProfileId, Initiator: &spacev1.SpaceChatCreationPublicActor_User{User: proto.Clone(b.GetUser()).(*spacev1.SpaceChatCreationUser)}}}
	out, err := s.GetSpaceChatCreationOutcome(ctx, req)
	require.Nil(t, out)
	require.Equal(t, codes.NotFound, status.Code(err))
	require.Zero(t, st.reads, "current public denial must precede receipt/expiry lookup")
	// The identical operation still supports private recovery after access loss.
	req.PublicActor = nil
	out, err = s.GetSpaceChatCreationOutcome(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Equal(t, 1, st.reads)
}

type fakeOutcomeStore struct {
	saved    *spacev1.SpaceChatCreationOutcome
	readErr  error
	commits  int
	decision codes.Code
	reads    int
}

func (s *fakeOutcomeStore) GetChatCreationOutcome(context.Context, uuid.UUID, uuid.UUID) (*spacev1.SpaceChatCreationOutcome, error) {
	s.reads++
	return s.saved, s.readErr
}
func (s *fakeOutcomeStore) CommitChatCreation(_ context.Context, b *spacev1.SpaceChatCreationBinding, code codes.Code) (*spacev1.SpaceChatCreationOutcome, error) {
	s.commits++
	s.decision = code
	return &spacev1.SpaceChatCreationOutcome{Binding: b, State: spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_ABORTED, FailureCode: uint32(code)}, nil
}

type fakeAdmission struct {
	err         error
	calls       int
	disclosures int
	scopeErr    error
	scopes      int
}

func (a *fakeAdmission) CheckOutcomeReadScope(context.Context, principal.Principal, uuid.UUID) error {
	return a.scopeErr
}

func (a *fakeAdmission) CheckPublicOutcomeReadScope(context.Context, *spacev1.SpaceChatCreationPublicActor, uuid.UUID) error {
	a.scopes++
	return a.scopeErr
}

func (a *fakeAdmission) CheckPublicOutcomeDisclosure(context.Context, *spacev1.SpaceChatCreationPublicActor, *spacev1.SpaceChatCreationBinding) error {
	a.disclosures++
	return a.err
}

func TestChatPublicReplayRechecksEveryCallAndRejectsGatewayOverride(t *testing.T) {
	b := bindingFixture()
	st := &fakeOutcomeStore{saved: &spacev1.SpaceChatCreationOutcome{Binding: b, State: spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_COMMITTED, Node: &spacev1.SpaceTreeNode{Id: b.NodeId, SpaceId: b.SpaceId, Kind: "text_chat", LinkedChat: &chatv1.ChatRef{Id: b.ChatId}}}}
	a := &fakeAdmission{}
	s := &Server{Store: st, Admission: a}
	ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "space", RPC: spacev1.SpaceService_GetSpaceChatCreationOutcome_FullMethodName})
	req := &spacev1.GetSpaceChatCreationOutcomeRequest{SpaceId: b.SpaceId, OperationId: b.OperationId, PublicActor: &spacev1.SpaceChatCreationPublicActor{ProfileId: b.CreatorProfileId, Initiator: &spacev1.SpaceChatCreationPublicActor_User{User: proto.Clone(b.GetUser()).(*spacev1.SpaceChatCreationUser)}}}
	_, err := s.GetSpaceChatCreationOutcome(ctx, req)
	require.NoError(t, err)
	a.scopeErr = status.Error(codes.NotFound, "membership removed")
	_, err = s.GetSpaceChatCreationOutcome(ctx, req)
	require.Equal(t, codes.NotFound, status.Code(err))
	require.Equal(t, 2, a.scopes)
	require.Equal(t, 1, st.reads)
	gateway := principal.WithVerified(context.Background(), principal.Principal{Kind: "delegated_user", Issuer: "gateway", Audience: "space", RPC: spacev1.SpaceService_GetSpaceChatCreationOutcome_FullMethodName})
	_, err = s.GetSpaceChatCreationOutcome(gateway, req)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Equal(t, 2, a.scopes)
	req.PublicActor.GetUser().AccountType = ""
	_, err = s.GetSpaceChatCreationOutcome(ctx, req)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Equal(t, 1, st.reads)
}

func (a *fakeAdmission) CheckChatCreation(context.Context, *spacev1.SpaceChatCreationBinding) error {
	a.calls++
	return a.err
}
func (a *fakeAdmission) CheckOutcomeDisclosure(context.Context, principal.Principal, *spacev1.SpaceChatCreationBinding) error {
	a.disclosures++
	return a.err
}
func chatContext() context.Context {
	return principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "space", RPC: spacev1.SpaceService_CommitSpaceChatCreation_FullMethodName})
}

func TestServerReplayDoesNotRequireCurrentAdmissionOrRewriteOutcome(t *testing.T) {
	b := bindingFixture()
	saved := &spacev1.SpaceChatCreationOutcome{Binding: b, State: spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_ABORTED, FailureCode: uint32(codes.PermissionDenied)}
	st := &fakeOutcomeStore{saved: saved}
	s := &Server{Store: st, UnknownError: errMissing}
	response, err := s.CommitSpaceChatCreation(chatContext(), &spacev1.CommitSpaceChatCreationRequest{Binding: b})
	require.NoError(t, err)
	require.True(t, proto.Equal(saved, response.Outcome))
	require.Zero(t, st.commits)
	changed := proto.Clone(b).(*spacev1.SpaceChatCreationBinding)
	changed.GetUser().AccountType = "guest"
	_, err = s.CommitSpaceChatCreation(chatContext(), &spacev1.CommitSpaceChatCreationRequest{Binding: changed})
	require.Equal(t, codes.AlreadyExists, status.Code(err))
	require.Zero(t, st.commits)
}

func TestServerUnknownAuthorityCannotBecomeTerminalAbort(t *testing.T) {
	for _, code := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded, codes.Unauthenticated, codes.Internal} {
		t.Run(code.String(), func(t *testing.T) {
			st := &fakeOutcomeStore{readErr: errMissing}
			admission := &fakeAdmission{err: status.Error(code, "dependency")}
			s := &Server{Store: st, Admission: admission, UnknownError: errMissing}
			_, err := s.CommitSpaceChatCreation(chatContext(), &spacev1.CommitSpaceChatCreationRequest{Binding: bindingFixture()})
			require.Equal(t, code, status.Code(err))
			require.Equal(t, 1, admission.calls)
			require.Zero(t, st.commits)
		})
	}
}

func TestServerDeterministicDomainRefusalIsPersisted(t *testing.T) {
	st := &fakeOutcomeStore{readErr: errMissing}
	admission := &fakeAdmission{err: status.Error(codes.PermissionDenied, "permission")}
	s := &Server{Store: st, Admission: admission, UnknownError: errMissing}
	out, err := s.CommitSpaceChatCreation(chatContext(), &spacev1.CommitSpaceChatCreationRequest{Binding: bindingFixture()})
	require.NoError(t, err)
	require.Equal(t, 1, st.commits)
	require.Equal(t, codes.PermissionDenied, st.decision)
	require.Equal(t, spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_ABORTED, out.Outcome.State)
}

func TestServerNeverAcceptsRawUnverifiedOrWrongServiceContext(t *testing.T) {
	st := &fakeOutcomeStore{readErr: errMissing}
	s := &Server{Store: st, UnknownError: errMissing}
	for _, ctx := range []context.Context{context.Background(), principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "gateway", Subject: "service:gateway", Audience: "space", RPC: spacev1.SpaceService_CommitSpaceChatCreation_FullMethodName}), principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "role"})} {
		_, err := s.CommitSpaceChatCreation(ctx, &spacev1.CommitSpaceChatCreationRequest{Binding: bindingFixture()})
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		require.Zero(t, st.commits)
	}
}

func TestServerGatewayOutcomeAlwaysRunsDisclosureButChatRecoveryDoesNot(t *testing.T) {
	b := bindingFixture()
	st := &fakeOutcomeStore{saved: &spacev1.SpaceChatCreationOutcome{Binding: b, State: spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_ABORTED, FailureCode: uint32(codes.PermissionDenied)}}
	a := &fakeAdmission{err: status.Error(codes.NotFound, "not found")}
	s := &Server{Store: st, Admission: a}
	req := &spacev1.GetSpaceChatCreationOutcomeRequest{SpaceId: b.SpaceId, OperationId: b.OperationId}
	_, err := s.GetSpaceChatCreationOutcome(principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "space", RPC: spacev1.SpaceService_GetSpaceChatCreationOutcome_FullMethodName}), req)
	require.NoError(t, err)
	require.Zero(t, a.disclosures)
	ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "delegated_user", Issuer: "gateway", Audience: "space", AccountID: b.GetUser().AccountId, ProfileID: b.CreatorProfileId, SessionEpoch: 7, RPC: spacev1.SpaceService_GetSpaceChatCreationOutcome_FullMethodName})
	out, err := s.GetSpaceChatCreationOutcome(ctx, req)
	require.Nil(t, out)
	require.Equal(t, codes.NotFound, status.Code(err))
	require.Equal(t, 1, a.disclosures)
}
