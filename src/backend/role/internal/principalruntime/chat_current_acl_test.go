package principalruntime

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func TestChatCurrentACLActualSignedBoundary(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	f.config.JWKSURLs["chat"] = f.config.JWKSURLs["space"]
	address, recorder, roots := startRuntimeListener(t, f)
	client := runtimeListenerClient(t, address, runtimeTLSCredentials(f, roots))
	req := &rolev1.CheckPermissionRequest{SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), PermissionName: "TEXT_CHAT_VIEW", Chat: &chatv1.ChatRef{Id: uuid.NewString(), Type: chatv1.ChatType_CHAT_TYPE_GROUP}}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	id := uuid.NewString()
	token := issueRuntimeToken(t, f.key, "chat", "current", "role", rolev1.RoleService_CheckPermission_FullMethodName, id, hash)
	ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), 2*time.Second)
	defer cancel()
	_, err = client.CheckPermission(ctx, req)
	require.NoError(t, err, "agreed exact service:chat CURRENTACL must reach the protected handler")
	p := <-recorder.principals
	require.Equal(t, "service:chat", p.Subject)
	require.EqualValues(t, 1, recorder.calls.Load())
}
