package sessionowners

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/gameintegration/internal/registry"
	"voice/backend/pkg/gisowner"
	"voice/backend/pkg/principal"
)

type rejectingVoiceRoster struct {
	callsv1.GameSessionProvisioningServiceClient
	err   error
	calls int
}

func (v *rejectingVoiceRoster) ApplyGameSessionRoster(context.Context, *callsv1.ApplyGameSessionRosterRequest, ...grpc.CallOption) (*callsv1.ApplyGameSessionRosterResponse, error) {
	v.calls++
	return nil, v.err
}

func TestVoiceRosterAdapterClassifiesOnlyExactBoundRejection(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gameintegration", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	operationID := uuid.New()
	request := &callsv1.ApplyGameSessionRosterRequest{OperationId: operationID.String()}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	owner := registry.SessionOwnerRequest{OperationID: operationID, RequestHash: hash, Proto: request}
	for _, tc := range []struct {
		name, domain, rpc, operation, hash string
		permanent                          bool
	}{
		{"exact", gisowner.VoiceDomain, gisowner.VoiceRosterRPC, operationID.String(), hash, true},
		{"wrong owner", gisowner.ChatDomain, gisowner.ChatRosterRPC, operationID.String(), hash, false},
		{"wrong method", gisowner.VoiceDomain, gisowner.VoiceProvisionRPC, operationID.String(), hash, false},
		{"wrong operation", gisowner.VoiceDomain, gisowner.VoiceRosterRPC, uuid.NewString(), hash, false},
		{"wrong hash", gisowner.VoiceDomain, gisowner.VoiceRosterRPC, operationID.String(), "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", false},
		{"untyped", "", "", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := status.Error(codes.AlreadyExists, "conflict")
			if tc.name != "untyped" {
				raw = gisowner.Annotate(raw, tc.domain, tc.rpc, gisowner.CategoryOperationConflict, tc.operation, tc.hash)
			}
			voice := &rejectingVoiceRoster{err: raw}
			client := &Clients{voice: voice, issuer: issuer}
			_, got := client.applyVoiceRoster(context.Background(), owner)
			require.Error(t, got)
			var rejection *registry.PermanentOwnerRejection
			require.Equal(t, tc.permanent, errors.As(got, &rejection))
			require.Equal(t, 1, voice.calls)
		})
	}
}
