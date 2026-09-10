package grpcsvc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"

	spacev1 "voice.app/voice/space/v1"
)

// ResolveVoiceRoomAccess returns Space-owned room and exact membership
// evidence to the authenticated Voice service. User metadata is never an S2S
// credential and is ignored for authorization of this endpoint.
func (s *SpaceGRPC) ResolveVoiceRoomAccess(ctx context.Context, req *spacev1.ResolveVoiceRoomAccessRequest) (*spacev1.ResolveVoiceRoomAccessResponse, error) {
	if s == nil || (s.Store == nil && s.voiceRoomAccessResolver == nil) {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	identity, ok := authctx.VerifiedServiceIdentity(ctx)
	if !ok || identity != authctx.ServiceIdentityVoice {
		return nil, status.Error(codes.Unauthenticated, "verified Voice service identity required")
	}
	voiceRoomID, err := parseUUIDField("voice_room_id", req.GetVoiceRoomId())
	if err != nil {
		return nil, err
	}
	profileID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	spaceID, err := parseUUIDField("space.id", req.GetSpace().GetId())
	if err != nil {
		return nil, err
	}
	resolver := voiceRoomAccessResolver(s.Store)
	if s.voiceRoomAccessResolver != nil {
		resolver = s.voiceRoomAccessResolver
	}
	access, err := resolver.ResolveVoiceRoomAccess(ctx, spaceID, voiceRoomID, profileID)
	if errors.Is(err, store.ErrVoiceRoomNotFound) {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "voice room access resolver unavailable")
	}
	return &spacev1.ResolveVoiceRoomAccessResponse{
		SpaceId:      access.SpaceID.String(),
		Member:       access.Member,
		Active:       access.Active,
		Discoverable: access.Discoverable,
		AccessEpoch:  access.AccessEpoch,
	}, nil
}
