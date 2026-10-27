package grpcsvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/catalogasset"
)

type catalogFetchFunc func(context.Context, string, string, int64) ([]byte, error)

func (f catalogFetchFunc) FetchGIF(ctx context.Context, p, u string, max int64) ([]byte, error) {
	return f(ctx, p, u, max)
}

type catalogPutFunc func(context.Context, string, []byte) error

func (f catalogPutFunc) PutCatalogSource(ctx context.Context, k string, b []byte) error {
	return f(ctx, k, b)
}

func TestCatalogImportHandlerPersistsBeforeIOAndPreservesRetryIdentity(t *testing.T) {
	ctx := context.Background()
	f := seedCatalogInput(t, ctx)
	body := []byte("GIF89a-source")
	fetches, puts, destinations := 0, 0, 0
	s := &CatalogAssetGRPC{Files: f.files, Keys: f.keys, Actors: catalogActorSnapshotFunc(func(context.Context, []catalogasset.ActorPair) ([]catalogasset.ActorState, error) {
		return []catalogasset.ActorState{catalogasset.ActorLive}, nil
	}), Destination: catalogDestinationFunc(func(_ context.Context, r catalogasset.DestinationRequest) (chatv1.ChatType, error) {
		destinations++
		require.Equal(t, f.actor, r.Actor)
		require.Equal(t, f.request.ChatAccountGeneration, r.ChatAccountGeneration.String())
		return chatv1.ChatType_CHAT_TYPE_GROUP, nil
	})}
	s.Fetcher = catalogFetchFunc(func(_ context.Context, p, u string, max int64) ([]byte, error) {
		fetches++
		require.Equal(t, f.request.Provider, p)
		require.Equal(t, f.request.SourceUrl, u)
		require.EqualValues(t, 50<<20, max)
		var state string
		var size int64
		require.NoError(t, f.pool.QueryRow(ctx, `SELECT status,size_bytes FROM files`).Scan(&state, &size))
		require.Equal(t, "pending_upload", state)
		require.Zero(t, size)
		return body, nil
	})
	s.Writer = catalogPutFunc(func(_ context.Context, k string, b []byte) error {
		puts++
		require.Equal(t, body, b)
		var state, sha string
		var size int64
		require.NoError(t, f.pool.QueryRow(ctx, `SELECT w.state,f.size_bytes,f.sha256_hash FROM file_catalog_writes w JOIN files f ON f.id=w.file_id WHERE w.object_key=$1`, k).Scan(&state, &size, &sha))
		require.Equal(t, "STARTED", state)
		require.EqualValues(t, len(body), size)
		digest := sha256.Sum256(body)
		require.Equal(t, hex.EncodeToString(digest[:]), sha)
		return nil
	})
	call := func() (*filev1.ImportCatalogAssetResponse, error) {
		return s.ImportCatalogAsset(catalogVerifiedContext(t, f.request, filev1.CatalogAssetService_ImportCatalogAsset_FullMethodName, "chat"), f.request)
	}
	out, err := call()
	require.NoError(t, err)
	require.Equal(t, "processing", out.Status)
	require.Equal(t, f.request.SourceId, out.SourceId)
	require.Equal(t, f.request.OperationId, out.OperationId)
	first := out.FileId
	out, err = call()
	require.NoError(t, err)
	require.Equal(t, first, out.FileId)
	require.Equal(t, 1, fetches)
	require.Equal(t, 1, puts)
	require.GreaterOrEqual(t, destinations, 2, "retry must independently recheck current destination")
	f.request.SourceUrl = "https://example.com/changed.gif"
	out, err = call()
	require.Equal(t, codes.AlreadyExists, status.Code(err))
	require.Nil(t, out)
	s.Destination = catalogDestinationFunc(func(context.Context, catalogasset.DestinationRequest) (chatv1.ChatType, error) {
		return 0, errors.New("no current destination membership")
	})
	f.request.OperationId = uuid.NewString()
	f.request.SourceId = uuid.NewString()
	out, err = call()
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Nil(t, out)
	require.Equal(t, 1, fetches)
	require.Equal(t, 1, puts)
}

func TestCatalogImportLostDestinationAfterFetchNeverStartsPUT(t *testing.T) {
	ctx := context.Background()
	f := seedCatalogInput(t, ctx)
	checks, puts := 0, 0
	s := &CatalogAssetGRPC{Files: f.files, Keys: f.keys, Actors: catalogActorSnapshotFunc(func(context.Context, []catalogasset.ActorPair) ([]catalogasset.ActorState, error) {
		return []catalogasset.ActorState{catalogasset.ActorLive}, nil
	}), Destination: catalogDestinationFunc(func(context.Context, catalogasset.DestinationRequest) (chatv1.ChatType, error) {
		checks++
		if checks == 2 {
			return 0, errors.New("membership removed while fetching")
		}
		return chatv1.ChatType_CHAT_TYPE_GROUP, nil
	}), Fetcher: catalogFetchFunc(func(context.Context, string, string, int64) ([]byte, error) { return []byte("GIF89a-source"), nil }), Writer: catalogPutFunc(func(context.Context, string, []byte) error { puts++; return nil })}
	out, err := s.ImportCatalogAsset(catalogVerifiedContext(t, f.request, filev1.CatalogAssetService_ImportCatalogAsset_FullMethodName, "chat"), f.request)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Nil(t, out)
	require.Zero(t, puts)
	var state string
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT status FROM files`).Scan(&state))
	require.Equal(t, "failed", state)
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT state FROM file_catalog_writes`).Scan(&state))
	require.Equal(t, "GC_PENDING", state)
	var mappings int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM file_catalog_assets`).Scan(&mappings))
	require.Zero(t, mappings)
}
