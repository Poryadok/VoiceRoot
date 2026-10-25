package principalruntime

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

// Exercise the actual mTLS listener chain: strict creation admission must not be
// followed by the separate nickname-only permission predicate.
func TestRuntimeListenerCreationPermissionExactSignedBranch(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	address, recorder, roots := startRuntimeListener(t, f)
	client := runtimeListenerClient(t, address, runtimeTLSCredentials(f, roots))
	method := rolev1.RoleService_CheckPermission_FullMethodName
	base := &rolev1.CheckPermissionRequest{SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), PermissionName: "TEXT_CHAT_CREATE_IN_SPACE"}
	for _, name := range []string{"valid", "other_permission", "chat_empty", "voice_empty", "wrong_issuer", "wrong_hash", "raw_actor", "duplicate_credential", "space_noncanonical", "profile_zero"} {
		t.Run(name, func(t *testing.T) {
			req := proto.Clone(base).(*rolev1.CheckPermissionRequest)
			issuer := "space"
			switch name {
			case "other_permission":
				req.PermissionName = "SPACE_MANAGE_ROLES"
			case "chat_empty":
				req.Chat = &chatv1.ChatRef{}
			case "voice_empty":
				v := ""
				req.VoiceRoomId = &v
			case "wrong_issuer":
				issuer = "voice"
			case "space_noncanonical":
				req.SpaceId = " " + req.SpaceId
			case "profile_zero":
				req.ProfileId = uuid.Nil.String()
			}
			hash, err := principal.RequestHash(req)
			require.NoError(t, err)
			if name == "wrong_hash" {
				hash = runtimeHash
			}
			id := uuid.NewString()
			token := issueRuntimeToken(t, f.key, issuer, "current", "role", method, id, hash)
			md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)
			if name == "raw_actor" {
				md.Set("x-voice-profile-id", req.ProfileId)
			}
			if name == "duplicate_credential" {
				md.Append("authorization", "Bearer "+token)
			}
			ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), md), 2*time.Second)
			defer cancel()
			_, err = client.CheckPermission(ctx, req)
			switch name {
			case "valid":
				require.NoError(t, err)
				p := <-recorder.principals
				require.Equal(t, "service:space", p.Subject)
				require.Equal(t, hash, p.RequestHash)
				_, err = client.CheckPermission(ctx, req)
				require.Equal(t, codes.Unauthenticated, status.Code(err), "replay")
			case "other_permission", "chat_empty", "voice_empty", "space_noncanonical", "profile_zero":
				require.Equal(t, codes.PermissionDenied, status.Code(err))
			default:
				require.Equal(t, codes.Unauthenticated, status.Code(err))
			}
		})
	}
	require.Equal(t, int64(1), recorder.calls.Load(), "invalid branch must not reach handler")
}
