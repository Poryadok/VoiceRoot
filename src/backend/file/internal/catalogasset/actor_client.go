package catalogasset

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
	catalogactorv1 "voice/backend/pkg/pb/voice/catalogactor/v1"
	"voice/backend/pkg/principal"
)

var ErrActorAuthority = errors.New("catalog actor authority unavailable")
var ErrActorRequest = errors.New("invalid catalog actor batch")

type ActorPair struct{ AccountID, ProfileID uuid.UUID }
type ActorState uint8

const (
	ActorUnknown ActorState = iota
	ActorLive
	ActorSoftDeleted
)

// ActorAuthorityClient reads a User-owned snapshot using File's own signer.
// UNKNOWN is not deletion evidence. No snapshot is an atomic permit or a local
// generation gate, nor does a live profile prove Auth account eligibility.
type ActorAuthorityClient struct {
	Client catalogactorv1.CatalogActorAuthorityServiceClient
	Issuer *principal.Issuer
}

func nilActorClient(client any) bool {
	if client == nil {
		return true
	}
	v := reflect.ValueOf(client)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	}
	return false
}

func (c *ActorAuthorityClient) States(ctx context.Context, actors []ActorPair) ([]ActorState, error) {
	if len(actors) == 0 || len(actors) > 100 {
		return nil, ErrActorRequest
	}
	request := &catalogactorv1.GetCatalogActorStatesRequest{ProtocolVersion: 1}
	seen := make(map[uuid.UUID]bool, len(actors))
	for _, actor := range actors {
		if actor.AccountID == uuid.Nil || actor.ProfileID == uuid.Nil || seen[actor.ProfileID] {
			return nil, ErrActorRequest
		}
		seen[actor.ProfileID] = true
		request.Actors = append(request.Actors, &catalogactorv1.CatalogActor{AccountId: actor.AccountID.String(), ProfileId: actor.ProfileID.String()})
	}
	if c == nil || nilActorClient(c.Client) || c.Issuer == nil {
		return nil, ErrActorAuthority
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, ErrActorRequest
	}
	requestID := uuid.NewString()
	token, err := c.Issuer.IssueService(principal.ServiceInput{Audience: "user", RPC: catalogactorv1.CatalogActorAuthorityService_GetCatalogActorStates_FullMethodName, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return nil, ErrActorAuthority
	}
	call, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	call = metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	response, err := c.Client.GetCatalogActorStates(call, request)
	if err != nil {
		return nil, errors.Join(ErrActorAuthority, call.Err())
	}
	if response == nil || response.ProtocolVersion != 1 || len(response.States) != len(actors) || len(response.ProtoReflect().GetUnknown()) != 0 {
		return nil, ErrActorAuthority
	}
	states := make([]ActorState, len(actors))
	for i, entry := range response.States {
		if entry == nil || entry.Actor == nil || len(entry.ProtoReflect().GetUnknown()) != 0 || len(entry.Actor.ProtoReflect().GetUnknown()) != 0 || entry.Actor.AccountId != request.Actors[i].AccountId || entry.Actor.ProfileId != request.Actors[i].ProfileId {
			return nil, ErrActorAuthority
		}
		switch entry.State {
		case catalogactorv1.CatalogActorState_CATALOG_ACTOR_STATE_UNKNOWN:
			states[i] = ActorUnknown
		case catalogactorv1.CatalogActorState_CATALOG_ACTOR_STATE_NOT_DELETED:
			states[i] = ActorLive
		case catalogactorv1.CatalogActorState_CATALOG_ACTOR_STATE_SOFT_DELETED:
			states[i] = ActorSoftDeleted
		default:
			return nil, ErrActorAuthority
		}
	}
	return states, nil
}
