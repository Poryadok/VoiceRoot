// This file is copied into the pinned upstream pkg/service test package.
// It probes the accepted MatchSquad contract; stock v1.8.4 is expected RED.
package service

import (
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"github.com/stretchr/testify/require"

	"github.com/livekit/livekit-server/pkg/config"
	"github.com/livekit/livekit-server/pkg/rtc/types/typesfakes"
)

const expiryProbeSecret = "isolated-expiry-fixture-not-a-deployment-key"

func expiryProbeParticipant(room, identity string) *typesfakes.FakeLocalParticipant {
	p := &typesfakes.FakeLocalParticipant{}
	p.IdentityReturns(livekit.ParticipantIdentity(identity))
	p.ClaimGrantsReturns(&auth.ClaimGrants{Identity: identity, Video: &auth.VideoGrant{Room: room, RoomJoin: true}})
	return p
}

func TestVoiceMatchSquadMissingVerifiedExpiryMustNotRefresh(t *testing.T) {
	room := "match-squad-11111111-1111-4111-8111-111111111111"
	identity := "ms:22222222-2222-4222-8222-222222222222:33333333-3333-4333-8333-333333333333"
	manager := &RoomManager{config: &config.Config{Keys: map[string]string{"fixture": expiryProbeSecret}}}
	for _, spoof := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "caller_attribute_is_not_verified_expiry"}[spoof], func(t *testing.T) {
			p := expiryProbeParticipant(room, identity)
			if spoof {
				p.ClaimGrantsReturns(&auth.ClaimGrants{Identity: identity, Video: &auth.VideoGrant{Room: room, RoomJoin: true}, Attributes: map[string]string{"voice_original_exp": "9999999999"}, Metadata: `{"voice_original_exp":9999999999}`})
			}
			err := manager.refreshToken(p)
			require.Error(t, err, "MatchSquad refresh needs retained verified original JWT expiry and current Voice authority")
			require.Zero(t, p.SendRefreshTokenCallCount(), "no bearer may be emitted from a caller expiry assertion")
		})
	}
}

func TestVoiceExpiryProbeOrdinaryRefreshKeepsStockBehavior(t *testing.T) {
	manager := &RoomManager{config: &config.Config{Keys: map[string]string{"fixture": expiryProbeSecret}}}
	p := expiryProbeParticipant("ordinary-room", "ordinary-user")
	before := time.Now()
	require.NoError(t, manager.refreshToken(p))
	require.Equal(t, 1, p.SendRefreshTokenCallCount())
	token, err := jwt.ParseSigned(p.SendRefreshTokenArgsForCall(0))
	require.NoError(t, err)
	claims := jwt.Claims{}
	require.NoError(t, token.Claims([]byte(expiryProbeSecret), &claims))
	require.NotNil(t, claims.Expiry)
	// Compare bounds, never log the bearer or include it in assertion messages.
	require.WithinDuration(t, before.Add(tokenDefaultTTL), claims.Expiry.Time(), 2*time.Second)
}
