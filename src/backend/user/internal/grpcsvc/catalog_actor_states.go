package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	catalogv1 "voice/backend/pkg/pb/voice/catalogactor/v1"
	"voice/backend/pkg/socialprincipal"
	"voice/backend/user/internal/store"
)

type CatalogActorGRPC struct {
	catalogv1.UnimplementedCatalogActorAuthorityServiceServer
	Profiles *store.ProfileStore
}

func RegisterCatalogActorServer(server grpc.ServiceRegistrar, profiles *store.ProfileStore) {
	catalogv1.RegisterCatalogActorAuthorityServiceServer(server, &CatalogActorGRPC{Profiles: profiles})
}

func (s *CatalogActorGRPC) GetCatalogActorStates(ctx context.Context, req *catalogv1.GetCatalogActorStatesRequest) (*catalogv1.GetCatalogActorStatesResponse, error) {
	if err := socialprincipal.RequireCatalogActor(ctx, req); err != nil {
		return nil, err
	}
	if req == nil || req.ProtocolVersion != 1 || len(req.Actors) < 1 || len(req.Actors) > 100 || len(req.ProtoReflect().GetUnknown()) != 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid catalog actor batch")
	}
	ids := make([]uuid.UUID, len(req.Actors))
	accounts := make([]uuid.UUID, len(ids))
	seen := make(map[uuid.UUID]bool, len(ids))
	for i, actor := range req.Actors {
		if actor == nil || len(actor.ProtoReflect().GetUnknown()) != 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid catalog actor")
		}
		account, err := uuid.Parse(actor.AccountId)
		if err != nil || account == uuid.Nil || actor.AccountId != account.String() {
			return nil, status.Error(codes.InvalidArgument, "invalid catalog account_id")
		}
		profile, err := uuid.Parse(actor.ProfileId)
		if err != nil || profile == uuid.Nil || actor.ProfileId != profile.String() || seen[profile] {
			return nil, status.Error(codes.InvalidArgument, "invalid or duplicate catalog profile_id")
		}
		ids[i] = profile
		accounts[i] = account
		seen[profile] = true
	}
	if s == nil || s.Profiles == nil {
		return nil, status.Error(codes.Unavailable, "catalog actor authority unavailable")
	}
	snapshots, err := s.Profiles.CatalogActorStates(ctx, ids)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "catalog actor authority unavailable")
	}
	response := &catalogv1.GetCatalogActorStatesResponse{ProtocolVersion: 1, States: make([]*catalogv1.CatalogActorStateEntry, len(ids))}
	for i, id := range ids {
		state := catalogv1.CatalogActorState_CATALOG_ACTOR_STATE_UNKNOWN
		if entry, exists := snapshots[id]; exists && entry.AccountID == accounts[i] {
			state = catalogv1.CatalogActorState_CATALOG_ACTOR_STATE_NOT_DELETED
			if entry.Deleted {
				state = catalogv1.CatalogActorState_CATALOG_ACTOR_STATE_SOFT_DELETED
			}
		}
		response.States[i] = &catalogv1.CatalogActorStateEntry{Actor: &catalogv1.CatalogActor{AccountId: accounts[i].String(), ProfileId: id.String()}, State: state}
	}
	return response, nil
}
