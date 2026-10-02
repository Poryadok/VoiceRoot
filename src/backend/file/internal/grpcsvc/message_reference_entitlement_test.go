package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	filev1 "voice.app/voice/file/v1"
)

type fixedMessageReferenceGuard struct {
	allowed bool
	message uuid.UUID
	profile uuid.UUID
}

func (g *fixedMessageReferenceGuard) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}
func (g *fixedMessageReferenceGuard) ChatE2EState(context.Context, uuid.UUID) (string, bool, error) {
	return "group", false, nil
}
func (g *fixedMessageReferenceGuard) MessageReadEntitledForMessage(_ context.Context, messageID, profileID uuid.UUID) (bool, error) {
	g.message, g.profile = messageID, profileID
	return g.allowed, nil
}

func TestRequireMessageReferenceEntitlementFailsClosedAtFileReadBoundary(t *testing.T) {
	messageID, profileID := uuid.New(), uuid.New()
	guard := &fixedMessageReferenceGuard{}
	err := requireMessageReferenceEntitlement(context.Background(), guard, messageID, profileID)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Equal(t, messageID, guard.message)
	require.Equal(t, profileID, guard.profile)

	guard.allowed = true
	require.NoError(t, requireMessageReferenceEntitlement(context.Background(), guard, messageID, profileID))
	require.Error(t, requireMessageReferenceEntitlement(context.Background(), nil, messageID, profileID))
}

func TestFileAccessSelectorMessageReferenceChecksEntitlementBeforeDatabaseAccess(t *testing.T) {
	fileID, messageID, profileID := uuid.New(), uuid.New(), uuid.New()
	guard := &fixedMessageReferenceGuard{}
	svc := &FileGRPC{chatGuard: guard}
	selector := &filev1.FileAccessSelector{Selector: &filev1.FileAccessSelector_Reference{Reference: &filev1.FileReferenceKey{
		FileId: fileID.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, OwnerId: messageID.String(),
	}}}

	_, err := svc.fileAccessibleBySelectorTx(context.Background(), nil, fileID, profileID, selector, filev1.FileReadSurface_FILE_READ_SURFACE_METADATA)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Equal(t, messageID, guard.message)
	require.Equal(t, profileID, guard.profile)
}
