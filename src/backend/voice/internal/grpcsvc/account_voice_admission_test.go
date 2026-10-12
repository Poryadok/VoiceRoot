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

	"voice/backend/pkg/principal"
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

func TestAccountVoiceFencesUseVerifiedDelegatedIdentityBeforeLegacyMetadata(t *testing.T) {
	account, otherAccount, profile, otherProfile := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	valid := principal.Principal{Kind: "delegated_user", Issuer: "gateway", Audience: "voice",
		AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 7}
	for _, tc := range []struct {
		name       string
		metadataID string
		change     func(*principal.Principal)
		allowed    bool
	}{
		{name: "delegated without legacy headers", allowed: true},
		{name: "verified identity overrides conflicting header", metadataID: otherAccount.String(), allowed: true},
		{name: "header cannot repair wrong account", metadataID: account.String(), change: func(p *principal.Principal) { p.AccountID = otherAccount.String() }},
		{name: "same account cannot impersonate another profile", metadataID: account.String(), change: func(p *principal.Principal) { p.ProfileID = otherProfile.String() }},
		{name: "service principal cannot use legacy identity", metadataID: account.String(), change: func(p *principal.Principal) { p.Kind = "service" }},
		{name: "invalid delegated identity cannot fall back", metadataID: account.String(), change: func(p *principal.Principal) { p.SessionEpoch = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			verified := valid
			if tc.change != nil {
				tc.change(&verified)
			}
			ctx := context.Background()
			if tc.metadataID != "" {
				ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(authctx.HeaderAccountID, tc.metadataID))
			}
			ctx = principal.WithVerified(ctx, verified)
			fences := &accountFenceStub{mappings: map[uuid.UUID]uuid.UUID{}, active: map[uuid.UUID]accountVoiceReservation{}}
			service := &VoiceGRPC{AccountVoiceFences: fences, AccountVoiceProfiles: accountResolverStub{profile: account}}
			reservations, err := service.reserveAccountVoiceProfiles(ctx, "room-a", []string{profile.String()}, profile.String())
			if tc.allowed {
				require.NoError(t, err)
				require.Len(t, reservations, 1)
				require.Equal(t, account, reservations[0].accountID)
			} else {
				require.Equal(t, codes.PermissionDenied, status.Code(err))
				require.Empty(t, fences.active)
				_, reserveErr := fences.Reserve(context.Background(), account, profile, "room-a")
				require.NoError(t, reserveErr)
			}
			err = service.transferAccountVoiceProfile(ctx, profile.String(), "room-a", "room-b", profile.String())
			if tc.allowed {
				require.NoError(t, err)
				require.Equal(t, "room-b", fences.active[account].roomID)
			} else {
				require.Equal(t, codes.PermissionDenied, status.Code(err))
				require.Equal(t, "room-a", fences.active[account].roomID)
			}
		})
	}
}
