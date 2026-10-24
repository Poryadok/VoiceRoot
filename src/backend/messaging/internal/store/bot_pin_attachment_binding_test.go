package store

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	messagingv1 "voice.app/voice/messaging/v1"
)

func TestBotPinAttachmentProjectionGoldenAndRootIndependence(t *testing.T) {
	// Independently encoded from protobuf tags and the c50 domain prefix.
	// The pinned deterministic Go codec emits ordinary fields 1,6,7,8,13,14
	// before the evidence oneof field10; protobuf wire order is not globally
	// canonical across languages. No generated marshal calculated this vector.
	request := &messagingv1.ApplyBotChatPinRemovalRequest{ProtocolVersion: 1, BotId: "00000000-0000-0000-0000-000000000101", ActorProfileId: "00000000-0000-0000-0000-000000000102", SpaceId: "00000000-0000-0000-0000-000000000103", ChatId: "00000000-0000-0000-0000-000000000104", Evidence: &messagingv1.ApplyBotChatPinRemovalRequest_Native{Native: &messagingv1.BotRemovalNativeEvidence{InstallationId: "00000000-0000-0000-0000-000000000105", InstallOperationId: "00000000-0000-0000-0000-000000000106", Generation: 1}}, Attachments: []*messagingv1.BotChatPinRemovalAttachment{{AttachmentId: "00000000-0000-0000-0000-000000000107", OriginalScopeEvidence: &messagingv1.BotChatPinRemovalAttachment_OriginalScopeUnqualified{OriginalScopeUnqualified: true}}}}
	first, err := bindBotPinAttachments(request)
	require.NoError(t, err)
	require.Equal(t, "bba7dd0e659ddbe78b0aa227759066e78371d63ccf3ec9a58812d2f9cd99813f", hex.EncodeToString(first[0].digest[:]))
	changed := nextBotPinDecision(request, "global_delete")
	changed.Attachments[0].CoveringOperationId = "00000000-0000-0000-0000-000000000108"
	second, err := bindBotPinAttachments(changed)
	require.NoError(t, err)
	require.Equal(t, first[0].projection, second[0].projection)
	changed.GetNative().Generation = 2
	third, err := bindBotPinAttachments(changed)
	require.NoError(t, err)
	require.NotEqual(t, first[0].digest, third[0].digest, "a new native installation generation cannot borrow coverage")
	for _, mode := range []string{"duplicate", "unsorted", "false origin", "unknown nested", "missing origin"} {
		t.Run(mode, func(t *testing.T) {
			bad := proto.Clone(request).(*messagingv1.ApplyBotChatPinRemovalRequest)
			switch mode {
			case "duplicate":
				bad.Attachments = append(bad.Attachments, proto.Clone(bad.Attachments[0]).(*messagingv1.BotChatPinRemovalAttachment))
			case "unsorted":
				other := proto.Clone(bad.Attachments[0]).(*messagingv1.BotChatPinRemovalAttachment)
				other.AttachmentId = "00000000-0000-0000-0000-000000000100"
				bad.Attachments = append(bad.Attachments, other)
			case "false origin":
				bad.Attachments[0].OriginalScopeEvidence = &messagingv1.BotChatPinRemovalAttachment_OriginalScopeUnqualified{OriginalScopeUnqualified: false}
			case "unknown nested":
				bad.Attachments[0].ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
			case "missing origin":
				bad.Attachments[0].OriginalScopeEvidence = nil
			}
			_, err := bindBotPinAttachments(bad)
			require.ErrorIs(t, err, ErrBotPinRemovalBinding)
		})
	}
}
