package identityassociation

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
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
	identityv1 "voice.app/voice/identityerasure/v1"
	"voice/backend/pkg/principal"
)

type parentRPCFunc func(context.Context, *identityv1.ResolveAccountHandleRequest, ...grpc.CallOption) (*identityv1.ResolveAccountHandleResponse, error)

func (f parentRPCFunc) ResolveAccountHandle(ctx context.Context, req *identityv1.ResolveAccountHandleRequest, opts ...grpc.CallOption) (*identityv1.ResolveAccountHandleResponse, error) {
	return f(ctx, req, opts...)
}

func parentClientFixture(t *testing.T) (*AuthParentClient, uuid.UUID) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "user", KeyID: "fixture", PrivateKey: key})
	require.NoError(t, err)
	account := uuid.New()
	client := &AuthParentClient{issuer: issuer}
	client.rpc = parentRPCFunc(func(ctx context.Context, req *identityv1.ResolveAccountHandleRequest, _ ...grpc.CallOption) (*identityv1.ResolveAccountHandleResponse, error) {
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		auth := md.Get("authorization")
		require.Len(t, auth, 1)
		require.True(t, strings.HasPrefix(auth[0], "Bearer "))
		require.Equal(t, []string{req.RequestId}, md.Get("x-request-id"))
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		verified, err := principal.VerifyService(ctx, strings.TrimPrefix(auth[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "user", ExpectedAudience: "auth", ExpectedRPC: identityv1.AuthUserIdentityAuthorityService_ResolveAccountHandle_FullMethodName, ExpectedRequestID: req.RequestId, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }})
		require.NoError(t, err)
		require.Equal(t, "service:user", verified.Subject)
		now := time.Now().UTC()
		return &identityv1.ResolveAccountHandleResponse{ProtocolVersion: 1, RequestId: req.RequestId, AccountId: req.AccountId,
			Window: &identityv1.ObservationWindow{ObservedAt: timestamppb.New(now.Add(-time.Second)), ValidUntil: timestamppb.New(now.Add(time.Minute))},
			Lifecycle: &identityv1.AccountLifecycleFact{OwnerHandle: uuid.NewString(), LifecycleRevision: 1, State: identityv1.AccountLifecycleState_ACCOUNT_LIFECYCLE_STATE_ACTIVE,
				Eligibility: identityv1.Eligibility_ELIGIBILITY_ELIGIBLE, Disposition: identityv1.ErasureDisposition_ERASURE_DISPOSITION_NOT_IRREVERSIBLE,
				OwnerCut: &identityv1.OwnerCut{Owner: "auth", StreamId: uuid.NewString(), HistoryGeneration: uuid.NewString(), ThroughRevision: 1, SelectorKeysetRevision: 1, SelectorKeyVersions: []string{"1"}, FactCount: 1, Digest: bytes32(1), Complete: true}}}, nil
	})
	return client, account
}

func bytes32(value byte) []byte {
	result := make([]byte, 32)
	result[0] = value
	return result
}

func TestAuthParentClientMapsBoundAuthenticatedObservation(t *testing.T) {
	client, account := parentClientFixture(t)
	parent, err := client.ObserveParent(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, account, parent.AccountID)
	require.Equal(t, ParentActive, parent.State)
	require.Equal(t, []uint32{1}, parent.Cut.RequiredVersions)
	verifier, err := NewParentVerifier(client)
	require.NoError(t, err)
	_, err = verifier.Verify(context.Background(), account)
	require.NoError(t, err)
}

func TestAuthParentClientRejectsForeignAndIncompleteObservation(t *testing.T) {
	client, account := parentClientFixture(t)
	original := client.rpc
	for _, mutate := range []func(*identityv1.ResolveAccountHandleResponse){
		func(r *identityv1.ResolveAccountHandleResponse) { r.AccountId = uuid.NewString() },
		func(r *identityv1.ResolveAccountHandleResponse) { r.RequestId = uuid.NewString() },
		func(r *identityv1.ResolveAccountHandleResponse) { r.Lifecycle.OwnerCut.Complete = false },
		func(r *identityv1.ResolveAccountHandleResponse) {
			r.Lifecycle.OwnerCut.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
		},
		func(r *identityv1.ResolveAccountHandleResponse) { r.Lifecycle.OwnerHandle = "not-a-uuid" },
		func(r *identityv1.ResolveAccountHandleResponse) { r.Lifecycle.OwnerCut.Digest = make([]byte, 32) },
		func(r *identityv1.ResolveAccountHandleResponse) {
			r.Lifecycle.OwnerCut.SelectorKeyVersions = []string{"01"}
		},
		func(r *identityv1.ResolveAccountHandleResponse) {
			r.Lifecycle.OwnerCut.SelectorKeyVersions = []string{"1", "1"}
		},
		func(r *identityv1.ResolveAccountHandleResponse) {
			r.Window.ValidUntil = timestamppb.New(time.Now().Add(-time.Second))
		},
		func(r *identityv1.ResolveAccountHandleResponse) {
			r.Window.ObservedAt = timestamppb.New(time.Now().Add(time.Minute))
		},
		func(r *identityv1.ResolveAccountHandleResponse) {
			r.Lifecycle.Disposition = identityv1.ErasureDisposition_ERASURE_DISPOSITION_IRREVERSIBLE
		},
	} {
		client.rpc = parentRPCFunc(func(ctx context.Context, req *identityv1.ResolveAccountHandleRequest, opts ...grpc.CallOption) (*identityv1.ResolveAccountHandleResponse, error) {
			response, err := original.ResolveAccountHandle(ctx, req, opts...)
			mutate(response)
			return response, err
		})
		_, err := client.ObserveParent(context.Background(), account)
		require.Error(t, err)
	}
	client.rpc = parentRPCFunc(func(ctx context.Context, req *identityv1.ResolveAccountHandleRequest, opts ...grpc.CallOption) (*identityv1.ResolveAccountHandleResponse, error) {
		response, err := original.ResolveAccountHandle(ctx, req, opts...)
		req.AccountId = uuid.NewString()
		return response, err
	})
	_, err := client.ObserveParent(context.Background(), account)
	require.Error(t, err, "mutating a dispatched request must not yield a valid parent observation")
}

func TestAuthParentClientMapsRecoverableAndTerminalLifecycle(t *testing.T) {
	client, account := parentClientFixture(t)
	original := client.rpc
	for _, state := range []identityv1.AccountLifecycleState{identityv1.AccountLifecycleState_ACCOUNT_LIFECYCLE_STATE_RECOVERABLE, identityv1.AccountLifecycleState_ACCOUNT_LIFECYCLE_STATE_TERMINAL} {
		client.rpc = parentRPCFunc(func(ctx context.Context, req *identityv1.ResolveAccountHandleRequest, opts ...grpc.CallOption) (*identityv1.ResolveAccountHandleResponse, error) {
			response, err := original.ResolveAccountHandle(ctx, req, opts...)
			response.Lifecycle.State = state
			response.Lifecycle.Eligibility = identityv1.Eligibility_ELIGIBILITY_INELIGIBLE
			response.Lifecycle.DeletionCycle = uuid.NewString()
			response.Lifecycle.EraseAt = timestamppb.New(time.Now().Add(time.Hour))
			if state == identityv1.AccountLifecycleState_ACCOUNT_LIFECYCLE_STATE_TERMINAL {
				response.Lifecycle.Disposition = identityv1.ErasureDisposition_ERASURE_DISPOSITION_IRREVERSIBLE
				response.Lifecycle.EraseAt = timestamppb.New(time.Now().Add(-time.Minute))
			}
			return response, err
		})
		observation, err := client.ObserveParent(context.Background(), account)
		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, observation.CycleID)
		if state == identityv1.AccountLifecycleState_ACCOUNT_LIFECYCLE_STATE_TERMINAL {
			require.Equal(t, ParentTerminal, observation.State)
			require.Equal(t, ParentIrreversible, observation.Disposition)
		} else {
			require.Equal(t, ParentRecoverable, observation.State)
			require.Equal(t, ParentNotIrreversible, observation.Disposition)
		}
	}
}
