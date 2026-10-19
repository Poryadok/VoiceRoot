package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/privacy"
	"voice/backend/space/internal/authctx"
)

func TestLegacyAdmissionDispatchRequiresPrivateCallBeforeDependencies(t *testing.T) {
	// Raw profile/account metadata cannot select a Space mutation path, even
	// when persistence/owner clients are absent. No database fixture is needed
	// to establish this authentication boundary.
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderUserID, uuid.NewString(), authctx.HeaderProfileID, uuid.NewString(), authctx.HeaderSessionEpoch, "7"))
	s := &SpaceGRPC{}
	_, err := s.JoinSpace(ctx, &spacev1.JoinSpaceRequest{SpaceId: uuid.NewString()})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = s.JoinByInvite(ctx, &spacev1.JoinByInviteRequest{Code: "exact-invite"})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestLegacyAdmissionDispatchInviterPrivacyDoesNotGuessType(t *testing.T) {
	for _, header := range []string{"", "regular", "guest"} {
		t.Run("raw_type_"+header, func(t *testing.T) {
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-account-type", header))
			recipient, inviter := uuid.New(), uuid.New()
			reader := &legacyNativePrivacy{privacy.FriendsOnly()}
			s := &SpaceGRPC{Privacy: reader}
			require.Equal(t, codes.Unavailable, status.Code(s.ensureLegacyInvitePrivacy(ctx, recipient, inviter)), "no qualified inviter type; raw/applicant type cannot authorize Friends")
			reader.audience = privacy.Nobody()
			require.Equal(t, codes.PermissionDenied, status.Code(s.ensureLegacyInvitePrivacy(ctx, recipient, inviter)))
			reader.audience = privacy.EveryoneWithGuests()
			require.NoError(t, s.ensureLegacyInvitePrivacy(ctx, recipient, inviter))
			s.Privacy = nil
			require.Equal(t, codes.Unavailable, status.Code(s.ensureLegacyInvitePrivacy(ctx, recipient, inviter)))
		})
	}
}
