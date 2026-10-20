package botmembershipprincipal

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func signNativePurgeInstallation(t *testing.T, key *rsa.PrivateKey, now time.Time, r *spacev1.ReconcileBotPurgeInstallationRequest) context.Context {
	t.Helper()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	hash, err := principal.RequestHash(r)
	require.NoError(t, err)
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_ReconcileBotPurgeInstallation_FullMethodName, RequestID: r.OperationId, RequestHash: hash})
	require.NoError(t, err)
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", r.OperationId))
}

func TestNativePurgeInstallationPureClaimsAndFinalBinding(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	v := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Clock: func() time.Time { return now }}
	for _, mode := range []string{"account_id", "profile_id", "session_epoch", "account_type", "actor_profile_id", "bot_id", "space_id", "installation_id", "bot_scope", "changed-body", "unknown", "protocol", "bad-uuid", "wrong-method", "no-replay", "dependency", "unknown-native", "unknown-intent", "unknown-manifest", "wrong-installation", "wrong-space", "wrong-bot", "parent-as-child", "cancel-as-child", "zero-generation", "nil-native"} {
		t.Run(mode, func(t *testing.T) {
			r := nativePurgeRequest()
			ctx := signNativePurgeInstallation(t, key, now, r)
			verifier := v
			switch mode {
			case "changed-body":
				r.BotId = uuid.NewString()
			case "unknown":
				r.ProtoReflect().SetUnknown([]byte{0x78, 1})
				ctx = signNativePurgeInstallation(t, key, now, r)
			case "protocol":
				r.ProtocolVersion = 2
				ctx = signNativePurgeInstallation(t, key, now, r)
			case "bad-uuid":
				r.OperationId = "not-a-uuid"
				ctx = signNativePurgeInstallation(t, key, now, r)
			case "unknown-native":
				r.NativeCancellation.ProtoReflect().SetUnknown([]byte{0x78, 1})
				ctx = signNativePurgeInstallation(t, key, now, r)
			case "unknown-intent":
				r.NativeCancellation.InstallationIntent.ProtoReflect().SetUnknown([]byte{0x78, 1})
				ctx = signNativePurgeInstallation(t, key, now, r)
			case "unknown-manifest":
				r.Manifest.ProtoReflect().SetUnknown([]byte{0x78, 1})
				ctx = signNativePurgeInstallation(t, key, now, r)
			case "wrong-installation":
				r.InstallationId = uuid.NewString()
				ctx = signNativePurgeInstallation(t, key, now, r)
			case "wrong-space":
				r.SpaceId = uuid.NewString()
				ctx = signNativePurgeInstallation(t, key, now, r)
			case "wrong-bot":
				r.BotId = uuid.NewString()
				ctx = signNativePurgeInstallation(t, key, now, r)
			case "parent-as-child":
				r.OperationId = r.DeletionOperationId
				ctx = signNativePurgeInstallation(t, key, now, r)
			case "cancel-as-child":
				r.OperationId = r.NativeCancellation.InstallationIntent.OperationId
				ctx = signNativePurgeInstallation(t, key, now, r)
			case "zero-generation":
				r.Generation = 0
				ctx = signNativePurgeInstallation(t, key, now, r)
			case "nil-native":
				r.NativeCancellation = nil
				ctx = signNativePurgeInstallation(t, key, now, r)
			case "no-replay":
				verifier.Replay = nil
			case "dependency":
				verifier.Keys = func(context.Context, string, string) (*rsa.PublicKey, error) {
					return nil, &dependencyUnavailable{cause: context.DeadlineExceeded}
				}
			default:
				if mode != "wrong-method" {
					md, _ := metadata.FromIncomingContext(ctx)
					parts := strings.Split(strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), ".")
					raw, err := base64.RawURLEncoding.DecodeString(parts[1])
					require.NoError(t, err)
					var claims map[string]any
					require.NoError(t, json.Unmarshal(raw, &claims))
					claims[mode] = uuid.NewString()
					if mode == "session_epoch" {
						claims[mode] = 7
					}
					if mode == "account_type" {
						claims[mode] = "regular"
					}
					raw, err = json.Marshal(claims)
					require.NoError(t, err)
					parts[1] = base64.RawURLEncoding.EncodeToString(raw)
					digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
					signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
					require.NoError(t, err)
					parts[2] = base64.RawURLEncoding.EncodeToString(signature)
					ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+strings.Join(parts, "."), "x-request-id", r.OperationId))
				}
			}
			method := spacev1.SpaceService_ReconcileBotPurgeInstallation_FullMethodName
			if mode == "wrong-method" {
				method = spacev1.SpaceService_ApplyBotInstallationRemoval_FullMethodName
			}
			called := false
			_, err := verifier.Unary()(ctx, r, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) { called = true; return nil, nil })
			require.Error(t, err)
			require.False(t, called)
		})
	}
	r := nativePurgeRequest()
	_, err = v.Unary()(signNativePurgeInstallation(t, key, now, r), r, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_ReconcileBotPurgeInstallation_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
		p, err := RequirePurgeInstallation(ctx, r)
		require.NoError(t, err)
		_, err = RequireGlobalRemoval(ctx, globalRemovalRequest())
		require.Error(t, err)
		changed := proto.Clone(r).(*spacev1.ReconcileBotPurgeInstallationRequest)
		changed.OperationId = uuid.NewString()
		_, err = p.Check(changed)
		require.Error(t, err)
		now = now.Add(time.Minute)
		_, err = p.Check(r)
		require.Error(t, err, "original private credential final deadline is retained")
		return nil, nil
	})
	require.NoError(t, err)
}
