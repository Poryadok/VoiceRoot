package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	userv1 "voice.app/voice/user/v1"
)

// The ordinary User listener must never interpret an arbitrary metadata value
// as a Social principal. The protected listener implementation is intentionally
// absent in this RED-only branch.
func TestGetPrivacySettings_RawSocialCallerIsUnauthenticated(t *testing.T) {
	req := &userv1.GetPrivacySettingsRequest{ProfileId: uuid.NewString()}

	for _, metadataPairs := range [][]string{
		{"x-voice-internal-caller", "social"},
		{"x-voice-internal-caller", "social", "x-voice-internal-caller", "social"},
		{"authorization", "Bearer forged", "x-voice-internal-caller", "social"},
	} {
		t.Run("raw metadata is not proof", func(t *testing.T) {
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(metadataPairs...))
			_, err := (&UserGRPC{}).GetPrivacySettings(ctx, req)
			require.Equal(t, codes.Unauthenticated, status.Code(err))
		})
	}
}
