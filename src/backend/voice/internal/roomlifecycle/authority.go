package roomlifecycle

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/principal"
)

const VoiceJoinPermission = "VOICE_JOIN"

type RoomAccess struct {
	SpaceID string
	Member  bool
	Active  bool
}

type RoomAuthority interface {
	ResolveJoinAccess(ctx context.Context, roomID, actorProfileID uuid.UUID) (RoomAccess, error)
}

type JoinPermissionQuery struct {
	SpaceID        uuid.UUID
	RoomID         uuid.UUID
	ActorProfileID uuid.UUID
	Permission     string
}

type PermissionAuthority interface {
	CheckJoinPermission(ctx context.Context, query JoinPermissionQuery) (bool, error)
}

type Authorizer struct {
	rooms       RoomAuthority
	permissions PermissionAuthority
	clock       func() time.Time
}

func NewAuthorizer(rooms RoomAuthority, permissions PermissionAuthority, clock func() time.Time) *Authorizer {
	if clock == nil {
		clock = time.Now
	}
	return &Authorizer{rooms: rooms, permissions: permissions, clock: clock}
}

type JoinDecision struct {
	actorProfileID uuid.UUID
	spaceID        uuid.UUID
	roomID         uuid.UUID
	valid          bool
}

func (d JoinDecision) ActorProfileID() string { return d.actorProfileID.String() }
func (d JoinDecision) SpaceID() string        { return d.spaceID.String() }
func (d JoinDecision) RoomID() string         { return d.roomID.String() }

func (a *Authorizer) AuthorizeJoin(ctx context.Context, pathSpaceID, pathRoomID string) (JoinDecision, error) {
	spaceID, err := parseNonNilUUID(pathSpaceID)
	if err != nil {
		return JoinDecision{}, status.Error(codes.InvalidArgument, "invalid space id")
	}
	roomID, err := parseNonNilUUID(pathRoomID)
	if err != nil {
		return JoinDecision{}, status.Error(codes.InvalidArgument, "invalid voice room id")
	}

	actor, actorProfileID, ok := verifiedVoiceActor(ctx, a.now())
	if !ok {
		return JoinDecision{}, status.Error(codes.Unauthenticated, "verified delegated user required")
	}
	if a == nil || a.rooms == nil || a.permissions == nil {
		return JoinDecision{}, status.Error(codes.Unavailable, "voice room authority unavailable")
	}

	access, err := a.rooms.ResolveJoinAccess(ctx, roomID, actorProfileID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return JoinDecision{}, status.Error(codes.NotFound, "voice room not found")
		}
		return JoinDecision{}, status.Error(codes.Unavailable, "voice room authority unavailable")
	}
	if !a.principalCurrent(actor) {
		return JoinDecision{}, status.Error(codes.Unauthenticated, "delegated user credential expired")
	}
	canonicalSpaceID, err := parseNonNilUUID(access.SpaceID)
	if err != nil {
		return JoinDecision{}, status.Error(codes.Unavailable, "invalid voice room authority response")
	}
	if !access.Member || canonicalSpaceID != spaceID {
		return JoinDecision{}, status.Error(codes.NotFound, "voice room not found")
	}
	if !access.Active {
		return JoinDecision{}, status.Error(codes.FailedPrecondition, "voice room is inactive")
	}

	allowed, err := a.permissions.CheckJoinPermission(ctx, JoinPermissionQuery{
		SpaceID: canonicalSpaceID, RoomID: roomID, ActorProfileID: actorProfileID, Permission: VoiceJoinPermission,
	})
	if err != nil {
		return JoinDecision{}, status.Error(codes.Unavailable, "voice room permission authority unavailable")
	}
	if !a.principalCurrent(actor) {
		return JoinDecision{}, status.Error(codes.Unauthenticated, "delegated user credential expired")
	}
	if !allowed {
		return JoinDecision{}, status.Error(codes.PermissionDenied, "voice join permission denied")
	}

	return JoinDecision{actorProfileID: actorProfileID, spaceID: canonicalSpaceID, roomID: roomID, valid: true}, nil
}

func (a *Authorizer) now() time.Time {
	if a == nil || a.clock == nil {
		return time.Now().UTC()
	}
	return a.clock().UTC()
}

func (a *Authorizer) principalCurrent(actor principal.Principal) bool {
	return !actor.ExpiresAt.IsZero() && a.now().Before(actor.ExpiresAt)
}

func verifiedVoiceActor(ctx context.Context, now time.Time) (principal.Principal, uuid.UUID, bool) {
	actor, ok := principal.FromContext(ctx)
	if !ok || actor.Kind != "delegated_user" || actor.Issuer != "gateway" || actor.Audience != "voice" || actor.SessionEpoch <= 0 || actor.ExpiresAt.IsZero() || !now.Before(actor.ExpiresAt) {
		return principal.Principal{}, uuid.Nil, false
	}
	accountID, err := parseNonNilUUID(actor.AccountID)
	if err != nil {
		return principal.Principal{}, uuid.Nil, false
	}
	subjectID, err := parseNonNilUUID(actor.Subject)
	if err != nil || subjectID != accountID {
		return principal.Principal{}, uuid.Nil, false
	}
	profileID, err := parseNonNilUUID(actor.ProfileID)
	if err != nil {
		return principal.Principal{}, uuid.Nil, false
	}
	return actor, profileID, true
}

func parseNonNilUUID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, status.Error(codes.InvalidArgument, "invalid uuid")
	}
	return id, nil
}
