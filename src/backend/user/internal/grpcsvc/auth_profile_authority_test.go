package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/authprofileauthority"
	"voice/backend/pkg/principal"
	"voice/backend/user/internal/identityassociation"
)

type authProfileReadFunc func(context.Context, uuid.UUID, []uuid.UUID) (identityassociation.AuthProfileBatch, error)

func (f authProfileReadFunc) ReadAuthProfiles(c context.Context, a uuid.UUID, ids []uuid.UUID) (identityassociation.AuthProfileBatch, error) {
	return f(c, a, ids)
}
func authProfileRequest() *userv1.MatchAuthProfileReferencesRequest {
	return &userv1.MatchAuthProfileReferencesRequest{ProtocolVersion: 1, RequestId: uuid.NewString(), InventoryId: uuid.NewString(), InventoryRevision: 1, AccountId: uuid.NewString(), OwnerHandle: uuid.NewString(), DeletionCycleId: uuid.NewString(), ProfileIds: []string{uuid.NewString()}}
}
func authProfileContext(t *testing.T, r proto.Message, id, method string) context.Context {
	t.Helper()
	hash, e := principal.RequestHash(r)
	require.NoError(t, e)
	return principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "auth", Subject: "service:auth", Audience: "user", RPC: method, RequestID: id, RequestHash: hash})
}
func authProfileBatch(ids []uuid.UUID) identityassociation.AuthProfileBatch {
	return identityassociation.AuthProfileBatch{ObservedAt: time.Now(), Cut: identityassociation.SourceCut{UserSourceGeneration: uuid.New(), HistoryGeneration: uuid.New(), KeysetRevision: 1, UserAuthorityRevision: 1, ThroughRevision: 1, Digest: [32]byte{1}}, Results: []identityassociation.AuthProfileResult{{ProfileID: ids[0], Match: "MATCH", Fact: identityassociation.Fact{AssociationID: uuid.New(), Revision: 1, Origin: identityassociation.OriginRetainedNow}}}}
}

func TestAuthProfileAdapterBindsExactRosterAndSuppressesForeignAccount(t *testing.T) {
	r := authProfileRequest()
	ctx := authProfileContext(t, r, r.RequestId, userv1.AuthProfileAuthorityService_MatchAuthProfileReferences_FullMethodName)
	calls := 0
	s := &AuthProfileAuthorityGRPC{Reader: authProfileReadFunc(func(c context.Context, a uuid.UUID, ids []uuid.UUID) (identityassociation.AuthProfileBatch, error) {
		calls++
		require.Equal(t, r.AccountId, a.String())
		require.Equal(t, r.ProfileIds[0], ids[0].String())
		out := authProfileBatch(ids)
		out.Results[0].CurrentAccount = uuid.New()
		return out, nil
	})}
	out, err := s.MatchAuthProfileReferences(ctx, r)
	require.NoError(t, err)
	require.NoError(t, authprofileauthority.ValidateMatchResponse(r, out, time.Now()))
	require.True(t, out.AllRequestedResolved)
	changed := proto.Clone(r).(*userv1.MatchAuthProfileReferencesRequest)
	changed.InventoryRevision++
	_, err = s.MatchAuthProfileReferences(ctx, changed)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, 1, calls)
	_, err = s.MatchAuthProfileReferences(context.Background(), r)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Equal(t, 1, calls)
	r.ExpectedSource = &userv1.AuthProfileSourcePin{UserSourceGeneration: uuid.NewString(), HistoryGeneration: uuid.NewString(), KeysetRevision: 1}
	ctx = authProfileContext(t, r, r.RequestId, userv1.AuthProfileAuthorityService_MatchAuthProfileReferences_FullMethodName)
	_, err = s.MatchAuthProfileReferences(ctx, r)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestAuthProfileTargetOmitsRawTupleForIncompleteAndTerminal(t *testing.T) {
	r := &userv1.ResolveAuthOwnershipTargetRequest{ProtocolVersion: 1, RequestId: uuid.NewString(), OperationId: uuid.NewString(), ProfileId: uuid.NewString()}
	ctx := authProfileContext(t, r, r.RequestId, userv1.AuthProfileAuthorityService_ResolveAuthOwnershipTarget_FullMethodName)
	for _, mode := range []string{"current", "terminal", "incomplete"} {
		t.Run(mode, func(t *testing.T) {
			s := &AuthProfileAuthorityGRPC{Reader: authProfileReadFunc(func(c context.Context, a uuid.UUID, ids []uuid.UUID) (identityassociation.AuthProfileBatch, error) {
				require.Equal(t, uuid.Nil, a)
				b := authProfileBatch(ids)
				v := &b.Results[0]
				v.CurrentAccount = uuid.New()
				v.CurrentRevision = 2
				v.Frozen = true
				if mode == "terminal" {
					v.Fact.Terminal = &identityassociation.TerminalEvidence{OperationID: uuid.New(), Revision: 1, ReceiptID: uuid.New(), ReceiptDigest: [32]byte{1}}
				}
				if mode == "incomplete" {
					v.Fact = identityassociation.Fact{}
				}
				return b, nil
			})}
			out, err := s.ResolveAuthOwnershipTarget(ctx, r)
			require.NoError(t, err)
			require.NoError(t, authprofileauthority.ValidateTargetResponse(r, out, time.Now()))
			if mode == "current" {
				require.NotEmpty(t, out.AccountId)
				require.True(t, out.Frozen)
			} else {
				require.Empty(t, out.AccountId)
				require.Zero(t, out.ProfileRevision)
				require.False(t, out.Frozen)
			}
		})
	}
}
