package grpcsvc

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/store"
)

func TestLifecycleAdmissionErrorsPreservePublicDenialSemantics(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		code codes.Code
	}{
		{"purged or missing Space", pgx.ErrNoRows, codes.NotFound},
		{"wrapped missing Space", fmt.Errorf("owner lookup: %w", pgx.ErrNoRows), codes.NotFound},
		{"restore after expiry decision", store.ErrLifecycleStateTransition, codes.FailedPrecondition},
		{"restore by a nonowner", store.ErrNotSpaceOwner, codes.PermissionDenied},
		{"frozen mutation", store.ErrLifecycleFrozen, codes.Unavailable},
		{"database failure", fmt.Errorf("private database diagnostic"), codes.Internal},
	} {
		t.Run(test.name, func(t *testing.T) {
			mapped := mapLifecycleAdmissionError(test.err)
			require.Equal(t, test.code, status.Code(mapped))
			require.NotContains(t, status.Convert(mapped).Message(), "private database diagnostic")
		})
	}
}

func TestRestoreSpaceRequiresAuthenticatedAccountProfileAndSession(t *testing.T) {
	service := &SpaceGRPC{Store: &store.SpaceStore{}}
	response, err := service.RestoreSpace(context.Background(), &spacev1.RestoreSpaceRequest{
		SpaceId: uuid.NewString(), OperationId: uuid.NewString(),
	})
	require.Nil(t, response)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestGetSpaceDeletionCoordinatorStatusRequiresAuthenticatedAccountProfileAndSession(t *testing.T) {
	service := &SpaceGRPC{Store: &store.SpaceStore{}}
	response, err := service.GetSpaceDeletionCoordinatorStatus(context.Background(), &spacev1.GetSpaceDeletionCoordinatorStatusRequest{
		ProtocolVersion: 1, SpaceId: uuid.NewString(), DeletionOperationId: uuid.NewString(),
	})
	require.Nil(t, response)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}
