package grpcsvc

import (
	"context"
	"errors"
	"reflect"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/botmembershipprincipal"
)

// BotMembershipStore accepts only the private installation capability minted
// by the dedicated verifier, never a human identity or raw membership UUIDs.
type BotMembershipStore interface {
	AddBotMemberWithInstallation(context.Context, *botmembershipprincipal.VerifiedInstallation, *spacev1.AddBotMemberRequest) error
	RemoveBotMemberWithInstallation(context.Context, *botmembershipprincipal.VerifiedInstallation, *spacev1.RemoveBotMemberRequest) error
}

// BotMembershipProtected exposes only the two installation operations. It must
// be registered with botmembershipprincipal.Runtime.ServerOptions; all other
// generated methods remain unimplemented on this listener.
type BotMembershipProtected struct {
	spacev1.UnimplementedSpaceServiceServer
	store BotMembershipStore
}

func NewBotMembershipProtected(persistence BotMembershipStore) (*BotMembershipProtected, error) {
	if persistence == nil || (reflect.ValueOf(persistence).Kind() == reflect.Ptr && reflect.ValueOf(persistence).IsNil()) {
		return nil, errors.New("bot membership persistence is required")
	}
	return &BotMembershipProtected{store: persistence}, nil
}

func (s *BotMembershipProtected) AddBotMember(ctx context.Context, request *spacev1.AddBotMemberRequest) (*spacev1.AddBotMemberResponse, error) {
	proof, err := botmembershipprincipal.RequireAdd(ctx, request)
	if err != nil {
		return nil, err
	}
	if s == nil || s.store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	// The typed store owns operation-before-Space lock ordering. Taking the
	// legacy handler's outer Space lock here would invert that order.
	if err := s.store.AddBotMemberWithInstallation(ctx, proof, request); err != nil {
		return nil, mapSpaceStoreError(err)
	}
	return &spacev1.AddBotMemberResponse{}, nil
}

func (s *BotMembershipProtected) RemoveBotMember(ctx context.Context, request *spacev1.RemoveBotMemberRequest) (*spacev1.RemoveBotMemberResponse, error) {
	proof, err := botmembershipprincipal.RequireRemove(ctx, request)
	if err != nil {
		return nil, err
	}
	if s == nil || s.store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	if err := s.store.RemoveBotMemberWithInstallation(ctx, proof, request); err != nil {
		return nil, mapSpaceStoreError(err)
	}
	return &spacev1.RemoveBotMemberResponse{}, nil
}
