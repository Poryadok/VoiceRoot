package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/chatcreation"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/principal"
)

type compositeRawStore struct {
	DMStore
	writes int
}

type compositeCoordinatorStore struct {
	CompositeStore
	op       *store.SpaceCompositeOperation
	resolved int
	claimed  bool
	retries  int
}

func (s *compositeCoordinatorStore) ClaimSpaceComposite(context.Context, time.Duration) (*store.SpaceCompositeClaim, error) {
	if s.claimed {
		return nil, nil
	}
	s.claimed = true
	return &store.SpaceCompositeClaim{Operation: s.op, Token: uuid.New()}, nil
}
func (s *compositeCoordinatorStore) ResolveClaimedSpaceComposite(ctx context.Context, c *store.SpaceCompositeClaim, out *spacev1.SpaceChatCreationOutcome) (*store.SpaceCompositeOperation, error) {
	return s.ResolveSpaceComposite(ctx, uuid.MustParse(c.Operation.Binding.OperationId), out)
}
func (s *compositeCoordinatorStore) RetrySpaceComposite(context.Context, *store.SpaceCompositeClaim, time.Duration) error {
	s.retries++
	return nil
}

func (s *compositeCoordinatorStore) PrepareSpaceComposite(context.Context, *chatv1.CreateChatRequest) (*store.SpaceCompositeOperation, error) {
	return s.op, nil
}
func (s *compositeCoordinatorStore) ResolveSpaceComposite(_ context.Context, _ uuid.UUID, out *spacev1.SpaceChatCreationOutcome) (*store.SpaceCompositeOperation, error) {
	s.resolved++
	if out == nil {
		return nil, store.ErrSpaceCompositeEvidence
	}
	s.op.State = "COMPLETE"
	return s.op, nil
}

type compositeRemote struct {
	outcome *spacev1.SpaceChatCreationOutcome
	err     error
}

func (r *compositeRemote) Commit(context.Context, *spacev1.SpaceChatCreationBinding) (*spacev1.SpaceChatCreationOutcome, error) {
	return r.outcome, r.err
}

type compositePublicScope struct {
	err   error
	calls int
}

func (p *compositePublicScope) Check(context.Context, chatcreation.Initiator, *store.SpaceCompositeOperation) error {
	p.calls++
	return p.err
}

func TestCompositeCoordinatorUnknownNeverAbortsAndPublicDenialDoesNotUndoCommit(t *testing.T) {
	req := &chatv1.CreateChatRequest{Type: chatv1.ChatType_CHAT_TYPE_GROUP, SpaceId: proto.String(uuid.NewString()), Name: proto.String("created"), RequestId: uuid.NewString()}
	actor := uuid.NewString()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "delegated_user", Issuer: "gateway", Subject: actor, AccountID: actor, ProfileID: actor, Audience: "chat", RPC: chatv1.ChatService_CreateChat_FullMethodName, RequestID: req.RequestId, RequestHash: hash, SessionEpoch: 1, AccountType: "regular", ExpiresAt: time.Now().Add(30 * time.Second)})
	db := &compositeCoordinatorStore{op: &store.SpaceCompositeOperation{State: "PREPARED", Binding: &spacev1.SpaceChatCreationBinding{OperationId: uuid.NewString()}, Response: &chatv1.CreateChatResponse{}}}
	remote := &compositeRemote{err: status.Error(codes.Unavailable, "lost reply")}
	public := &compositePublicScope{err: status.Error(codes.PermissionDenied, "permission removed")}
	c := &SpaceCompositeCoordinator{Store: db, Remote: remote, Public: public}
	_, err = c.Create(ctx, req)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Zero(t, db.resolved, "unavailable is not a verified abort")
	require.Zero(t, public.calls, "no prepared result is disclosed")
	remote.err = nil
	remote.outcome = &spacev1.SpaceChatCreationOutcome{State: spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_COMMITTED}
	_, err = c.Create(ctx, req)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Equal(t, 1, db.resolved, "durable committed fact rolls forward before public disclosure decision")
	require.Equal(t, "COMPLETE", db.op.State)
}

func TestCompositeRecoveryIgnoresOriginalSessionAndPublicDisclosure(t *testing.T) {
	db := &compositeCoordinatorStore{op: &store.SpaceCompositeOperation{State: "PREPARED", Binding: &spacev1.SpaceChatCreationBinding{OperationId: uuid.NewString()}, Response: &chatv1.CreateChatResponse{}}}
	remote := &compositeRemote{err: status.Error(codes.Unavailable, "outcome unknown")}
	public := &compositePublicScope{err: status.Error(codes.PermissionDenied, "original account no longer admitted")}
	c := &SpaceCompositeCoordinator{Store: db, Remote: remote, Public: public}
	worked, err := c.RecoverOne(context.Background())
	require.True(t, worked)
	require.Error(t, err)
	require.Zero(t, db.resolved)
	require.Equal(t, 1, db.retries)
	require.Equal(t, "PREPARED", db.op.State)
	db.claimed = false
	remote.err = nil
	remote.outcome = &spacev1.SpaceChatCreationOutcome{State: spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_COMMITTED}
	worked, err = c.RecoverOne(context.Background())
	require.True(t, worked)
	require.NoError(t, err)
	require.Equal(t, "COMPLETE", db.op.State)
	require.Equal(t, 1, db.resolved)
	require.Zero(t, public.calls, "public ACL/session loss must not gate private committed rollforward")
}

func (s *compositeRawStore) CreateChatWithRequestID(_ context.Context, r store.ChatCreateRequest) (*store.ChatRow, bool, error) {
	s.writes++
	now := time.Now()
	return &store.ChatRow{ID: uuid.New(), Type: r.Type, SpaceID: r.SpaceID, CreatorProfileID: r.CreatorProfileID, Name: &r.Name, CreatedAt: now, UpdatedAt: now}, false, nil
}

func TestSpaceCompositeCreateRejectsRawInitiatorBeforeWrite(t *testing.T) {
	db := &compositeRawStore{}
	server := &ChatGRPC{DM: db}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderProfileID, uuid.NewString()))
	spaceID, name := uuid.NewString(), "untrusted"
	response, err := server.CreateChat(ctx, &chatv1.CreateChatRequest{
		Type: chatv1.ChatType_CHAT_TYPE_GROUP, SpaceId: &spaceID, Name: &name,
		RequestId: uuid.NewString(),
	})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Nil(t, response)
	require.Zero(t, db.writes, "raw profile metadata must never prepare an audit-producing Chat")
}
