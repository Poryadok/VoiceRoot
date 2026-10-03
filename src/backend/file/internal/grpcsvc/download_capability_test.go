package grpcsvc

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	filev1 "voice.app/voice/file/v1"
)

func TestRevocableDownloadCapabilityBindsIdentityReferenceVariantAndExpiry(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	key := []byte("0123456789abcdef0123456789abcdef")
	profileID, fileID, messageID, chatID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	access := &filev1.FileAccessSelector{Selector: &filev1.FileAccessSelector_Reference{
		Reference: &filev1.FileReferenceKey{
			FileId: fileID.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE,
			OwnerId: messageID.String(), ScopeSpaceId: stringPtr(chatID.String()),
		},
	}}
	capability := revocableDownloadCapability{
		ProfileID: profileID, FileID: fileID, Variant: filev1.FileURLVariant_FILE_URL_VARIANT_UNSPECIFIED,
		Access: access, ExpiresAt: now.Add(time.Minute),
	}
	token, err := signRevocableDownloadCapability(capability, key)
	require.NoError(t, err)

	decoded, err := verifyRevocableDownloadCapability(token, key, now)
	require.NoError(t, err)
	require.Equal(t, capability.ProfileID, decoded.ProfileID)
	require.Equal(t, capability.FileID, decoded.FileID)
	require.Equal(t, capability.Variant, decoded.Variant)
	require.True(t, proto.Equal(capability.Access, decoded.Access))
	require.True(t, capability.ExpiresAt.Equal(decoded.ExpiresAt))

	_, err = verifyRevocableDownloadCapability(token, []byte("abcdef0123456789abcdef0123456789"), now)
	require.Error(t, err, "a capability is a bearer only under the File signing key")
	_, err = verifyRevocableDownloadCapability(token+"x", key, now)
	require.Error(t, err, "changing any selector byte invalidates the capability")
	_, err = verifyRevocableDownloadCapability(token, key, capability.ExpiresAt)
	require.Error(t, err, "expiry is exclusive")
}

func stringPtr(value string) *string { return &value }
