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

func globalRemovalRequest() *spacev1.ApplyBotInstallationRemovalRequest {
	nested, _ := installationRequest(true)
	cancel := nested.(*spacev1.RemoveBotMemberRequest)
	root, frozen := sha256.Sum256([]byte(uuid.NewString())), sha256.Sum256([]byte(uuid.NewString()))
	return &spacev1.ApplyBotInstallationRemovalRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), DecisionId: uuid.NewString(), DecisionSha256: root[:], Purpose: "global_delete", BotId: cancel.InstallationIntent.BotId, ActorProfileId: cancel.ProfileId, SpaceId: cancel.SpaceId, NativeCancellation: cancel, FrozenScopeSha256: frozen[:]}
}

func TestGlobalRemovalRejectsSignedHumanCredentialFields(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	v := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Clock: func() time.Time { return now }}
	for _, field := range []string{"account_id", "profile_id", "session_epoch"} {
		t.Run(field, func(t *testing.T) {
			r := globalRemovalRequest()
			ctx := signGlobalRemoval(t, key, now, r)
			md, _ := metadata.FromIncomingContext(ctx)
			parts := strings.Split(strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), ".")
			raw, err := base64.RawURLEncoding.DecodeString(parts[1])
			require.NoError(t, err)
			var claims map[string]any
			require.NoError(t, json.Unmarshal(raw, &claims))
			if field == "session_epoch" {
				claims[field] = 7
			} else {
				claims[field] = uuid.NewString()
			}
			raw, err = json.Marshal(claims)
			require.NoError(t, err)
			parts[1] = base64.RawURLEncoding.EncodeToString(raw)
			hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
			signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
			require.NoError(t, err)
			parts[2] = base64.RawURLEncoding.EncodeToString(signature)
			ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+strings.Join(parts, "."), "x-request-id", r.OperationId))
			called := false
			_, err = v.Unary()(ctx, r, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_ApplyBotInstallationRemoval_FullMethodName}, func(context.Context, any) (any, error) { called = true; return nil, nil })
			require.Error(t, err, "pure service credential cannot carry an ambient human identity")
			require.False(t, called)
		})
	}
}

func signGlobalRemoval(t *testing.T, key *rsa.PrivateKey, now time.Time, r *spacev1.ApplyBotInstallationRemovalRequest) context.Context {
	t.Helper()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	hash, err := principal.RequestHash(r)
	require.NoError(t, err)
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_ApplyBotInstallationRemoval_FullMethodName, RequestID: r.OperationId, RequestHash: hash})
	require.NoError(t, err)
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", r.OperationId))
}

func TestGlobalRemovalPrincipalAdmitsOnlyExactNativeContract(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	v := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Clock: func() time.Time { return now }}
	for _, mode := range []string{"exact", "changed-root", "purpose", "protocol", "outer-actor", "outer-bot", "outer-space", "unknown-outer", "unknown-cancel", "unknown-intent", "missing-cancel", "original-add", "zero-root-hash", "no-replay"} {
		t.Run(mode, func(t *testing.T) {
			r := globalRemovalRequest()
			ctx := signGlobalRemoval(t, key, now, r)
			copyVerifier := v
			switch mode {
			case "changed-root":
				r.DecisionId = uuid.NewString()
			case "purpose":
				r.Purpose = "uninstall"
			case "protocol":
				r.ProtocolVersion = 2
			case "outer-actor":
				r.ActorProfileId = uuid.NewString()
			case "outer-bot":
				r.BotId = uuid.NewString()
			case "outer-space":
				r.SpaceId = uuid.NewString()
			case "unknown-outer":
				r.ProtoReflect().SetUnknown([]byte{0x78, 0x01})
			case "unknown-cancel":
				r.NativeCancellation.ProtoReflect().SetUnknown([]byte{0x78, 0x01})
			case "unknown-intent":
				r.NativeCancellation.InstallationIntent.ProtoReflect().SetUnknown([]byte{0x78, 0x01})
			case "missing-cancel":
				r.NativeCancellation = nil
			case "original-add":
				r.NativeCancellation.InstallOperationId = r.NativeCancellation.InstallationIntent.OperationId
			case "zero-root-hash":
				r.DecisionSha256 = nil
			case "no-replay":
				copyVerifier.Replay = nil
			}
			// Structural negatives are freshly signed, proving the verifier
			// rejects their actual tuple rather than only a mismatched old hash.
			if mode != "changed-root" {
				ctx = signGlobalRemoval(t, key, now, r)
			}
			called := false
			_, err := copyVerifier.Unary()(ctx, r, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_ApplyBotInstallationRemoval_FullMethodName}, func(context.Context, any) (any, error) { called = true; return nil, nil })
			if mode == "exact" {
				require.NoError(t, err)
				require.True(t, called)
			} else {
				require.Error(t, err)
				require.False(t, called)
			}
		})
	}
}

func TestGlobalRemovalCapabilityRemainsDistinctAndExpires(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	v := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Clock: func() time.Time { return now }}
	r := globalRemovalRequest()
	_, err = RequireGlobalRemoval(context.Background(), r)
	require.Error(t, err)
	_, err = v.Unary()(signGlobalRemoval(t, key, now, r), r, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_ApplyBotInstallationRemoval_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
		p, err := RequireGlobalRemoval(ctx, r)
		require.NoError(t, err)
		_, err = p.Check(r)
		require.NoError(t, err)
		_, err = RequireRemove(ctx, r.NativeCancellation)
		require.Error(t, err, "global capability must not manufacture a native-method capability")
		changed := proto.Clone(r).(*spacev1.ApplyBotInstallationRemovalRequest)
		changed.NativeCancellation.InstallationIntent.Generation++
		_, err = p.Check(changed)
		require.Error(t, err)
		now = now.Add(time.Minute)
		_, err = p.Check(r)
		require.Error(t, err)
		return nil, nil
	})
	require.NoError(t, err)
	_, err = OrdinaryUnary()(context.Background(), r, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_ApplyBotInstallationRemoval_FullMethodName}, func(context.Context, any) (any, error) {
		t.Fatal("ordinary listener admitted global cleanup")
		return nil, nil
	})
	require.Error(t, err)
}
