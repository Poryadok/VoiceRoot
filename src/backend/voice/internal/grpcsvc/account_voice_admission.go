package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/voice/internal/authctx"
	"voice/backend/voice/internal/gameprovision"
	voicestore "voice/backend/voice/internal/store"
)

type accountVoiceReservation struct {
	accountID uuid.UUID
	profileID uuid.UUID
	roomID    string
	created   bool
}

func (s *VoiceGRPC) reserveAccountVoiceProfiles(ctx context.Context, roomID string, profiles []string, authenticatedProfile string) ([]accountVoiceReservation, error) {
	if s == nil || s.AccountVoiceFences == nil {
		return nil, nil
	}
	if s.AccountVoiceProfiles == nil {
		return nil, status.Error(codes.Unavailable, "account voice authority unavailable")
	}
	reservations := make([]accountVoiceReservation, 0, len(profiles))
	for _, profile := range profiles {
		profileUUID, err := uuid.Parse(profile)
		if err != nil || profileUUID == uuid.Nil {
			s.releaseAccountVoiceReservations(ctx, reservations)
			return nil, status.Error(codes.InvalidArgument, "invalid voice profile identity")
		}
		accountID, err := s.AccountVoiceProfiles.AccountIDByProfileID(ctx, profileUUID)
		if err != nil {
			s.releaseAccountVoiceReservations(ctx, reservations)
			if status.Code(err) == codes.NotFound {
				return nil, status.Error(codes.PermissionDenied, "voice profile has no trusted account")
			}
			return nil, status.Error(codes.Unavailable, "trusted profile account lookup unavailable")
		}
		if profile == authenticatedProfile {
			claimedAccount, ok := authctx.AccountID(ctx)
			parsedAccount, parseErr := uuid.Parse(claimedAccount)
			if !ok || parseErr != nil || parsedAccount != accountID {
				s.releaseAccountVoiceReservations(ctx, reservations)
				return nil, status.Error(codes.PermissionDenied, "authenticated account does not own voice profile")
			}
		}
		created, err := s.AccountVoiceFences.Reserve(ctx, accountID, profileUUID, roomID)
		if err != nil {
			s.releaseAccountVoiceReservations(ctx, reservations)
			switch {
			case errors.Is(err, gameprovision.ErrAccountProfileMappingConflict):
				return nil, status.Error(codes.PermissionDenied, "trusted profile account mapping changed")
			case errors.Is(err, gameprovision.ErrActiveAccountVoiceSession):
				return nil, status.Error(codes.FailedPrecondition, "account already has an active Voice session")
			default:
				return nil, status.Error(codes.Unavailable, "account voice reservation unavailable")
			}
		}
		reservations = append(reservations, accountVoiceReservation{accountID: accountID, profileID: profileUUID, roomID: roomID, created: created})
	}
	return reservations, nil
}

func (s *VoiceGRPC) commitAccountVoiceReservations(ctx context.Context, reservations []accountVoiceReservation) error {
	for _, reservation := range reservations {
		if err := s.AccountVoiceFences.Commit(ctx, reservation.accountID, reservation.profileID, reservation.roomID); err != nil {
			return status.Error(codes.Unavailable, "account voice reservation commit unavailable")
		}
	}
	return nil
}

func (s *VoiceGRPC) releaseAccountVoiceReservations(ctx context.Context, reservations []accountVoiceReservation) {
	if s == nil || s.AccountVoiceFences == nil {
		return
	}
	for _, reservation := range reservations {
		if reservation.created {
			_ = s.AccountVoiceFences.Release(ctx, reservation.accountID, reservation.profileID, reservation.roomID)
		}
	}
}

func (s *VoiceGRPC) releaseAccountVoiceProfile(ctx context.Context, profileID, roomID string) error {
	if s == nil || s.AccountVoiceFences == nil {
		return nil
	}
	profileUUID, err := uuid.Parse(profileID)
	if err != nil || profileUUID == uuid.Nil || s.AccountVoiceProfiles == nil {
		return status.Error(codes.Unavailable, "account voice authority unavailable")
	}
	accountID, err := s.AccountVoiceProfiles.AccountIDByProfileID(ctx, profileUUID)
	if err != nil {
		return status.Error(codes.Unavailable, "trusted profile account lookup unavailable")
	}
	if err := s.AccountVoiceFences.Release(ctx, accountID, profileUUID, roomID); err != nil {
		return status.Error(codes.Unavailable, "account voice fence release unavailable")
	}
	return nil
}

func (s *VoiceGRPC) releaseAccountVoiceCall(ctx context.Context, call voicestore.Call) error {
	for _, profileID := range call.ProfileIDs() {
		if err := s.releaseAccountVoiceProfile(ctx, profileID, call.RoomID); err != nil {
			return err
		}
	}
	return nil
}

func (s *VoiceGRPC) transferAccountVoiceProfile(ctx context.Context, profileID, fromRoom, toRoom, authenticatedProfile string) error {
	if s == nil || s.AccountVoiceFences == nil {
		return nil
	}
	if s.AccountVoiceProfiles == nil {
		return status.Error(codes.Unavailable, "account voice authority unavailable")
	}
	profileUUID, err := uuid.Parse(profileID)
	if err != nil || profileUUID == uuid.Nil {
		return status.Error(codes.InvalidArgument, "invalid voice profile identity")
	}
	accountID, err := s.AccountVoiceProfiles.AccountIDByProfileID(ctx, profileUUID)
	if err != nil {
		return status.Error(codes.Unavailable, "trusted profile account lookup unavailable")
	}
	if profileID == authenticatedProfile {
		claimed, ok := authctx.AccountID(ctx)
		parsed, parseErr := uuid.Parse(claimed)
		if !ok || parseErr != nil || parsed != accountID {
			return status.Error(codes.PermissionDenied, "authenticated account does not own voice profile")
		}
	}
	if err := s.AccountVoiceFences.Transfer(ctx, accountID, profileUUID, fromRoom, toRoom); err != nil {
		switch {
		case errors.Is(err, gameprovision.ErrAccountProfileMappingConflict):
			return status.Error(codes.PermissionDenied, "trusted profile account mapping changed")
		case errors.Is(err, gameprovision.ErrActiveAccountVoiceSession):
			return status.Error(codes.FailedPrecondition, "account voice transfer conflicts with its active session")
		default:
			return status.Error(codes.Unavailable, "account voice transfer unavailable")
		}
	}
	return nil
}
