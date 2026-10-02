package lifecyclecoord

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/spacecore"
)

func TestFenceBarrierPersistsPartialProgressAndResumesExactRequests(t *testing.T) {
	ctx := context.Background()
	aggregate, err := spacecore.NewLifecycleAggregate("11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222")
	require.NoError(t, err)
	require.NoError(t, aggregate.BeginSchedule(1))
	require.NoError(t, aggregate.BeginFreeze(&commonv1.ManifestBinding{ManifestId: "33333333-3333-4333-8333-333333333333", ManifestSha256: make([]byte, sha256.Size), ItemCount: 9}))
	spaceID, err := uuid.Parse(aggregate.Snapshot().SpaceID)
	require.NoError(t, err)
	store := &memoryLifecycleStore{aggregate: aggregate}
	participants := &recordingParticipants{failOnce: commonv1.ParticipantId_PARTICIPANT_ID_VOICE}
	participantClients := make(map[commonv1.ParticipantId]FenceParticipant)
	for _, id := range spacecore.CanonicalLifecycleParticipants() {
		participantClients[id] = recordingParticipant{owner: participants, id: id}
	}
	coordinator := New(Dependencies{Store: store, Participants: participantClients})

	_, _, err = coordinator.ApplyFenceBarrier(ctx, spaceID)
	require.Error(t, err)
	require.Equal(t, 4, len(store.receipts))
	require.Zero(t, store.scheduleCompletions)
	firstRequests := cloneRequests(participants.requests)

	_, _, err = coordinator.ApplyFenceBarrier(ctx, spaceID)
	require.NoError(t, err)
	require.Equal(t, len(spacecore.CanonicalLifecycleParticipants()), len(store.receipts))
	require.Equal(t, 1, store.scheduleCompletions)
	for id, request := range firstRequests {
		require.True(t, proto.Equal(request, participants.requests[id]), "participant %s retry changed its fence request", id)
	}
	for _, participantID := range spacecore.CanonicalLifecycleParticipants() {
		wantCalls := 1
		if participantID == commonv1.ParticipantId_PARTICIPANT_ID_VOICE {
			wantCalls = 2
		}
		require.Equal(t, wantCalls, participants.calls[participantID], "participant %s had an unexpected retry count", participantID)
	}
}

func TestPurgeBarrierRetiresRoleFirstAndResumesExactRequests(t *testing.T) {
	ctx := context.Background()
	aggregate := purgingAggregate(t)
	spaceID, err := uuid.Parse(aggregate.Snapshot().SpaceID)
	require.NoError(t, err)
	store := &memoryLifecycleStore{aggregate: aggregate}
	participants := &recordingPurgeParticipants{failOnce: commonv1.ParticipantId_PARTICIPANT_ID_FILE}
	purgeClients := make(map[commonv1.ParticipantId]PurgeParticipant)
	for _, id := range spacecore.CanonicalLifecycleParticipants()[1:] {
		purgeClients[id] = recordingPurgeParticipant{owner: participants, id: id}
	}
	role := &recordingRoleRetirement{owner: participants}
	coordinator := New(Dependencies{Store: store, RoleRetirement: role, PurgeParticipants: purgeClients, FileProducer: recordingSpaceFileProducer{}})

	_, err = coordinator.ApplyPurgeBarrier(ctx, spaceID)
	require.Error(t, err)
	require.Equal(t, []string{commonv1.ParticipantId_PARTICIPANT_ID_ROLE.String(), commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING.String(), commonv1.ParticipantId_PARTICIPANT_ID_CHAT.String(), commonv1.ParticipantId_PARTICIPANT_ID_FILE.String()}, participants.order)
	require.NotNil(t, store.aggregate.Snapshot().RoleReceipt)
	require.Len(t, store.aggregate.Snapshot().PurgeReceipts, 2)
	firstRequests := clonePurgeRequests(participants.requests)

	result, err := coordinator.ApplyPurgeBarrier(ctx, spaceID)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING, result.Phase())
	require.Equal(t, []string{
		commonv1.ParticipantId_PARTICIPANT_ID_ROLE.String(),
		commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING.String(),
		commonv1.ParticipantId_PARTICIPANT_ID_CHAT.String(),
		commonv1.ParticipantId_PARTICIPANT_ID_FILE.String(),
		commonv1.ParticipantId_PARTICIPANT_ID_FILE.String(),
		commonv1.ParticipantId_PARTICIPANT_ID_VOICE.String(),
		commonv1.ParticipantId_PARTICIPANT_ID_MATCHMAKING.String(),
		commonv1.ParticipantId_PARTICIPANT_ID_SEARCH.String(),
		commonv1.ParticipantId_PARTICIPANT_ID_SUBSCRIPTION.String(),
		commonv1.ParticipantId_PARTICIPANT_ID_BOT.String(),
		commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION.String(),
	}, participants.order)
	require.Len(t, result.Snapshot().PurgeReceipts, 9)
	for id, request := range firstRequests {
		if id != commonv1.ParticipantId_PARTICIPANT_ID_FILE {
			require.True(t, proto.Equal(request, participants.requests[id]), "participant %s retry changed its purge request", id)
		}
	}
}

type recordingSpaceFileProducer struct{}

func (recordingSpaceFileProducer) ReleaseSpaceFileProducer(context.Context, spacecore.LifecycleSnapshot) error {
	return nil
}

func purgingAggregate(t *testing.T) *spacecore.LifecycleAggregate {
	t.Helper()
	aggregate, err := spacecore.NewLifecycleAggregate("11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222")
	require.NoError(t, err)
	manifest := &commonv1.ManifestBinding{ManifestId: "33333333-3333-4333-8333-333333333333", ManifestSha256: make([]byte, sha256.Size), ItemCount: 9}
	require.NoError(t, aggregate.BeginSchedule(1))
	require.NoError(t, aggregate.BeginFreeze(manifest))
	for _, id := range spacecore.CanonicalLifecycleParticipants() {
		request, err := aggregate.FenceRequest(id)
		require.NoError(t, err)
		receipt := &commonv1.SpaceLifecycleFenceReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: request.GetSpaceId(), DeletionOperationId: request.GetDeletionOperationId(), Generation: request.GetGeneration(), ParticipantId: id, AppliedState: request.GetDesiredState(), RequestSha256: wrappedHash(tParticipantPackage(id)+".ApplySpaceLifecycleFenceRequest", request), ManifestSha256: manifest.GetManifestSha256(), AppliedAt: timestamppb.Now()}
		require.NoError(t, aggregate.RecordFenceReceipt(receipt))
	}
	_, err = aggregate.CompleteSchedule(time.Now().UTC())
	require.NoError(t, err)
	decisionAt := aggregate.PurgeAfter().Add(time.Hour)
	_, err = aggregate.DecideRecovery(decisionAt, 2)
	require.NoError(t, err)
	for _, id := range spacecore.CanonicalLifecycleParticipants() {
		request, err := aggregate.FenceRequest(id)
		require.NoError(t, err)
		receipt := &commonv1.SpaceLifecycleFenceReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: request.GetSpaceId(), DeletionOperationId: request.GetDeletionOperationId(), Generation: request.GetGeneration(), ParticipantId: id, AppliedState: request.GetDesiredState(), RequestSha256: wrappedHash(tParticipantPackage(id)+".ApplySpaceLifecycleFenceRequest", request), ManifestSha256: manifest.GetManifestSha256(), AppliedAt: timestamppb.Now()}
		require.NoError(t, aggregate.RecordFenceReceipt(receipt))
	}
	require.NoError(t, aggregate.BeginPurging())
	return aggregate
}

func clonePurgeRequests(source map[commonv1.ParticipantId]*commonv1.SpacePurgeRequest) map[commonv1.ParticipantId]*commonv1.SpacePurgeRequest {
	out := make(map[commonv1.ParticipantId]*commonv1.SpacePurgeRequest, len(source))
	for id, request := range source {
		out[id] = proto.Clone(request).(*commonv1.SpacePurgeRequest)
	}
	return out
}

type memoryLifecycleStore struct {
	aggregate           *spacecore.LifecycleAggregate
	receipts            map[commonv1.ParticipantId]*commonv1.SpaceLifecycleFenceReceipt
	scheduleCompletions int
}

func (s *memoryLifecycleStore) LoadLifecycle(context.Context, uuid.UUID) (*spacecore.LifecycleAggregate, error) {
	return spacecore.RestoreLifecycleAggregate(s.aggregate.Snapshot())
}

func (s *memoryLifecycleStore) RecordLifecycleFenceReceipt(_ context.Context, receipt *commonv1.SpaceLifecycleFenceReceipt) (*spacecore.LifecycleAggregate, error) {
	if s.receipts == nil {
		s.receipts = map[commonv1.ParticipantId]*commonv1.SpaceLifecycleFenceReceipt{}
	}
	if err := s.aggregate.RecordFenceReceipt(receipt); err != nil {
		return nil, err
	}
	s.receipts[receipt.GetParticipantId()] = proto.Clone(receipt).(*commonv1.SpaceLifecycleFenceReceipt)
	return spacecore.RestoreLifecycleAggregate(s.aggregate.Snapshot())
}

func (s *memoryLifecycleStore) CompleteLifecycleSchedule(_ context.Context, _ uuid.UUID) (*spacecore.LifecycleAggregate, spacecore.LifecycleOutboxRecord, error) {
	s.scheduleCompletions++
	event, err := s.aggregate.CompleteSchedule(time.Now().UTC())
	// event is deliberately returned only after the aggregate barrier succeeds.
	// Keep fake behavior equivalent to PostgreSQL's fresh-clock completion.
	restored, restoreErr := spacecore.RestoreLifecycleAggregate(s.aggregate.Snapshot())
	return restored, event, errors.Join(err, restoreErr)
}

func (s *memoryLifecycleStore) CompleteLifecycleRestore(context.Context, uuid.UUID) (*spacecore.LifecycleAggregate, spacecore.LifecycleOutboxRecord, error) {
	return s.aggregate, spacecore.LifecycleOutboxRecord{}, errors.New("unexpected restore completion")
}

func (s *memoryLifecycleStore) PersistLifecycle(_ context.Context, aggregate *spacecore.LifecycleAggregate) error {
	s.aggregate = aggregate
	return nil
}

func (s *memoryLifecycleStore) RecordLifecycleRoleRetirementReceipt(_ context.Context, receipt *rolev1.RetireSpaceReceipt) (*spacecore.LifecycleAggregate, error) {
	if err := s.aggregate.RecordRoleRetirementReceipt(receipt); err != nil {
		return nil, err
	}
	return spacecore.RestoreLifecycleAggregate(s.aggregate.Snapshot())
}

func (s *memoryLifecycleStore) RecordLifecyclePurgeReceipt(_ context.Context, receipt *commonv1.SpacePurgeReceipt) (*spacecore.LifecycleAggregate, error) {
	if err := s.aggregate.RecordPurgeReceipt(receipt); err != nil {
		return nil, err
	}
	return spacecore.RestoreLifecycleAggregate(s.aggregate.Snapshot())
}

type recordingPurgeParticipants struct {
	failOnce commonv1.ParticipantId
	failed   bool
	order    []string
	requests map[commonv1.ParticipantId]*commonv1.SpacePurgeRequest
}

type recordingPurgeParticipant struct {
	owner *recordingPurgeParticipants
	id    commonv1.ParticipantId
}

func (p recordingPurgeParticipant) PurgeSpace(ctx context.Context, request *commonv1.SpacePurgeRequest) (*commonv1.SpacePurgeReceipt, error) {
	return p.owner.PurgeSpace(ctx, p.id, request)
}
func (p *recordingPurgeParticipants) PurgeSpace(_ context.Context, id commonv1.ParticipantId, request *commonv1.SpacePurgeRequest) (*commonv1.SpacePurgeReceipt, error) {
	p.order = append(p.order, id.String())
	if p.requests == nil {
		p.requests = map[commonv1.ParticipantId]*commonv1.SpacePurgeRequest{}
	}
	p.requests[id] = proto.Clone(request).(*commonv1.SpacePurgeRequest)
	if id == p.failOnce && !p.failed {
		p.failed = true
		return nil, context.DeadlineExceeded
	}
	return &commonv1.SpacePurgeReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: request.GetSpaceId(), DeletionOperationId: request.GetDeletionOperationId(), Generation: request.GetGeneration(), ParticipantId: id, State: commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, RequestSha256: wrappedHash(tParticipantPackage(id)+".PurgeSpaceRequest", request), CompletedAt: timestamppb.Now()}, nil
}

type recordingRoleRetirement struct{ owner *recordingPurgeParticipants }

func (p *recordingRoleRetirement) RetireSpace(_ context.Context, request *rolev1.RetireSpaceRequest) (*rolev1.RetireSpaceReceipt, error) {
	p.owner.order = append(p.owner.order, commonv1.ParticipantId_PARTICIPANT_ID_ROLE.String())
	requestBytes, _ := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	digest := sha256.Sum256(append(append([]byte("voice.role.v1.RetireSpaceRequest"), 0), requestBytes...))
	return &rolev1.RetireSpaceReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: request.GetSpaceId(), DeletionOperationId: request.GetDeletionOperationId(), Generation: request.GetGeneration(), State: rolev1.RoleRetirementState_ROLE_RETIREMENT_STATE_RETIRED, RequestSha256: digest[:], ManifestSha256: request.GetManifest().GetManifestSha256(), RetiredAt: timestamppb.Now()}, nil
}

type recordingParticipants struct {
	failOnce commonv1.ParticipantId
	failed   bool
	calls    map[commonv1.ParticipantId]int
	requests map[commonv1.ParticipantId]*commonv1.SpaceLifecycleFenceRequest
}

type recordingParticipant struct {
	owner *recordingParticipants
	id    commonv1.ParticipantId
}

func (participant recordingParticipant) ApplySpaceLifecycleFence(ctx context.Context, request *commonv1.SpaceLifecycleFenceRequest) (*commonv1.SpaceLifecycleFenceReceipt, error) {
	return participant.owner.ApplySpaceLifecycleFence(ctx, participant.id, request)
}

func (p *recordingParticipants) ApplySpaceLifecycleFence(_ context.Context, participantID commonv1.ParticipantId, request *commonv1.SpaceLifecycleFenceRequest) (*commonv1.SpaceLifecycleFenceReceipt, error) {
	if p.calls == nil {
		p.calls = map[commonv1.ParticipantId]int{}
	}
	if p.requests == nil {
		p.requests = map[commonv1.ParticipantId]*commonv1.SpaceLifecycleFenceRequest{}
	}
	p.calls[participantID]++
	p.requests[participantID] = proto.Clone(request).(*commonv1.SpaceLifecycleFenceRequest)
	if participantID == p.failOnce && !p.failed {
		p.failed = true
		return nil, context.DeadlineExceeded
	}
	hash := wrappedHash(tParticipantPackage(participantID)+".ApplySpaceLifecycleFenceRequest", request)
	return &commonv1.SpaceLifecycleFenceReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: request.GetSpaceId(), DeletionOperationId: request.GetDeletionOperationId(), Generation: request.GetGeneration(), ParticipantId: participantID, AppliedState: request.GetDesiredState(), RequestSha256: hash, ManifestSha256: request.GetManifest().GetManifestSha256(), AppliedAt: timestamppb.Now()}, nil
}

func tParticipantPackage(id commonv1.ParticipantId) string {
	return map[commonv1.ParticipantId]string{
		commonv1.ParticipantId_PARTICIPANT_ID_ROLE: "voice.role.v1", commonv1.ParticipantId_PARTICIPANT_ID_CHAT: "voice.chat.v1",
		commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING: "voice.messaging.v1", commonv1.ParticipantId_PARTICIPANT_ID_FILE: "voice.file.v1",
		commonv1.ParticipantId_PARTICIPANT_ID_VOICE: "voice.calls.v1", commonv1.ParticipantId_PARTICIPANT_ID_MATCHMAKING: "voice.matchmaking.v1",
		commonv1.ParticipantId_PARTICIPANT_ID_SEARCH: "voice.search.v1", commonv1.ParticipantId_PARTICIPANT_ID_SUBSCRIPTION: "voice.subscription.v1",
		commonv1.ParticipantId_PARTICIPANT_ID_BOT: "voice.bot.v1", commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION: "voice.notification.v1",
	}[id]
}

func wrappedHash(fqn string, request proto.Message) []byte {
	payload, _ := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	wrapped := protowire.AppendBytes(protowire.AppendTag(nil, 1, protowire.BytesType), payload)
	digest := sha256.Sum256(append(append([]byte(fqn), 0), wrapped...))
	return digest[:]
}

func cloneRequests(source map[commonv1.ParticipantId]*commonv1.SpaceLifecycleFenceRequest) map[commonv1.ParticipantId]*commonv1.SpaceLifecycleFenceRequest {
	out := make(map[commonv1.ParticipantId]*commonv1.SpaceLifecycleFenceRequest, len(source))
	for id, request := range source {
		out[id] = proto.Clone(request).(*commonv1.SpaceLifecycleFenceRequest)
	}
	return out
}
