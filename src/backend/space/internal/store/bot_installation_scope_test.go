package store

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/botmembershipprincipal"
)

type botInstallationScopeFixture struct {
	store    *SpaceStore
	key      *rsa.PrivateKey
	verifier botmembershipprincipal.Verifier
	add      *spacev1.AddBotMemberRequest
}

func newBotInstallationScopeFixture(t *testing.T) *botInstallationScopeFixture {
	t.Helper()
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), "postgres:18-alpine")
	db.Run(true, "goto", "34")
	db.Run(true, "goto", "45")
	transition, err := (&SpaceStore{Pool: db.Pool}).TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, err, "actual trusted Native45 transition; no marker or seal is fabricated")
	require.True(t, transition.Changed)
	st := &SpaceStore{Pool: projectionServingLogin(t, ctx, db.Pool)}
	create, _, ownerProfile := signedProjectionCreator(t, st, &spacev1.CreateSpaceRequest{Name: "typed Bot installation scope", Visibility: "private"})
	space, err := create()
	require.NoError(t, err, "real private Creator proof publishes the Native Space before Bot installation")
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	// This is an explicit controlled Bot producer identity for Space component
	// qualification, not a substitute for User birth or Bot registration proof.
	actorProfile := uuid.New()
	require.NotEqual(t, ownerProfile, actorProfile)
	intent := &spacev1.BotInstallationIntent{OperationId: uuid.NewString(), BotId: uuid.NewString(), InstallationId: uuid.NewString(), SpaceId: space.ID.String(), ActorProfileId: actorProfile.String(), Generation: 1}
	f := &botInstallationScopeFixture{store: st, key: key, add: &spacev1.AddBotMemberRequest{SpaceId: intent.SpaceId, ProfileId: intent.ActorProfileId, InstallationIntent: intent}}
	var mutex sync.Mutex
	seen := map[string]bool{}
	f.verifier = botmembershipprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(_ context.Context, issuer, jti string, _ time.Time) error {
		mutex.Lock()
		defer mutex.Unlock()
		if seen[issuer+":"+jti] {
			return errors.New("credential replay")
		}
		seen[issuer+":"+jti] = true
		return nil
	}}
	return f
}

// Every attempt passes the real private-capability verifier with a fresh signed
// transport credential while retaining the exact immutable business request.
func (f *botInstallationScopeFixture) call(t *testing.T, request proto.Message) error {
	t.Helper()
	var method, operation string
	switch r := request.(type) {
	case *spacev1.AddBotMemberRequest:
		method = spacev1.SpaceService_AddBotMember_FullMethodName
		operation = r.InstallationIntent.OperationId
	case *spacev1.RemoveBotMemberRequest:
		method = spacev1.SpaceService_RemoveBotMember_FullMethodName
		operation = r.InstallationIntent.OperationId
	default:
		t.Fatal("unsupported installation fixture request")
	}
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: f.key})
	require.NoError(t, err)
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	token, err := signer.IssueService(principal.ServiceInput{Audience: "space", RPC: method, RequestID: operation, RequestHash: hash})
	require.NoError(t, err)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", operation))
	_, err = f.verifier.Unary()(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, value any) (any, error) {
		switch r := value.(type) {
		case *spacev1.AddBotMemberRequest:
			proof, err := botmembershipprincipal.RequireAdd(ctx, r)
			if err != nil {
				return nil, err
			}
			return nil, f.store.AddBotMemberWithInstallation(ctx, proof, r)
		case *spacev1.RemoveBotMemberRequest:
			proof, err := botmembershipprincipal.RequireRemove(ctx, r)
			if err != nil {
				return nil, err
			}
			return nil, f.store.RemoveBotMemberWithInstallation(ctx, proof, r)
		}
		return nil, errors.New("unsupported installation fixture request")
	})
	return err
}

func botScopeCancel(add *spacev1.AddBotMemberRequest) *spacev1.RemoveBotMemberRequest {
	intent := proto.Clone(add.InstallationIntent).(*spacev1.BotInstallationIntent)
	intent.OperationId = uuid.NewString()
	return &spacev1.RemoveBotMemberRequest{SpaceId: add.SpaceId, ProfileId: add.ProfileId, InstallationIntent: intent, InstallOperationId: add.InstallationIntent.OperationId}
}

func requireBotInstallationScopePositive(t *testing.T, remove bool) {
	t.Helper()
	f := newBotInstallationScopeFixture(t)
	require.NoError(t, f.call(t, f.add))
	var state, operation, profile, publication string
	var generation, membership uint64
	require.NoError(t, f.store.Pool.QueryRow(context.Background(), `SELECT h.state,h.install_operation_id::text,h.profile_id::text,h.generation,m.membership_generation,m.admission_operation_id::text FROM space_bot_installation_heads h JOIN space_members m ON m.space_id=h.space_id AND m.profile_id=h.profile_id WHERE h.space_id=$1 AND h.bot_id=$2`, f.add.SpaceId, f.add.InstallationIntent.BotId).Scan(&state, &operation, &profile, &generation, &membership, &publication))
	require.Equal(t, "INSTALLED", state)
	require.Equal(t, f.add.InstallationIntent.OperationId, operation)
	require.Equal(t, operation, publication)
	require.Equal(t, f.add.ProfileId, profile)
	require.EqualValues(t, 1, generation)
	require.Positive(t, membership)
	// A fresh transport retry must acknowledge this same actual publication.
	require.NoError(t, f.call(t, f.add))
	var count int
	require.NoError(t, f.store.Pool.QueryRow(context.Background(), `SELECT count(*) FROM space_bot_installation_attempts WHERE install_operation_id=$1 AND state='PUBLISHED' AND membership_generation=$2`, operation, membership).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, f.store.Pool.QueryRow(context.Background(), `SELECT count(*) FROM space_native_member_publications WHERE operation_id=$1 AND space_id=$2 AND profile_id=$3 AND generation=$4 AND kind=6 AND state='PUBLISHED'`, operation, f.add.SpaceId, f.add.ProfileId, membership).Scan(&count))
	require.Equal(t, 1, count, "typed Bot installation must publish its actual native source")
	if !remove {
		return
	}
	cancel := botScopeCancel(f.add)
	require.NoError(t, f.call(t, cancel))
	require.NoError(t, f.call(t, cancel), "exact cancellation replay retains its original install operation")
	require.NoError(t, f.store.Pool.QueryRow(context.Background(), `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, f.add.SpaceId, f.add.ProfileId).Scan(&count))
	require.Zero(t, count)
	var floor uint64
	require.NoError(t, f.store.Pool.QueryRow(context.Background(), `SELECT state,retired_through FROM space_bot_installation_heads WHERE space_id=$1 AND bot_id=$2`, f.add.SpaceId, f.add.InstallationIntent.BotId).Scan(&state, &floor))
	require.Equal(t, "CANCELED", state)
	require.EqualValues(t, 1, floor)
	require.NoError(t, f.store.Pool.QueryRow(context.Background(), `SELECT count(*) FROM space_bot_installation_cancellations WHERE operation_id=$1 AND install_operation_id=$2`, cancel.InstallationIntent.OperationId, cancel.InstallOperationId).Scan(&count))
	require.Equal(t, 1, count)
	require.ErrorIs(t, f.call(t, f.add), ErrMemberAdmissionStale, "late Add cannot reopen the retired installation")
}

func TestBotInstallationScopeTypedAddAndExactPriorInstallationRemove(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real migrated Native45 PostgreSQL fixture")
	}
	for _, remove := range []bool{false, true} {
		name := "add"
		if remove {
			name = "remove exact installed generation"
		}
		t.Run(name, func(t *testing.T) { requireBotInstallationScopePositive(t, remove) })
	}
}

func TestBotInstallationScopeCancelBeforeAddPreservesTombstone(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real migrated Native45 PostgreSQL fixture")
	}
	f := newBotInstallationScopeFixture(t)
	cancel := botScopeCancel(f.add)
	require.NoError(t, f.call(t, cancel))
	require.ErrorIs(t, f.call(t, f.add), ErrMemberAdmissionStale)
	var state string
	var count int
	require.NoError(t, f.store.Pool.QueryRow(context.Background(), `SELECT state FROM space_bot_installation_attempts WHERE install_operation_id=$1`, cancel.InstallOperationId).Scan(&state))
	require.Equal(t, "CANCELED", state)
	require.NoError(t, f.store.Pool.QueryRow(context.Background(), `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, f.add.SpaceId, f.add.ProfileId).Scan(&count))
	require.Zero(t, count)
}
