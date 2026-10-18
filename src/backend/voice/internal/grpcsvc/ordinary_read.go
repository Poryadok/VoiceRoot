package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/voice/internal/ordinarycall"
	voicestore "voice/backend/voice/internal/store"
)

// Native lookup precedes the Redis projection: an unavailable cache cannot turn
// pending authority into absence, nor make a stale ordinary projection live.
func (s *VoiceGRPC) ordinaryActiveCall(ctx context.Context, profile, chat string) (*callsv1.GetActiveCallResponse, bool, error) {
	if s == nil || s.OrdinaryCalls == nil {
		return nil, false, nil
	}
	if s.Calls == nil || s.OrdinaryCalls.Journal == nil {
		return nil, true, ordinaryErr(ordinarycall.ErrUnavailable)
	}
	var cached voicestore.Call
	var cacheErr error
	if chat != "" {
		cached, cacheErr = s.Calls.GetActiveGroupCallForChat(ctx, chat)
	} else {
		cached, cacheErr = s.Calls.GetActiveCall(ctx, profile)
	}
	if cacheErr == nil && foreignOrdinaryResource(cached) {
		return nil, false, nil
	}
	lookup, ok := s.OrdinaryCalls.Journal.(ordinarycall.Lookup)
	if !ok {
		return nil, true, ordinaryErr(ordinarycall.ErrUnavailable)
	}
	var r *ordinarycall.Room
	var err error
	if chat != "" {
		id, e := uuid.Parse(chat)
		if e != nil || id == uuid.Nil || id.String() != chat {
			return nil, true, status.Error(codes.InvalidArgument, "invalid chat identity")
		}
		r, err = lookup.FindActiveGroupForChat(ctx, id)
	} else {
		id, e := uuid.Parse(profile)
		if e != nil || id == uuid.Nil || id.String() != profile {
			return nil, true, status.Error(codes.Unauthenticated, "invalid profile identity")
		}
		r, err = lookup.FindForProfile(ctx, id)
	}
	if errors.Is(err, ordinarycall.ErrNotFound) {
		if cacheErr == nil || !errors.Is(cacheErr, voicestore.ErrNotFound) {
			return nil, true, ordinaryErr(ordinarycall.ErrUnavailable)
		}
		return nil, true, ordinaryErr(err)
	}
	if err != nil {
		return nil, true, ordinaryErr(err)
	}
	if chat == "" {
		if _, err = s.ordinaryActor(ctx, r, profile); err != nil {
			return nil, true, err
		}
	} else {
		actor, e := s.ordinaryIdentity(ctx, profile, true)
		if e != nil {
			return nil, true, e
		}
		if e = s.requireOrdinaryEpoch(ctx, actor); e != nil {
			return nil, true, e
		}
	}
	call, err := s.ordinaryProjectedCall(ctx, r)
	if err != nil {
		return nil, true, ordinaryErr(err)
	}
	return &callsv1.GetActiveCallResponse{CallSession: callToProto(call)}, true, nil
}
