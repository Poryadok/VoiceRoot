package grpcsvc

import (
	"encoding/hex"
	"github.com/stretchr/testify/require"
	"testing"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

func TestManifestImportAcceptsCanonicalChatOwnerPageGolden(t *testing.T) {
	root, err := hex.DecodeString("09e184464f01e9fd66422cd60767fd8751f7420168101052fa1a3a23b37bc6d1")
	require.NoError(t, err)
	pageHash, err := hex.DecodeString("059cef4e2162c562f656af4c60c4d555aed9c13e8e5a211c7cd88215e8a8a750")
	require.NoError(t, err)
	request := &messagingv1.ImportSpacePurgeManifestPageRequest{ProtocolVersion: 1, SpaceId: "20000000-0000-4000-8000-000000000301", DeletionOperationId: "20000000-0000-4000-8000-000000000302", ScheduleGeneration: 7, SealsManifest: true, Page: &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: &commonv1.ManifestBinding{ManifestId: "20000000-0000-4000-8000-000000000304", ItemCount: 1, ManifestSha256: root}, ItemIds: []string{"20000000-0000-4000-8000-000000000303"}, PageSha256: pageHash}}
	require.NoError(t, validateSpacePurgeManifestPageRequest(request), "Chat's published page algorithm is the wire contract")
	request.Page.PageSha256[0] ^= 1
	require.Error(t, validateSpacePurgeManifestPageRequest(request))
}
