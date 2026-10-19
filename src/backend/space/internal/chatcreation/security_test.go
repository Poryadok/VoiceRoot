package chatcreation

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func TestServiceVerifierBindsActualCredentialAndRejectsAlternateIdentity(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Second)
	clock := func() time.Time { return now }
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "chat", KeyID: "current", PrivateKey: key, Clock: clock})
	require.NoError(t, err)
	req := &spacev1.CommitSpaceChatCreationRequest{Binding: bindingFixture()}
	method := spacev1.SpaceService_CommitSpaceChatCreation_FullMethodName
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: method, RequestID: req.Binding.OperationId, RequestHash: hash})
	require.NoError(t, err)
	incoming := func(extra ...string) context.Context {
		md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", req.Binding.OperationId)
		for i := 0; i < len(extra); i += 2 {
			md.Append(extra[i], extra[i+1])
		}
		return metadata.NewIncomingContext(context.Background(), md)
	}
	calls := 0
	verifier := ServiceVerifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { calls++; return nil }, Clock: clock}
	p, err := verifier.Verify(incoming(), method, req)
	require.NoError(t, err)
	require.Equal(t, "service:chat", p.Subject)
	require.Equal(t, 1, calls)
	for _, name := range []string{"x-voice-profile-id", "x-voice-user-id", "x-voice-account-id", "x-voice-account-type", "x-voice-internal-caller"} {
		_, err = verifier.Verify(incoming(name, "forged"), method, req)
		require.Error(t, err, name)
	}
	_, err = verifier.Verify(incoming("authorization", "Bearer "+token), method, req)
	require.Error(t, err)
	changed := proto.Clone(req).(*spacev1.CommitSpaceChatCreationRequest)
	changed.Binding.AuditEventId = changed.Binding.NodeId
	_, err = verifier.Verify(incoming(), method, changed)
	require.Error(t, err)
	_, err = verifier.Verify(incoming(), spacev1.SpaceService_UpsertTreeNode_FullMethodName, req)
	require.Error(t, err)
	unknown := proto.Clone(req).(*spacev1.CommitSpaceChatCreationRequest)
	unknown.Binding.GetUser().ProtoReflect().SetUnknown([]byte{0x78, 1})
	_, err = verifier.Verify(incoming(), method, unknown)
	require.Error(t, err)
	unavailable := verifier
	unavailable.Replay = nil
	_, err = unavailable.Verify(incoming(), method, req)
	require.Error(t, err)
	unavailable = verifier
	unavailable.Replay = func(context.Context, string, string, time.Time) error {
		return errors.New("replay backend unavailable")
	}
	_, err = unavailable.Verify(incoming(), method, req)
	require.Error(t, err)
}
