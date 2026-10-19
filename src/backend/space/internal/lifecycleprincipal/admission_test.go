package lifecycleprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func TestAdmissionCallExactActorMethodBudgetAndNoEligibilityForgery(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	for _, name := range []string{"regular", "guest", "missing type", "sdk", "wrong rpc", "wrong audience", "changed body", "raw actor", "unknown fields", "owner unavailable", "owner deadline", "budget before", "budget equal", "expired"} {
		t.Run(name, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Second)
			clock := func() time.Time { return now }
			issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: clock})
			require.NoError(t, err)
			request := &spacev1.JoinSpaceRequest{SpaceId: uuid.NewString()}
			if name == "unknown fields" {
				request.ProtoReflect().SetUnknown([]byte{0x78, 1})
			}
			hash, err := principal.RequestHash(request)
			require.NoError(t, err)
			input := principal.DelegatedUserInput{Audience: "space", RPC: admissionRPCPrefix + "JoinSpace", RequestID: uuid.NewString(), RequestHash: hash, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: now.Add(time.Minute)}
			switch name {
			case "guest":
				input.AccountType = "guest"
			case "missing type":
				input.AccountType = ""
			case "sdk":
				input.AccountType = "sdk-account"
			case "wrong rpc":
				input.RPC = admissionRPCPrefix + "SubmitJoinAttempt"
			case "wrong audience":
				input.Audience = "role"
			}
			token, err := issuer.IssueDelegatedUser(input)
			require.NoError(t, err)
			if name == "changed body" {
				request.SpaceId = uuid.NewString()
			}
			md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", input.RequestID)
			if name == "raw actor" {
				md.Set("x-voice-profile-id", input.ProfileID)
			}
			ctx := metadata.NewIncomingContext(context.Background(), md)
			seen := map[string]bool{}
			v := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Clock: clock,
				AdmissionReplay: func(_ context.Context, _ string, jti string, _ time.Time) error {
					if seen[jti] {
						return errors.New("replay")
					}
					seen[jti] = true
					return nil
				},
				AdmissionEpoch: func(ctx context.Context, account string, epoch int64) error {
					require.Equal(t, input.AccountID, account)
					require.EqualValues(t, 7, epoch)
					deadline, ok := ctx.Deadline()
					require.True(t, ok)
					require.LessOrEqual(t, time.Until(deadline), 2*time.Second)
					if name == "owner unavailable" {
						return ErrDependencyUnavailable
					}
					if name == "owner deadline" {
						now = now.Add(2 * time.Second)
					}
					return nil
				},
				CreatorEpoch: func(context.Context, string, int64) error {
					t.Fatal("admission cannot use Creator's five-second checker")
					return nil
				},
			}
			handled := false
			_, err = v.Unary()(ctx, request, &grpc.UnaryServerInfo{FullMethod: admissionRPCPrefix + "JoinSpace"}, func(ctx context.Context, _ any) (any, error) {
				handled = true
				switch name {
				case "budget before":
					now = now.Add(5*time.Second - time.Nanosecond)
				case "budget equal":
					now = now.Add(5 * time.Second)
				case "expired":
					now = now.Add(40 * time.Second)
				}
				call, err := RequireAdmissionJoinSpace(ctx, request)
				if err != nil {
					return nil, err
				}
				actor, err := call.ActorFor(request)
				require.NoError(t, err)
				require.Equal(t, input.AccountType, actor.AccountType)
				_, err = RequireAdmissionApproval(ctx, request)
				require.Equal(t, codes.Unauthenticated, status.Code(err), "one method's capability cannot authorize approval")
				_, err = RequireAdmissionJoinSpace(principal.WithVerified(context.Background(), actor), request)
				require.Equal(t, codes.Unauthenticated, status.Code(err), "a generic verified principal cannot construct the private capability")
				changed := proto.Clone(request).(*spacev1.JoinSpaceRequest)
				changed.SpaceId = uuid.NewString()
				_, err = call.ActorFor(changed)
				require.Equal(t, codes.Unauthenticated, status.Code(err))
				return nil, nil
			})
			want := codes.Unauthenticated
			switch name {
			case "regular", "guest", "budget before":
				want = codes.OK
			case "unknown fields":
				want = codes.InvalidArgument
			case "owner unavailable", "owner deadline", "budget equal":
				want = codes.Unavailable
			}
			require.Equal(t, want, status.Code(err))
			if want == codes.OK {
				require.True(t, handled)
				_, err = v.Unary()(ctx, request, &grpc.UnaryServerInfo{FullMethod: admissionRPCPrefix + "JoinSpace"}, func(context.Context, any) (any, error) { t.Fatal("credential replay reached handler"); return nil, nil })
				require.Equal(t, codes.Unauthenticated, status.Code(err))
			}
		})
	}
	for _, method := range []string{"StartJoinAttempt", "GetJoinAttempt", "SubmitJoinAttempt", "ListJoinRequests", "GetJoinRequest", "ApproveJoinRequest", "JoinSpace", "JoinByInvite", "SetEntryPolicy"} {
		_, err := OrdinaryUnary()(context.Background(), &spacev1.JoinSpaceRequest{}, &grpc.UnaryServerInfo{FullMethod: admissionRPCPrefix + method}, func(context.Context, any) (any, error) {
			t.Fatal("ordinary listener reached protected admission handler")
			return nil, nil
		})
		require.Equal(t, codes.PermissionDenied, status.Code(err))
		_, err = (Verifier{}).Unary()(context.Background(), &spacev1.JoinSpaceRequest{}, &grpc.UnaryServerInfo{FullMethod: admissionRPCPrefix + method}, func(context.Context, any) (any, error) {
			t.Fatal("missing owner/replay configuration reached handler")
			return nil, nil
		})
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	}
	_, err = (&VerifiedAdmissionCall{}).ActorFor(&spacev1.JoinSpaceRequest{})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	nested, err := structpb.NewStruct(map[string]any{"answer": map[string]any{"value": "safe"}})
	require.NoError(t, err)
	nested.Fields["answer"].GetStructValue().ProtoReflect().SetUnknown([]byte{0x78, 1})
	require.False(t, validAdmissionMessage(nested, 0), "unknown nested input is not a signed policy bypass")
}
