package grpcsvc

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
)

// TestR23ChatPurgeDependencyProofContractGap keeps the accepted Chat purge
// dependency barrier visible without inventing a wire contract. The current
// request has only the common purge envelope, while docs require evidence that
// Messaging completed and File accepted Chat-owned reference releases before
// Chat may delete authoritative rows.
func TestR23ChatPurgeDependencyProofContractGap(t *testing.T) {
	descriptor := (&chatv1.PurgeSpaceRequest{}).ProtoReflect().Descriptor()
	require.Equal(t, "voice.chat.v1.PurgeSpaceRequest", string(descriptor.FullName()))
	require.Equal(t, 1, descriptor.Fields().Len(),
		"accepted Messaging/File dependency-proof fields or RPC are still missing")
	require.Equal(t, "purge", string(descriptor.Fields().Get(0).Name()))

	request := &chatv1.PurgeSpaceRequest{Purge: &commonv1.SpacePurgeRequest{
		ProtocolVersion:     1,
		SpaceId:             "20000000-0000-4000-8000-000000000001",
		DeletionOperationId: "20000000-0000-4000-8000-000000000002",
		Generation:          2,
		PurgeDecidedAt:      timestamppb.New(time.Unix(1_800_000_000, 0).UTC()),
		ParticipantId:       commonv1.ParticipantId_PARTICIPANT_ID_CHAT,
		Manifest: &commonv1.ManifestBinding{
			ManifestId:     "20000000-0000-4000-8000-000000000003",
			ManifestSha256: bytes.Repeat([]byte{0x23}, 32),
			ItemCount:      3,
		},
	}}
	response, err := (&ChatGRPC{}).PurgeSpace(context.Background(), request)
	require.Nil(t, response, "an unimplemented purge must never report success")
	require.Error(t, err)
	require.Equal(t, codes.Unimplemented, status.Code(err),
		"replace this contract-gap assertion with fail-closed dependency validation when the accepted wire lands")
}

// TestR23ChatDeferredREDManifest is intentionally non-executable product work.
// It preserves the reviewed RED scope until the dependency-proof descriptor is
// accepted; converting these entries to behavior tests before then would either
// invent wire or make main CI deliberately fail on generated Unimplemented RPCs.
func TestR23ChatDeferredREDManifest(t *testing.T) {
	deferred := []string{
		"guarded DOWN independently rejects every evidence, manifest, page, item, purge-receipt and permanent-fence root",
		"empty Space manifest returns immutable zero-count receipt and empty pages",
		"manifest pages sort raw UUID bytes, cap at 1000 items and replay exact stored bytes",
		"unknown fields, wrong kind, invalid root hash/count and changed Prepare bytes fail before mutation",
		"freeze linearizes with concurrent create and update without a TOCTOU escape",
		"ListChats, delete, member, mute, quick-access and folder paths fail closed under a Space fence",
		"higher LIVE after PURGE_DECIDED and permanent PURGED identity reuse are rejected",
		"full evidence expires on PostgreSQL time exactly 30 days after completion while permanent fences survive compaction",
		"successful purge requires accepted Messaging completion and File release evidence, then replays exact bytes and rejects changed proof",
	}
	require.Len(t, deferred, 9)
}
