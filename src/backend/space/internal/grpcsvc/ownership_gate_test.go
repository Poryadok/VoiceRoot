package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/store"
)

type ownershipGateLockProbe struct{ calls int }

func (p *ownershipGateLockProbe) Acquire(context.Context, uuid.UUID) (func(), error) {
	p.calls++
	return nil, status.Error(codes.Unavailable, "unexpected dependency access")
}

func TestTransferOwnership_DefaultGateDeniesBeforeDependencies(t *testing.T) {
	for _, configured := range []bool{false, true} {
		name := "without Role runtime"
		if configured {
			name = "with signed Role runtime"
		}
		t.Run(name, func(t *testing.T) {
			lock := &ownershipGateLockProbe{}
			// A nil pool is intentional: any attempt to access persistence must fail.
			svc := &SpaceGRPC{Store: &store.SpaceStore{Pool: nil}, MutationLocker: lock}
			if configured {
				svc.Roles = &ownershipPrincipalRoleClient{}
				svc.OwnershipRoles = &ownershipPrincipalRoleClient{}
				svc.PrincipalIssuer = ownershipTestIssuer(t)
			}
			ctx := directServerContext(withAccountProfileCtx(context.Background(), uuid.New(), uuid.New()))
			response, err := svc.TransferOwnership(ctx, &spacev1.TransferOwnershipRequest{SpaceId: uuid.NewString(), NewOwnerProfileId: uuid.NewString()})
			require.Nil(t, response)
			require.Equal(t, codes.Unavailable, status.Code(err), "production ownership transfer must remain disabled until v2")
			require.Zero(t, lock.calls, "disabled entrypoint must reject before taking a mutation lock or reading the database")
		})
	}
}
