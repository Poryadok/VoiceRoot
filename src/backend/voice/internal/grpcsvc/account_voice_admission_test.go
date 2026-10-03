package grpcsvc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/voice/internal/authctx"
	"voice/backend/voice/internal/gameprovision"
)

type accountResolverStub map[uuid.UUID]uuid.UUID

func (s accountResolverStub) AccountIDByProfileID(_ context.Context, profile uuid.UUID) (uuid.UUID, error) {
	account, ok := s[profile]
	if !ok {
		return uuid.Nil, status.Error(codes.NotFound, "missing")
	}
	return account, nil
}

type accountFenceStub struct {
	mappings map[uuid.UUID]uuid.UUID
	active   map[uuid.UUID]accountVoiceReservation
}

func (s *accountFenceStub) Reserve(_ context.Context, account, profile uuid.UUID, room string) (bool, error) {
	if mapped, ok := s.mappings[profile]; ok && mapped != account {
		return false, gameprovision.ErrAccountProfileMappingConflict
	}
	s.mappings[profile] = account
	if active, ok := s.active[account]; ok {
		if active.profileID == profile && active.roomID == room {
			return false, nil
		}
		return false, gameprovision.ErrActiveAccountVoiceSession
	}
	s.active[account] = accountVoiceReservation{accountID: account, profileID: profile, roomID: room, created: true}
	return true, nil
}

func (s *accountFenceStub) Commit(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }

func (s *accountFenceStub) Release(_ context.Context, account, profile uuid.UUID, room string) error {
	active, ok := s.active[account]
	if !ok || active.profileID != profile || active.roomID != room {
		return errors.New("fence tuple mismatch")
	}
	delete(s.active, account)
	return nil
}

func (s *accountFenceStub) Transfer(_ context.Context, account, profile uuid.UUID, fromRoom, toRoom string) error {
	active, ok := s.active[account]
	if !ok || active.profileID != profile || active.roomID != fromRoom {
		return gameprovision.ErrActiveAccountVoiceSession
	}
	active.roomID = toRoom
	s.active[account] = active
	return nil
}

func TestAccountVoiceAdmissionBindsAuthenticatedAccountAndFencesOtherProfiles(t *testing.T) {
	accountA, accountB := uuid.New(), uuid.New()
	profileA, profileB, profileC := uuid.New(), uuid.New(), uuid.New()
	fences := &accountFenceStub{mappings: map[uuid.UUID]uuid.UUID{}, active: map[uuid.UUID]accountVoiceReservation{}}
	service := &VoiceGRPC{
		AccountVoiceFences: fences,
		AccountVoiceProfiles: accountResolverStub{
			profileA: accountA,
			profileB: accountA,
			profileC: accountB,
		},
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderAccountID, accountA.String()))
	reservations, err := service.reserveAccountVoiceProfiles(ctx, "room-a", []string{profileA.String()}, profileA.String())
	require.NoError(t, err)
	require.NoError(t, service.commitAccountVoiceReservations(ctx, reservations))

	_, err = service.reserveAccountVoiceProfiles(ctx, "room-b", []string{profileB.String()}, profileB.String())
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "another profile on the same account must be fenced")

	service.releaseAccountVoiceReservations(ctx, reservations)
	reservations, err = service.reserveAccountVoiceProfiles(ctx, "room-b", []string{profileB.String()}, profileB.String())
	require.NoError(t, err, "explicit release permits a later profile session")
	service.releaseAccountVoiceReservations(ctx, reservations)

	wrongAccount := metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderAccountID, accountB.String()))
	_, err = service.reserveAccountVoiceProfiles(wrongAccount, "room-c", []string{profileA.String()}, profileA.String())
	require.Equal(t, codes.PermissionDenied, status.Code(err), "client claims cannot override User profile ownership")
}

func TestAccountVoiceAdmissionFailsClosedWithoutDurableStore(t *testing.T) {
	profileID, accountID := uuid.New(), uuid.New()
	service := &VoiceGRPC{
		AccountVoiceFences: gameprovision.UnavailableAccountVoiceFenceStore{},
		AccountVoiceProfiles: accountResolverStub{
			profileID: accountID,
		},
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderAccountID, accountID.String()))
	_, err := service.reserveAccountVoiceProfiles(ctx, "room", []string{profileID.String()}, profileID.String())
	require.Equal(t, codes.Unavailable, status.Code(err))
}
