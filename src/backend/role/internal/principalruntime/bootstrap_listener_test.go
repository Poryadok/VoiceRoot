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
 rolev1 "voice.app/voice/role/v1"
 "voice/backend/pkg/principal"
)

func (s *recordingRoleServer) BootstrapSpaceRoles(ctx context.Context, _ *rolev1.BootstrapSpaceRolesRequest) (*rolev1.BootstrapSpaceRolesResponse, error) {
 s.calls.Add(1)
 p, _ := principal.FromContext(ctx)
 s.principals <- p
 return &rolev1.BootstrapSpaceRolesResponse{}, nil
}

// This proves only transport admission to a recorder, never Owner qualification.
func TestRuntimeBootstrapExactSpaceTransport(t *testing.T) {
 f := newRuntimeFixture(t, 2)
 address, recorder, roots := startRuntimeListener(t, f)
 client := runtimeListenerClient(t, address, runtimeTLSCredentials(f, roots))
 req := &rolev1.BootstrapSpaceRolesRequest{SpaceId: uuid.NewString(), OwnerProfileId: uuid.NewString()}
 hash, err := principal.RequestHash(req)
 require.NoError(t, err)
 method := rolev1.RoleService_BootstrapSpaceRoles_FullMethodName
 id := uuid.NewString()
 token := issueRuntimeToken(t, f.key, "space", "current", "role", method, id, hash)
 md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)
 call := func() error {
  ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), md.Copy()), 2*time.Second)
  defer cancel()
  _, err := client.BootstrapSpaceRoles(ctx, req)
  require.NoError(t, ctx.Err(), "auth status must not be a dead RPC context")
  return err
 }
 require.NoError(t, call())
 p := <-recorder.principals
 require.Equal(t, "service:space", p.Subject)
 require.Equal(t, method, p.RPC)
 require.Equal(t, id, p.RequestID)
 require.Equal(t, hash, p.RequestHash)
 require.Equal(t, codes.Unauthenticated, status.Code(call()), "spent JTI must not reach handler")
 require.Equal(t, int64(1), recorder.calls.Load())
}
