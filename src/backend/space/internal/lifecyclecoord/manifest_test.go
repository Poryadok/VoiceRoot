package lifecyclecoord

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	notificationv1 "voice.app/voice/notification/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/spacecore"
)

type manifestMemoryStore struct {
	memoryLifecycleStore
	proof   bool
	freezes int
}

func (s *manifestMemoryStore) LifecycleDeletionProofRecorded(context.Context, uuid.UUID) (bool, error) {
	return s.proof, nil
}
func (s *manifestMemoryStore) BeginLifecycleFreeze(_ context.Context, _ uuid.UUID, m *commonv1.ManifestBinding) (*spacecore.LifecycleAggregate, error) {
	s.freezes++
	if err := s.aggregate.BeginFreeze(m); err != nil {
		return nil, err
	}
	return s.aggregate, nil
}

type manifestOwnerFixture struct {
	manifest            *commonv1.ManifestBinding
	page                *chatv1.SpacePurgeManifestPage
	calls               []string
	corrupt             string
	notificationFailure bool
}

func (o *manifestOwnerFixture) SealSpaceFileProducer(context.Context, spacecore.LifecycleSnapshot) error {
	return nil
}

func manifestTestDigest(request proto.Message) []byte {
	wire, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		panic(err)
	}
	digest := sha256.Sum256(append(append([]byte(request.ProtoReflect().Descriptor().FullName()), 0), wire...))
	return digest[:]
}
func (o *manifestOwnerFixture) hash(owner string, request proto.Message) []byte {
	hash := manifestTestDigest(request)
	if o.corrupt == owner {
		hash[0] ^= 1
	}
	return hash
}
func (o *manifestOwnerFixture) PrepareChatManifest(_ context.Context, request *chatv1.PrepareSpaceDeletionManifestRequest) (*chatv1.PrepareSpaceDeletionManifestResponse, error) {
	o.calls = append(o.calls, "chat")
	return &chatv1.PrepareSpaceDeletionManifestResponse{Receipt: &chatv1.PrepareSpaceDeletionManifestReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: request.SpaceId, DeletionOperationId: request.DeletionOperationId, ScheduleGeneration: request.ScheduleGeneration, AppliedState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, ChatManifest: o.manifest, RequestSha256: o.hash("chat", request), AppliedAt: timestamppb.Now()}}, nil
}
func (o *manifestOwnerFixture) GetChatManifestPage(context.Context, *chatv1.GetSpacePurgeManifestPageRequest) (*chatv1.GetSpacePurgeManifestPageResponse, error) {
	o.calls = append(o.calls, "page")
	return &chatv1.GetSpacePurgeManifestPageResponse{Page: o.page}, nil
}
func (o *manifestOwnerFixture) ImportMessagingManifestPage(_ context.Context, request *messagingv1.ImportSpacePurgeManifestPageRequest) (*messagingv1.ImportSpacePurgeManifestPageResponse, error) {
	o.calls = append(o.calls, "messaging")
	return &messagingv1.ImportSpacePurgeManifestPageResponse{Receipt: &messagingv1.ImportSpacePurgeManifestPageReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: request.SpaceId, DeletionOperationId: request.DeletionOperationId, Generation: request.ScheduleGeneration, Manifest: request.Page.Manifest, PageIndex: request.Page.PageIndex, AcceptedCount: uint64(len(request.Page.ItemIds)), PageSha256: request.Page.PageSha256, ManifestSealed: request.SealsManifest, RequestSha256: o.hash("messaging", request), CompletedAt: timestamppb.Now()}}, nil
}
func (o *manifestOwnerFixture) ImportNotificationManifestPage(_ context.Context, request *notificationv1.ImportSpacePurgeManifestPageRequest) (*notificationv1.ImportSpacePurgeManifestPageResponse, error) {
	o.calls = append(o.calls, "notification")
	if o.notificationFailure {
		return nil, errors.New("Notification unavailable")
	}
	return &notificationv1.ImportSpacePurgeManifestPageResponse{Receipt: &notificationv1.ImportSpacePurgeManifestPageReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: request.SpaceId, DeletionOperationId: request.DeletionOperationId, Generation: request.ScheduleGeneration, Manifest: request.Page.Manifest, PageIndex: request.Page.PageIndex, AcceptedCount: uint64(len(request.Page.ItemIds)), PageSha256: request.Page.PageSha256, ManifestSealed: request.SealsManifest, RequestSha256: o.hash("notification", request), CompletedAt: timestamppb.Now()}}, nil
}
func (o *manifestOwnerFixture) PrepareFileManifest(_ context.Context, request *filev1.PrepareSpaceDeletionReferenceManifestRequest) (*filev1.PrepareSpaceDeletionReferenceManifestResponse, error) {
	o.calls = append(o.calls, "file")
	return &filev1.PrepareSpaceDeletionReferenceManifestResponse{Receipt: &filev1.PrepareSpaceDeletionReferenceManifestReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: request.SpaceId, DeletionOperationId: request.DeletionOperationId, ScheduleGeneration: request.ScheduleGeneration, AppliedState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, RequestSha256: o.hash("file", request), AppliedAt: timestamppb.Now()}}, nil
}
func newManifestFixture(t *testing.T) (uuid.UUID, *manifestMemoryStore, *manifestOwnerFixture) {
	t.Helper()
	spaceID, operationID := uuid.New(), uuid.New()
	aggregate, err := spacecore.NewLifecycleAggregate(spaceID.String(), operationID.String())
	require.NoError(t, err)
	require.NoError(t, aggregate.BeginSchedule(7))
	h := sha256.New()
	h.Write([]byte("voice.chat.v1.SpaceDeletionManifest\x00"))
	h.Write(spaceID[:])
	h.Write(operationID[:])
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], 7)
	h.Write(n[:])
	h.Write(make([]byte, 8))
	root := h.Sum(nil)
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: root}
	h = sha256.New()
	h.Write([]byte("voice.chat.v1.SpaceDeletionManifestPage\x00"))
	h.Write(root)
	h.Write(make([]byte, 16))
	return spaceID, &manifestMemoryStore{memoryLifecycleStore: memoryLifecycleStore{aggregate: aggregate}, proof: true}, &manifestOwnerFixture{manifest: manifest, page: &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: manifest, PageSha256: h.Sum(nil)}}
}

func TestPrepareFreezeRequiresEveryExactOwnerReceiptBeforePersistence(t *testing.T) {
	for _, owner := range []string{"chat", "messaging", "notification", "file"} {
		t.Run(owner, func(t *testing.T) {
			id, store, owners := newManifestFixture(t)
			owners.corrupt = owner
			_, err := New(Dependencies{Store: store, Manifests: owners}).PrepareFreeze(context.Background(), id)
			require.Error(t, err, "wrong digest cannot advance the common freeze")
			require.Zero(t, store.freezes)
			require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULE_PENDING, store.aggregate.Phase())
		})
	}
}
func TestPrepareFreezeImportsNotificationBeforeFileAndKeepsScheduleGeneration(t *testing.T) {
	id, store, owners := newManifestFixture(t)
	owners.notificationFailure = true
	coordinator := New(Dependencies{Store: store, Manifests: owners})
	_, err := coordinator.PrepareFreeze(context.Background(), id)
	require.Error(t, err)
	require.Zero(t, store.freezes)
	require.Equal(t, []string{"chat", "page", "messaging", "notification"}, owners.calls)
	owners.notificationFailure = false
	owners.calls = nil
	result, err := coordinator.PrepareFreeze(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, []string{"chat", "page", "messaging", "notification", "file"}, owners.calls)
	require.Equal(t, uint64(7), result.Generation())
	require.True(t, proto.Equal(owners.manifest, result.Snapshot().Manifest))
}
func TestPrepareFreezeWithoutDurableAuthProofNeverCallsOwners(t *testing.T) {
	id, store, owners := newManifestFixture(t)
	store.proof = false
	_, err := New(Dependencies{Store: store, Manifests: owners}).PrepareFreeze(context.Background(), id)
	require.Error(t, err)
	require.Empty(t, owners.calls)
	require.Zero(t, store.freezes)
}
