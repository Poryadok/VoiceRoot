package grpcsvc

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	searchv1 "voice.app/voice/search/v1"
)

func TestImportChatManifestAcceptsCanonicalSourceUnderDistinctSpaceRoot(t *testing.T) {
	spaceID := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	opID := uuid.MustParse("00000000-0000-4000-8000-000000000002")
	root := &commonv1.ManifestBinding{ManifestId: "00000000-0000-4000-8000-000000000003", ManifestSha256: make([]byte, 32), ItemCount: 9}
	for _, fixture := range []struct {
		name, manifestHash, pageHash string
		ids                          []string
	}{
		{"empty", "2ba65445d67a8aeef96f4ee918e2b46c38bc1928a7d96953f0d706c6a67fcb0f", "83f1e2c6a7dfa6a7bd651f56fc26cd50942e9b7a3baaafd1c9f64544e3695654", nil},
		{"one chat", "619f3fb355b406e95d67070613667a54e1dae8df95e32bbb347d6389e51db400", "6b11f19bf96c185b2b0960c4ff98b1e0ce69c6aaaa385026c4771ca49df96d98", []string{"00000000-0000-4000-8000-000000000005"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			manifestHash, err := hex.DecodeString(fixture.manifestHash)
			require.NoError(t, err)
			pageHash, err := hex.DecodeString(fixture.pageHash)
			require.NoError(t, err)
			page := &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: &commonv1.ManifestBinding{ManifestId: "00000000-0000-4000-8000-000000000004", ManifestSha256: manifestHash, ItemCount: uint64(len(fixture.ids))}, ItemIds: fixture.ids, PageSha256: pageHash}
			svc := &SearchGRPC{ChatManifest: &r23ManifestClient{page: page}}
			imported, err := svc.importChatManifest(context.Background(), spaceID, opID, 1, root)
			require.NoError(t, err, "authenticated Chat source is independent of the aggregate Space root")
			require.True(t, proto.Equal(page.Manifest, imported.Binding))
			require.Len(t, imported.ChatIDs, len(fixture.ids))
		})
	}
}

func TestImportChatManifestRejectsEmptyIncompleteTamperedAndUnsortedPages(t *testing.T) {
	spaceID, opID := uuid.New(), uuid.New()
	root := &commonv1.ManifestBinding{ManifestId: "root", ManifestSha256: make([]byte, 32), ItemCount: 1}
	a, b := uuid.New(), uuid.New()
	if string(a[:]) > string(b[:]) {
		a, b = b, a
	}
	valid := func(ids []uuid.UUID, itemCount uint64) *chatv1.SpacePurgeManifestPage {
		raw := make([]string, len(ids))
		for i, id := range ids {
			raw[i] = id.String()
		}
		binding := &commonv1.ManifestBinding{ManifestId: "root", ItemCount: itemCount, ManifestSha256: chatManifestSHA(spaceID, opID, 1, ids)}
		page := &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: binding, ItemIds: raw}
		page.PageSha256 = chatManifestPageSHA(binding.ManifestSha256, 0, ids)
		return page
	}
	cases := map[string]*chatv1.SpacePurgeManifestPage{"empty": valid(nil, 0), "incomplete": valid([]uuid.UUID{a}, 2), "unsorted": valid([]uuid.UUID{b, a}, 2), "tampered-hash": valid([]uuid.UUID{a}, 1)}
	cases["empty"].NextPageToken = "unexpected-next-page"
	cases["tampered-hash"].PageSha256[0] ^= 1
	for name, page := range cases {
		t.Run(name, func(t *testing.T) {
			svc := &SearchGRPC{ChatManifest: &r23ManifestClient{page: page}}
			_, err := svc.importChatManifest(context.Background(), spaceID, opID, 1, root)
			require.Error(t, err)
		})
	}
}

func TestLifecycleValidationRejectsNestedUnknownFieldsAndNonCanonicalUUID(t *testing.T) {
	req := &searchv1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: uuid.NewString(), DeletionOperationId: uuid.NewString(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: &commonv1.ManifestBinding{ManifestId: "root", ManifestSha256: make([]byte, 32), ItemCount: 1}}}
	req.Fence.Manifest.ProtoReflect().SetUnknown(protowire.AppendTag(nil, 99, protowire.VarintType))
	_, _, err := validateFence(req)
	require.Error(t, err)
	req.Fence.Manifest.ProtoReflect().SetUnknown(nil)
	req.Fence.SpaceId = "{00000000-0000-0000-0000-000000000001}"
	_, _, err = validateFence(req)
	require.Error(t, err)
	_, _, err = validatePurge(&searchv1.PurgeSpaceRequest{})
	require.Error(t, err)
	require.NotEqual(t, codes.OK, status.Code(err))
}
