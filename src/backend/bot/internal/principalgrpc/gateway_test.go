package principalgrpc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/pkg/principal"
)

func TestGatewayVerifierRequiresFreshEpochReplayAndExactLifecycleBody(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "gateway-current", PrivateKey: key})
	require.NoError(t, err)
	seen := map[string]bool{}
	reads := 0
	floor := int64(7)
	outage := false
	verifier := GatewayUserVerifier{
		Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil },
		Replay: func(_ context.Context, _ string, jti string, _ time.Time) error {
			if seen[jti] {
				return errors.New("replay")
			}
			seen[jti] = true
			return nil
		},
		Epoch: func(_ context.Context, account string, epoch int64) error {
			reads++
			if outage {
				return errors.New("unavailable")
			}
			if account != "00000000-0000-0000-0000-000000000247" || epoch < floor {
				return errors.New("stale")
			}
			return nil
		},
	}
	request := &botv1.DeleteBotRequest{BotId: "00000000-0000-0000-0000-000000000170"}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	method := botv1.BotService_DeleteBot_FullMethodName
	sign := func(epoch int64) string {
		token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "bot", RPC: method, RequestID: "delete-request", RequestHash: hash, AccountID: "00000000-0000-0000-0000-000000000247", ProfileID: "00000000-0000-0000-0000-000000000248", SessionEpoch: epoch, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
		require.NoError(t, err)
		return token
	}
	token := sign(7)
	verified, err := verifier.VerifyGateway(t.Context(), token, method, "delete-request", hash)
	require.NoError(t, err)
	require.Equal(t, int64(7), verified.SessionEpoch)
	require.Equal(t, 1, reads)
	_, err = verifier.VerifyGateway(t.Context(), token, method, "delete-request", hash)
	require.Error(t, err)
	floor = 8
	_, err = verifier.VerifyGateway(t.Context(), sign(7), method, "delete-request", hash)
	require.Error(t, err)
	_, err = verifier.VerifyGateway(t.Context(), sign(8), method, "delete-request", hash)
	require.NoError(t, err)
	outage = true
	_, err = verifier.VerifyGateway(t.Context(), sign(8), method, "delete-request", hash)
	require.Error(t, err)
	_, err = verifier.VerifyGateway(t.Context(), sign(8), botv1.BotService_UpdateBot_FullMethodName, "delete-request", hash)
	require.Error(t, err)
	_, err = verifier.VerifyGateway(t.Context(), sign(8), method, "delete-request", "sha256:0000000000000000000000000000000000000000000000000000000000000000")
	require.Error(t, err)
	verifier.Replay = nil
	_, err = verifier.VerifyGateway(t.Context(), sign(8), method, "delete-request", hash)
	require.Error(t, err)
}
