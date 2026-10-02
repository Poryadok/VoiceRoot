package gameconsent

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	socialv1 "voice.app/voice/social/v1"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/notification/internal/s2s"
)

type ProfileAccountReader interface {
	GetProfile(context.Context, *userv1.GetProfileRequest, ...grpc.CallOption) (*userv1.GetProfileResponse, error)
}
type AccountBlockReader interface {
	IsBlocked(context.Context, *socialv1.IsBlockedRequest, ...grpc.CallOption) (*socialv1.IsBlockedResponse, error)
}

// BlockChecker evaluates both directed account blocks for a game message's
// sender and recipient profiles immediately before each device push.
type BlockChecker struct {
	Profiles ProfileAccountReader
	Blocks   AccountBlockReader
}

func (b *BlockChecker) IsGamePushBlocked(ctx context.Context, recipientProfileID, senderProfileID uuid.UUID) (bool, error) {
	if b == nil || b.Profiles == nil || b.Blocks == nil || recipientProfileID == uuid.Nil || senderProfileID == uuid.Nil {
		return false, ErrUnavailable
	}
	recipient, err := b.profileAccount(ctx, recipientProfileID)
	if err != nil {
		return false, err
	}
	sender, err := b.profileAccount(ctx, senderProfileID)
	if err != nil {
		return false, err
	}
	if recipient == sender {
		return false, nil
	}
	for _, pair := range [][2]uuid.UUID{{recipient, sender}, {sender, recipient}} {
		response, err := b.Blocks.IsBlocked(s2s.Context(ctx), &socialv1.IsBlockedRequest{AccountIdA: pair[0].String(), AccountIdB: pair[1].String()})
		if err != nil || response == nil {
			return false, fmt.Errorf("%w: account block authority failed", ErrUnavailable)
		}
		if response.GetBlocked() {
			return true, nil
		}
	}
	return false, nil
}

func (b *BlockChecker) profileAccount(ctx context.Context, profileID uuid.UUID) (uuid.UUID, error) {
	response, err := b.Profiles.GetProfile(s2s.Context(ctx), &userv1.GetProfileRequest{By: &userv1.GetProfileRequest_ProfileId{ProfileId: profileID.String()}})
	if err != nil || response == nil || response.GetProfile() == nil {
		return uuid.Nil, fmt.Errorf("%w: profile account authority failed", ErrUnavailable)
	}
	profile := response.GetProfile()
	accountID, parseErr := uuid.Parse(profile.GetAccountId())
	if parseErr != nil || accountID == uuid.Nil || profile.GetId() != profileID.String() {
		return uuid.Nil, errors.Join(ErrUnavailable, errors.New("profile account response is invalid"))
	}
	return accountID, nil
}
