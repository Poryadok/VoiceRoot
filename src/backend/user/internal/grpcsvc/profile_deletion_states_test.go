package grpcsvc

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
)

func deletionStatesContext(t *testing.T, req *userv1.GetProfileDeletionStatesRequest) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	return principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "messaging", Subject: "service:messaging", Audience: "user", RPC: userv1.UserService_GetProfileDeletionStates_FullMethodName, RequestHash: hash})
}

func TestProfileDeletionStatesRejectsEntireInvalidBatchBeforeStore(t *testing.T) {
	id := uuid.NewString()
	tooMany := make([]string, 501)
	for i := range tooMany {
		tooMany[i] = uuid.NewString()
	}
	for name, ids := range map[string][]string{"empty": nil, "duplicate": {id, id}, "nil uuid": {uuid.Nil.String()}, "malformed": {id, "bad"}, "uppercase": {strings.ToUpper(id)}, "space": {" " + id}, "oversize": tooMany} {
		t.Run(name, func(t *testing.T) {
			req := &userv1.GetProfileDeletionStatesRequest{ProfileIds: ids}
			svc := &MessagingScheduledPresenceGRPC{User: &UserGRPC{}}
			resp, err := svc.GetProfileDeletionStates(deletionStatesContext(t, req), req)
			require.Nil(t, resp)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

func TestProfileDeletionStatesRequiresExactPrincipalAndStore(t *testing.T) {
	req := &userv1.GetProfileDeletionStatesRequest{ProfileIds: []string{uuid.NewString()}}
	svc := &MessagingScheduledPresenceGRPC{User: &UserGRPC{}}
	_, err := svc.GetProfileDeletionStates(context.Background(), req)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	_, err = svc.GetProfileDeletionStates(deletionStatesContext(t, req), req)
	require.Equal(t, codes.Unavailable, status.Code(err))
	changed := &userv1.GetProfileDeletionStatesRequest{ProfileIds: []string{uuid.NewString()}}
	_, err = svc.GetProfileDeletionStates(deletionStatesContext(t, req), changed)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = (&UserGRPC{}).GetProfileDeletionStates(deletionStatesContext(t, req), req)
	require.Equal(t, codes.Unimplemented, status.Code(err), "ordinary User entrypoint must remain absent")
}
