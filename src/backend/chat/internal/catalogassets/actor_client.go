package catalogassets

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	authv1 "voice.app/voice/auth/v1"
	catalogactorv1 "voice/backend/pkg/pb/voice/catalogactor/v1"
	"voice/backend/pkg/principal"
)

type ProfileState uint8

const (
	ProfileUnknown ProfileState = iota
	ProfileLive
	ProfileSoftDeleted
)

type ActorObservation struct {
	SessionEpochFloor int64
	Profile           ProfileState
}
type ActorObserver interface {
	Observe(context.Context, Actor) (ActorObservation, error)
}
type AuthFloorClient interface {
	GetChatSessionEpochFloor(context.Context, *authv1.GetChatSessionEpochFloorRequest, ...grpc.CallOption) (*authv1.GetChatSessionEpochFloorResponse, error)
}
type GRPCActorObserver struct {
	Auth   AuthFloorClient
	User   catalogactorv1.CatalogActorAuthorityServiceClient
	Issuer *principal.Issuer
}

func (c *GRPCActorObserver) Observe(ctx context.Context, actor Actor) (ActorObservation, error) {
	if actor.AccountID == uuid.Nil || actor.ProfileID == uuid.Nil {
		return ActorObservation{}, status.Error(codes.InvalidArgument, "invalid catalog actor")
	}
	floor, err := c.accountFloor(ctx, actor.AccountID.String())
	if err != nil {
		return ActorObservation{}, err
	}
	states, err := c.ProfileStates(ctx, []Actor{actor})
	if err != nil {
		return ActorObservation{}, err
	}
	return ActorObservation{SessionEpochFloor: floor, Profile: states[actor]}, nil
}

func (c *GRPCActorObserver) CheckSessionEpoch(ctx context.Context, account string, epoch int64) error {
	if !canonicalID(account) || epoch <= 0 {
		return status.Error(codes.Unauthenticated, "invalid catalog actor session")
	}
	floor, err := c.accountFloor(ctx, account)
	if err != nil {
		return err
	}
	if epoch < floor {
		return status.Error(codes.Unauthenticated, "catalog actor session revoked")
	}
	return nil
}

func (c *GRPCActorObserver) accountFloor(ctx context.Context, account string) (int64, error) {
	if c == nil || c.Auth == nil || c.Issuer == nil {
		return 0, status.Error(codes.Unavailable, "catalog account authority unavailable")
	}
	req := &authv1.GetChatSessionEpochFloorRequest{AccountId: account}
	call, cancel, err := bindServiceCall(ctx, c.Issuer, "auth", authv1.AuthService_GetChatSessionEpochFloor_FullMethodName, req, 3*time.Second)
	if err != nil {
		return 0, err
	}
	defer cancel()
	response, err := c.Auth.GetChatSessionEpochFloor(call, req)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return 0, status.Error(codes.PermissionDenied, "catalog account unavailable")
		}
		return 0, status.Error(codes.Unavailable, "catalog account authority unavailable")
	}
	if response == nil || hasUnknown(response) || response.GetSessionEpochFloor() <= 0 {
		return 0, status.Error(codes.Unavailable, "invalid catalog account authority response")
	}
	return response.SessionEpochFloor, nil
}

func (c *GRPCActorObserver) ProfileStates(ctx context.Context, actors []Actor) (map[Actor]ProfileState, error) {
	if c == nil || c.User == nil || c.Issuer == nil {
		return nil, status.Error(codes.Unavailable, "catalog profile authority unavailable")
	}
	if len(actors) > 100 {
		return nil, status.Error(codes.InvalidArgument, "catalog profile batch too large")
	}
	out := make(map[Actor]ProfileState, len(actors))
	if len(actors) == 0 {
		return out, nil
	}
	req := &catalogactorv1.GetCatalogActorStatesRequest{ProtocolVersion: 1}
	seen := make(map[uuid.UUID]bool, len(actors))
	for _, actor := range actors {
		if actor.AccountID == uuid.Nil || actor.ProfileID == uuid.Nil || seen[actor.ProfileID] {
			return nil, status.Error(codes.InvalidArgument, "invalid or duplicate catalog actor")
		}
		seen[actor.ProfileID] = true
		req.Actors = append(req.Actors, &catalogactorv1.CatalogActor{AccountId: actor.AccountID.String(), ProfileId: actor.ProfileID.String()})
	}
	call, cancel, err := bindServiceCall(ctx, c.Issuer, "user", catalogactorv1.CatalogActorAuthorityService_GetCatalogActorStates_FullMethodName, req, 3*time.Second)
	if err != nil {
		return nil, err
	}
	defer cancel()
	response, err := c.User.GetCatalogActorStates(call, req)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "catalog profile authority unavailable")
	}
	if response == nil || hasUnknown(response) || response.GetProtocolVersion() != 1 || len(response.GetStates()) != len(actors) {
		return nil, status.Error(codes.Unavailable, "invalid catalog profile authority response")
	}
	for i, entry := range response.States {
		actor := actors[i]
		if entry.GetActor().GetAccountId() != actor.AccountID.String() || entry.GetActor().GetProfileId() != actor.ProfileID.String() {
			return nil, status.Error(codes.Unavailable, "invalid catalog profile authority binding")
		}
		switch entry.GetState() {
		case catalogactorv1.CatalogActorState_CATALOG_ACTOR_STATE_NOT_DELETED:
			out[actor] = ProfileLive
		case catalogactorv1.CatalogActorState_CATALOG_ACTOR_STATE_SOFT_DELETED:
			out[actor] = ProfileSoftDeleted
		case catalogactorv1.CatalogActorState_CATALOG_ACTOR_STATE_UNKNOWN:
			out[actor] = ProfileUnknown
		default:
			return nil, status.Error(codes.Unavailable, "invalid catalog profile authority state")
		}
	}
	return out, nil
}
