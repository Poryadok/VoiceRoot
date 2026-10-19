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
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func purgeHandoffRequest() *spacev1.GetBotPurgeHandoffRequest {
	return &spacev1.GetBotPurgeHandoffRequest{ProtocolVersion: 1, RequestId: uuid.NewString(), SpaceId: uuid.NewString(), DeletionOperationId: uuid.NewString(), Generation: 2, Manifest: &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32), ItemCount: 1}}
}

func TestPurgeHandoffRejectsChangedBindingsAndMixedClaims(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	v := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Clock: func() time.Time { return now }}
	for _, mode := range []string{"changed-body", "protocol", "generation", "manifest", "unknown-manifest", "unknown-request", "account_id", "profile_id", "session_epoch", "account_type", "actor_profile_id", "no-replay"} {
		t.Run(mode, func(t *testing.T) {
			r := purgeHandoffRequest()
			verifier := v
			ctx := signPurgeHandoff(t, key, now, r)
			switch mode {
			case "changed-body":
				r.SpaceId = uuid.NewString()
			case "protocol":
				r.ProtocolVersion = 2
			case "generation":
				r.Generation = 0
			case "manifest":
				r.Manifest = nil
			case "unknown-manifest":
				r.Manifest.ProtoReflect().SetUnknown([]byte{0x78, 1})
			case "unknown-request":
				r.ProtoReflect().SetUnknown([]byte{0x78, 1})
			case "no-replay":
				verifier.Replay = nil
			}
			if mode != "changed-body" {
				ctx = signPurgeHandoff(t, key, now, r)
			}
			if mode == "account_id" || mode == "profile_id" || mode == "session_epoch" || mode == "account_type" || mode == "actor_profile_id" {
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
				hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
				signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
				require.NoError(t, err)
				parts[2] = base64.RawURLEncoding.EncodeToString(signature)
				ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+strings.Join(parts, "."), "x-request-id", r.RequestId))
			}
			called := false
			_, err := verifier.Unary()(ctx, r, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_GetBotPurgeHandoff_FullMethodName}, func(context.Context, any) (any, error) { called = true; return nil, nil })
			require.Error(t, err)
			require.False(t, called)
		})
	}
	r := purgeHandoffRequest()
	_, err = v.Unary()(signPurgeHandoff(t, key, now, r), r, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_GetBotPurgeHandoff_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
		p, err := RequirePurgeHandoff(ctx, r)
		require.NoError(t, err)
		_, err = RequireGlobalRemoval(ctx, globalRemovalRequest())
		require.Error(t, err, "read capability cannot become a removal grant")
		changed := proto.Clone(r).(*spacev1.GetBotPurgeHandoffRequest)
		changed.Generation++
		_, err = p.Check(changed)
		require.Error(t, err)
		now = now.Add(time.Minute)
		_, err = p.Check(r)
		require.Error(t, err, "post-SQL expiration must refuse response")
		return nil, nil
	})
	require.NoError(t, err)
}

func signPurgeHandoff(t *testing.T, key *rsa.PrivateKey, now time.Time, r *spacev1.GetBotPurgeHandoffRequest) context.Context {
	t.Helper()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	hash, err := principal.RequestHash(r)
	require.NoError(t, err)
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_GetBotPurgeHandoff_FullMethodName, RequestID: r.RequestId, RequestHash: hash})
	require.NoError(t, err)
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", r.RequestId))
}

func TestPurgeHandoffPrivateRouteRequired(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	v := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Clock: func() time.Time { return now }}
	r := purgeHandoffRequest()
	t.Run("exact protected Bot", func(t *testing.T) {
		called := false
		_, err := v.Unary()(signPurgeHandoff(t, key, now, r), r, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_GetBotPurgeHandoff_FullMethodName}, func(context.Context, any) (any, error) { called = true; return nil, nil })
		require.NoError(t, err)
		require.True(t, called)
	})
	t.Run("ordinary listener refuses", func(t *testing.T) {
		called := false
		_, err := OrdinaryUnary()(context.Background(), r, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_GetBotPurgeHandoff_FullMethodName}, func(context.Context, any) (any, error) { called = true; return nil, nil })
		require.Error(t, err)
		require.False(t, called)
	})
}
