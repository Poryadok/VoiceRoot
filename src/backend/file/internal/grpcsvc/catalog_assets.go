package grpcsvc

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/catalogasset"
	"voice/backend/file/internal/store"
	"voice/backend/pkg/principal"
)

type CatalogActorAuthority interface {
	States(context.Context, []catalogasset.ActorPair) ([]catalogasset.ActorState, error)
}

type CatalogDestinationAuthority interface {
	Authorize(context.Context, catalogasset.DestinationRequest) (chatv1.ChatType, error)
}

// CatalogAssetGRPC is registered only on the protected File listener. Each
// handler also checks the private verified principal and exact request binding;
// forwarded user headers never authorize source-scoped metadata access.
type CatalogAssetGRPC struct {
	filev1.UnimplementedCatalogAssetServiceServer
	Files        *store.FilesStore
	Actors       CatalogActorAuthority
	Keys         *catalogasset.ReplayKeys
	Clock        Clock
	Destination  CatalogDestinationAuthority
	Fetcher      CatalogByteFetcher
	Writer       CatalogSourceWriter
	Entitlements EntitlementResolver
}

func (s *CatalogAssetGRPC) GetCatalogAssetStatus(ctx context.Context, r *filev1.GetCatalogAssetStatusRequest) (*filev1.GetCatalogAssetStatusResponse, error) {
	if err := catalogCaller(ctx, r, filev1.CatalogAssetService_GetCatalogAssetStatus_FullMethodName, "chat"); err != nil {
		return nil, err
	}
	if r == nil || r.ProtocolVersion != 2 {
		return nil, status.Error(codes.InvalidArgument, "invalid catalog status")
	}
	actor, err := s.catalogActor(ctx, r.ActorAccountId, r.ActorProfileId)
	if err != nil {
		return nil, err
	}
	out, err := s.Files.CatalogStatus(ctx, r, s.Keys, s.catalogNow())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) && r.GetOperation() != nil {
			return nil, status.Error(codes.NotFound, "catalog operation unknown")
		}
		if errors.Is(err, store.ErrCatalogConflict) {
			return nil, status.Error(codes.AlreadyExists, "catalog operation selection conflicts")
		}
		if errors.Is(err, store.ErrCatalogTerminal) || errors.Is(err, store.ErrCatalogActorClosed) {
			return nil, status.Error(codes.FailedPrecondition, "catalog operation unavailable")
		}
		return nil, catalogReadError(err)
	}
	if r.GetOperation() != nil {
		if s.Destination == nil || nilCatalogAuthority(s.Destination) || !catalogCanonicalID(out.ChatAccountGeneration) || !catalogCanonicalID(out.ChatProfileGeneration) {
			return nil, status.Error(codes.Unavailable, "catalog destination authority unavailable")
		}
		_, err = s.Destination.Authorize(ctx, catalogasset.DestinationRequest{Actor: actor, Destination: out.DestinationChat, ChatAccountGeneration: uuid.MustParse(out.ChatAccountGeneration), ChatProfileGeneration: uuid.MustParse(out.ChatProfileGeneration)})
		if err != nil {
			return nil, status.Error(codes.PermissionDenied, "catalog destination unavailable")
		}
	}
	return out, nil
}

func (s *CatalogAssetGRPC) LookupCatalogAsset(ctx context.Context, r *filev1.LookupCatalogAssetRequest) (*filev1.LookupCatalogAssetResponse, error) {
	if err := catalogCaller(ctx, r, filev1.CatalogAssetService_LookupCatalogAsset_FullMethodName, "messaging"); err != nil {
		return nil, err
	}
	if r == nil || r.ProtocolVersion != 2 || !catalogCanonicalID(r.FileId) {
		return nil, status.Error(codes.InvalidArgument, "invalid catalog lookup")
	}
	actor, err := s.catalogActor(ctx, r.ActorAccountId, r.ActorProfileId)
	if err != nil {
		return nil, err
	}
	out, err := s.Files.LookupCatalogFile(ctx, actor, uuid.MustParse(r.FileId), s.catalogNow())
	if err != nil {
		return nil, catalogReadError(err)
	}
	return out, nil
}

func (s *CatalogAssetGRPC) ValidateCatalogAsset(ctx context.Context, r *filev1.ValidateCatalogAssetRequest) (*filev1.ValidateCatalogAssetResponse, error) {
	if err := catalogCaller(ctx, r, filev1.CatalogAssetService_ValidateCatalogAsset_FullMethodName, "messaging"); err != nil {
		return nil, err
	}
	if r == nil || r.ProtocolVersion != 2 || !catalogCanonicalID(r.SourceId) || !catalogCanonicalID(r.FileId) || r.Provider == "" || r.ProviderId == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid catalog validation")
	}
	if _, err := s.catalogActor(ctx, r.ActorAccountId, r.ActorProfileId); err != nil {
		return nil, err
	}
	out, err := s.Files.ValidateCatalogFile(ctx, r, s.catalogNow())
	if err != nil {
		return nil, catalogReadError(err)
	}
	return out, nil
}

func catalogCaller(ctx context.Context, r proto.Message, method, issuer string) error {
	if r == nil || !r.ProtoReflect().IsValid() {
		return status.Error(codes.Unauthenticated, "catalog service principal required")
	}
	p, ok := principal.FromContext(ctx)
	hash, err := principal.RequestHash(r)
	if !ok || err != nil || p.Kind != "service" || p.Issuer != issuer || p.Subject != "service:"+issuer || p.Audience != "file" || p.RPC != method || p.RequestHash != hash || !catalogCanonicalID(p.RequestID) || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 {
		return status.Error(codes.Unauthenticated, "catalog service principal required")
	}
	if len(r.ProtoReflect().GetUnknown()) != 0 {
		return status.Error(codes.InvalidArgument, "invalid catalog request")
	}
	return nil
}

func catalogCanonicalID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

func (s *CatalogAssetGRPC) catalogActor(ctx context.Context, account, profile string) (catalogasset.ActorPair, error) {
	actor := catalogasset.ActorPair{}
	if !catalogCanonicalID(account) || !catalogCanonicalID(profile) {
		return actor, status.Error(codes.InvalidArgument, "invalid catalog actor")
	}
	actor.AccountID = uuid.MustParse(account)
	actor.ProfileID = uuid.MustParse(profile)
	if s == nil || s.Files == nil || s.Actors == nil || nilCatalogAuthority(s.Actors) {
		return actor, status.Error(codes.Unavailable, "catalog authority unavailable")
	}
	states, err := s.Actors.States(ctx, []catalogasset.ActorPair{actor})
	if err != nil || len(states) != 1 {
		return actor, status.Error(codes.Unavailable, "catalog actor authority unavailable")
	}
	if states[0] == catalogasset.ActorSoftDeleted {
		if err = s.Files.PurgeCatalogProfile(ctx, actor, s.Keys); err != nil {
			return actor, status.Error(codes.Unavailable, "catalog actor cleanup unavailable")
		}
	}
	if states[0] != catalogasset.ActorLive {
		return actor, status.Error(codes.PermissionDenied, "catalog actor unavailable")
	}
	return actor, nil
}

func nilCatalogAuthority(value any) bool {
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return v.IsNil()
	}
	return false
}
func (s *CatalogAssetGRPC) catalogNow() time.Time {
	if s.Clock != nil {
		return s.Clock.Now()
	}
	return time.Now()
}
func catalogReadError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return status.Error(codes.PermissionDenied, "catalog binding unavailable")
	}
	if errors.Is(err, catalogasset.ErrAdmission) {
		return status.Error(codes.FailedPrecondition, "catalog asset unavailable")
	}
	return status.Error(codes.Unavailable, "catalog file authority unavailable")
}
