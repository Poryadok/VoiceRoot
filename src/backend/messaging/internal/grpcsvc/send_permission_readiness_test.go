package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Without authoritative Chat scope, the service cannot prove that Space Role
// checks are unnecessary. Missing wiring must not silently authorize a send.
func TestSpaceSendPermissionMissingScopeFailsClosed(t *testing.T) {
	for _, configuredRole := range []bool{false, true} {
		service := &MessagingGRPC{}
		if configuredRole {
			service.RolePermissions = testRolePerms{allowed: true}
		}
		err := service.checkSpaceSendPermission(context.Background(), uuid.New(), uuid.New())
		require.Equal(t, codes.Unavailable, status.Code(err))
	}
}
