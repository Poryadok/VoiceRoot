package grpcsvc

import (
	"context"
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
		wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(page)
		require.NoError(t, err)
		page.PageSha256 = domainSeparatedSHA(string(page.ProtoReflect().Descriptor().FullName()), wire)
		return page
	}
	cases := map[string]*chatv1.SpacePurgeManifestPage{"empty": valid(nil, 0), "incomplete": valid([]uuid.UUID{a}, 2), "unsorted": valid([]uuid.UUID{b, a}, 2), "tampered-hash": valid([]uuid.UUID{a}, 1)}
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
