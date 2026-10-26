package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/catalogasset"
	"voice/backend/file/internal/store"
)

type CatalogByteFetcher interface {
	FetchGIF(context.Context, string, string, int64) ([]byte, error)
}

// Implementations acknowledge exactly one tracked storage attempt; the real
// r2file implementation disables SDK retries and all HTTP redirects.
type CatalogSourceWriter interface {
	PutCatalogSource(context.Context, string, []byte) error
}

func (s *CatalogAssetGRPC) ImportCatalogAsset(ctx context.Context, r *filev1.ImportCatalogAssetRequest) (*filev1.ImportCatalogAssetResponse, error) {
	if err := catalogCaller(ctx, r, filev1.CatalogAssetService_ImportCatalogAsset_FullMethodName, "chat"); err != nil {
		return nil, err
	}
	if r == nil || r.ProtocolVersion != 2 || r.DestinationChat == nil || len(r.DestinationChat.ProtoReflect().GetUnknown()) != 0 || !catalogCanonicalID(r.OperationId) || !catalogCanonicalID(r.SourceId) || !catalogCanonicalID(r.ChatAccountGeneration) || !catalogCanonicalID(r.ChatProfileGeneration) || !catalogCanonicalID(r.DestinationChat.Id) || r.Provider == "" || r.ProviderId == "" || r.Provider != strings.TrimSpace(r.Provider) || r.ProviderId != strings.TrimSpace(r.ProviderId) || r.SourceUrl == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid catalog import")
	}
	if s == nil || s.Destination == nil || nilCatalogAuthority(s.Destination) || s.Fetcher == nil || nilCatalogAuthority(s.Fetcher) || s.Writer == nil || nilCatalogAuthority(s.Writer) || s.Keys == nil {
		return nil, status.Error(codes.Unavailable, "catalog importer unavailable")
	}
	actor, err := s.catalogActor(ctx, r.ActorAccountId, r.ActorProfileId)
	if err != nil {
		return nil, err
	}
	capture, err := s.Files.CaptureCatalogActor(ctx, actor, s.Keys)
	if err != nil {
		return nil, catalogImportError(err)
	}
	observed, err := s.Destination.Authorize(ctx, catalogasset.DestinationRequest{Actor: actor, Destination: r.DestinationChat, ChatAccountGeneration: uuid.MustParse(r.ChatAccountGeneration), ChatProfileGeneration: uuid.MustParse(r.ChatProfileGeneration)})
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "catalog destination unavailable")
	}
	premium := false
	if s.Entitlements != nil && !nilCatalogAuthority(s.Entitlements) {
		value, e := s.Entitlements.Premium(ctx, actor.AccountID, s.catalogNow().UTC())
		premium = e == nil && value
	}
	clock := s.Clock
	if clock == nil {
		clock = realClock{}
	}
	// Initial unattached File policy is the existing verified upload policy.
	// This is not a new per-reference retention entitlement or a fake reference.
	expires := retentionExpiresAt(clock, premium, false)
	imp, created, err := s.Files.BeginCatalogImport(ctx, r, capture, s.Keys, observed, expires, 50<<20)
	if err != nil {
		return nil, catalogImportError(err)
	}
	if !created {
		return s.catalogImportResponse(ctx, r)
	}
	body, fetchErr := s.Fetcher.FetchGIF(ctx, r.Provider, r.SourceUrl, imp.MaxFetchBytes)
	if fetchErr == nil && (len(body) < 6 || int64(len(body)) > imp.MaxFetchBytes || (!bytes.Equal(body[:6], []byte("GIF89a")) && !bytes.Equal(body[:6], []byte("GIF87a")))) {
		fetchErr = catalogasset.ErrSource
	}
	if fetchErr != nil {
		cleanup, cancel := catalogCompletionContext(ctx)
		defer cancel()
		if err = s.Files.FailCatalogFetch(cleanup, imp, s.Keys); err != nil {
			return nil, catalogImportError(err)
		}
		return nil, status.Error(codes.Unavailable, "catalog source unavailable")
	}
	digest := sha256.Sum256(body)
	if err = s.Files.RecordCatalogFetched(ctx, imp, capture, s.Keys, int64(len(body)), digest[:]); err != nil {
		return nil, catalogImportError(err)
	}
	// Recheck the current destination and actor before crossing the object-write
	// boundary. The local generation check in Start also rejects a concurrent
	// File purge; no server cache or source File ID grants chat permission.
	if _, err = s.catalogActor(ctx, r.ActorAccountId, r.ActorProfileId); err != nil {
		if cleanupErr := s.abortCatalogPrepared(ctx, imp); cleanupErr != nil {
			return nil, cleanupErr
		}
		return nil, err
	}
	if _, err = s.Destination.Authorize(ctx, catalogasset.DestinationRequest{Actor: actor, Destination: r.DestinationChat, ChatAccountGeneration: uuid.MustParse(r.ChatAccountGeneration), ChatProfileGeneration: uuid.MustParse(r.ChatProfileGeneration)}); err != nil {
		if cleanupErr := s.abortCatalogPrepared(ctx, imp); cleanupErr != nil {
			return nil, cleanupErr
		}
		return nil, status.Error(codes.PermissionDenied, "catalog destination unavailable")
	}
	if err = s.Files.StartCatalogWrite(ctx, imp, capture, s.Keys); err != nil {
		if cleanupErr := s.abortCatalogPrepared(ctx, imp); cleanupErr != nil {
			return nil, cleanupErr
		}
		return nil, catalogImportError(err)
	}
	writeErr := s.Writer.PutCatalogSource(ctx, imp.R2Key, body)
	completion, cancel := catalogCompletionContext(ctx)
	defer cancel()
	if err = s.Files.SettleCatalogWrite(completion, imp, capture, s.Keys, writeErr == nil); err != nil {
		return nil, catalogImportError(err)
	}
	if writeErr != nil {
		return nil, status.Error(codes.Unavailable, "catalog source write requires reconciliation")
	}
	return s.catalogImportResponse(completion, r)
}

func (s *CatalogAssetGRPC) abortCatalogPrepared(ctx context.Context, imp store.CatalogImport) error {
	cleanup, cancel := catalogCompletionContext(ctx)
	defer cancel()
	if err := s.Files.AbortCatalogPrepared(cleanup, imp, s.Keys); err != nil {
		return catalogImportError(err)
	}
	return nil
}

func catalogCompletionContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
}

func (s *CatalogAssetGRPC) catalogImportResponse(ctx context.Context, r *filev1.ImportCatalogAssetRequest) (*filev1.ImportCatalogAssetResponse, error) {
	state, err := s.Files.CatalogStatus(ctx, &filev1.GetCatalogAssetStatusRequest{ProtocolVersion: 2, ActorAccountId: r.ActorAccountId, ActorProfileId: r.ActorProfileId, Selector: &filev1.GetCatalogAssetStatusRequest_Operation{Operation: &filev1.CatalogImportOperationSelector{OperationId: r.OperationId, Provider: r.Provider, ProviderId: r.ProviderId, DestinationChat: r.DestinationChat}}}, s.Keys, s.catalogNow())
	if err != nil {
		return nil, catalogImportError(err)
	}
	return &filev1.ImportCatalogAssetResponse{SourceId: state.SourceId, ActorAccountId: state.ActorAccountId, ActorProfileId: state.ActorProfileId, Provider: state.Provider, ProviderId: state.ProviderId, FileId: state.FileId, Status: state.Status, OperationId: state.OperationId, DestinationChat: state.DestinationChat, ChatAccountGeneration: state.ChatAccountGeneration, ChatProfileGeneration: state.ChatProfileGeneration, SelectionRevision: state.SelectionRevision, CurrentSelectionRevision: state.CurrentSelectionRevision}, nil
}

func catalogImportError(err error) error {
	if errors.Is(err, store.ErrCatalogConflict) {
		return status.Error(codes.AlreadyExists, "catalog operation selection conflicts")
	}
	if errors.Is(err, store.ErrCatalogTerminal) || errors.Is(err, store.ErrCatalogActorClosed) {
		return status.Error(codes.FailedPrecondition, "catalog import no longer available")
	}
	return catalogReadError(err)
}
