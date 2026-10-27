package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/catalogasset"
	"voice/backend/file/internal/store"
	"voice/backend/pkg/principal"
)

type catalogActorSnapshotFunc func(context.Context, []catalogasset.ActorPair) ([]catalogasset.ActorState, error)

func (f catalogActorSnapshotFunc) States(ctx context.Context, a []catalogasset.ActorPair) ([]catalogasset.ActorState, error) {
	return f(ctx, a)
}

func catalogVerifiedContext(t *testing.T, r proto.Message, method, issuer string) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(r)
	require.NoError(t, err)
	return principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Subject: "service:" + issuer, Issuer: issuer, Audience: "file", RPC: method, RequestID: uuid.NewString(), RequestHash: hash})
}

func TestCatalogHandlersRequireBoundPrivateMessagingPrincipal(t *testing.T) {
	r := &filev1.LookupCatalogAssetRequest{ProtocolVersion: 2, ActorAccountId: uuid.NewString(), ActorProfileId: uuid.NewString(), FileId: uuid.NewString()}
	s := &CatalogAssetGRPC{}
	public := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-account-id", r.ActorAccountId, "x-voice-profile-id", r.ActorProfileId, "authorization", "Bearer forged"))
	for _, ctx := range []context.Context{public, catalogVerifiedContext(t, r, filev1.CatalogAssetService_LookupCatalogAsset_FullMethodName, "chat"), catalogVerifiedContext(t, r, filev1.CatalogAssetService_ValidateCatalogAsset_FullMethodName, "messaging")} {
		_, err := s.LookupCatalogAsset(ctx, r)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	}
	ctx := catalogVerifiedContext(t, r, filev1.CatalogAssetService_LookupCatalogAsset_FullMethodName, "messaging")
	r.FileId = uuid.NewString()
	_, err := s.LookupCatalogAsset(ctx, r)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "valid principal bound to another body must fail")
}

func TestCatalogHandlersUseActorSnapshotAndAuthoritativeDatabase(t *testing.T) {
	ctx := context.Background()
	f := seedCatalogPurgeFixture(t, ctx, true)
	seedCatalogReadyRepresentation(t, ctx, f)
	state := catalogasset.ActorLive
	calls := 0
	s := &CatalogAssetGRPC{Files: store.NewFilesStore(f.pool), Keys: f.keys, Actors: catalogActorSnapshotFunc(func(_ context.Context, actors []catalogasset.ActorPair) ([]catalogasset.ActorState, error) {
		require.Equal(t, []catalogasset.ActorPair{f.actor}, actors)
		calls++
		return []catalogasset.ActorState{state}, nil
	})}
	r := &filev1.LookupCatalogAssetRequest{ProtocolVersion: 2, ActorAccountId: f.actor.AccountID.String(), ActorProfileId: f.actor.ProfileID.String(), FileId: f.file.String()}
	call := func() (*filev1.LookupCatalogAssetResponse, error) {
		return s.LookupCatalogAsset(catalogVerifiedContext(t, r, filev1.CatalogAssetService_LookupCatalogAsset_FullMethodName, "messaging"), r)
	}
	out, err := call()
	require.NoError(t, err)
	require.Equal(t, f.source.String(), out.GetCatalog().SourceId)
	state = catalogasset.ActorUnknown
	_, err = call()
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	var gate string
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT state FROM file_catalog_actor_gates WHERE id=$1`, f.profileGate).Scan(&gate))
	require.Equal(t, "LIVE", gate, "UNKNOWN is not purge authority")
	state = catalogasset.ActorSoftDeleted
	_, err = call()
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT state FROM file_catalog_actor_gates WHERE id=$1`, f.profileGate).Scan(&gate))
	require.Equal(t, "CLOSED", gate)
	state = catalogasset.ActorLive
	_, err = call()
	require.Error(t, err, "later live snapshot cannot reopen local closed generation")
	require.Equal(t, 4, calls)
}
