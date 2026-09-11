package spacecore

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
)

const (
	deletionRecoveryWindow = 7 * 24 * time.Hour
	fullEvidenceLifetime   = 30 * 24 * time.Hour
	tombstoneLifetime      = 365 * 24 * time.Hour
)

var canonicalParticipants = [...]commonv1.ParticipantId{
	commonv1.ParticipantId_PARTICIPANT_ID_ROLE,
	commonv1.ParticipantId_PARTICIPANT_ID_CHAT,
	commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING,
	commonv1.ParticipantId_PARTICIPANT_ID_FILE,
	commonv1.ParticipantId_PARTICIPANT_ID_VOICE,
	commonv1.ParticipantId_PARTICIPANT_ID_MATCHMAKING,
	commonv1.ParticipantId_PARTICIPANT_ID_SEARCH,
	commonv1.ParticipantId_PARTICIPANT_ID_SUBSCRIPTION,
	commonv1.ParticipantId_PARTICIPANT_ID_BOT,
	commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION,
}

var participantRequestPackages = map[commonv1.ParticipantId]string{
	commonv1.ParticipantId_PARTICIPANT_ID_ROLE:         "voice.role.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_CHAT:         "voice.chat.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING:    "voice.messaging.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_FILE:         "voice.file.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_VOICE:        "voice.calls.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_MATCHMAKING:  "voice.matchmaking.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_SEARCH:       "voice.search.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_SUBSCRIPTION: "voice.subscription.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_BOT:          "voice.bot.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION: "voice.notification.v1",
}

// LifecycleOutboxRecord is the stable event identity committed by a terminal
// aggregate transition. Persistence and delivery are owned by later layers.
type LifecycleOutboxRecord struct {
	EventID             string
	EventType           string
	SpaceID             string
	DeletionOperationID string
	Generation          uint64
	OccurredAt          time.Time
}

// LifecycleAggregate implements the pure deletion lifecycle barriers. It does
// not perform persistence, participant calls, or outbox delivery.
type LifecycleAggregate struct {
	spaceID             string
	deletionOperationID string
	phase               spacev1.SpaceDeletionPhase
	generation          uint64
	manifest            *commonv1.ManifestBinding
	scheduledAt         time.Time
	purgeAfter          time.Time
	purgeDecidedAt      time.Time
	fenceReceipts       map[commonv1.ParticipantId]*commonv1.SpaceLifecycleFenceReceipt
	expectedFenceHashes map[commonv1.ParticipantId][]byte
	purgeReceipts       map[commonv1.ParticipantId]*commonv1.SpacePurgeReceipt
	expectedPurgeHashes map[commonv1.ParticipantId][]byte
	roleReceipt         *rolev1.RetireSpaceReceipt
	expectedRoleHash    []byte
	localPurgeCompleted bool
	scheduleEvent       *LifecycleOutboxRecord
	restoreEvent        *LifecycleOutboxRecord
	deletedEvent        *LifecycleOutboxRecord
}

func NewLifecycleAggregate(spaceID, deletionOperationID string) (*LifecycleAggregate, error) {
	if err := validateCanonicalUUID("space ID", spaceID); err != nil {
		return nil, err
	}
	if err := validateCanonicalUUID("deletion operation ID", deletionOperationID); err != nil {
		return nil, err
	}
	return &LifecycleAggregate{
		spaceID:             spaceID,
		deletionOperationID: deletionOperationID,
		phase:               spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE,
	}, nil
}

func (a *LifecycleAggregate) Phase() spacev1.SpaceDeletionPhase {
	return a.phase
}

func (a *LifecycleAggregate) Generation() uint64 {
	return a.generation
}

func (a *LifecycleAggregate) PurgeAfter() time.Time {
	return a.purgeAfter
}

func (a *LifecycleAggregate) BeginSchedule(generation uint64) error {
	if a.phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE {
		return fmt.Errorf("begin schedule from phase %s", a.phase)
	}
	if generation == 0 || generation <= a.generation {
		return fmt.Errorf("schedule generation %d must be positive and greater than %d", generation, a.generation)
	}

	a.generation = generation
	a.phase = spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULE_PENDING
	a.manifest = nil
	a.scheduledAt = time.Time{}
	a.purgeAfter = time.Time{}
	a.purgeDecidedAt = time.Time{}
	a.fenceReceipts = nil
	a.expectedFenceHashes = nil
	a.purgeReceipts = nil
	a.expectedPurgeHashes = nil
	a.roleReceipt = nil
	a.expectedRoleHash = nil
	a.localPurgeCompleted = false
	a.scheduleEvent = nil
	a.restoreEvent = nil
	a.deletedEvent = nil
	return nil
}

func (a *LifecycleAggregate) BeginFreeze(manifest *commonv1.ManifestBinding) error {
	if a.phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULE_PENDING {
		return fmt.Errorf("begin freeze from phase %s", a.phase)
	}
	if err := validateManifest(manifest); err != nil {
		return err
	}
	expectedHashes, err := a.fenceRequestHashes(a.generation, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, manifest)
	if err != nil {
		return err
	}

	a.manifest = proto.Clone(manifest).(*commonv1.ManifestBinding)
	a.fenceReceipts = make(map[commonv1.ParticipantId]*commonv1.SpaceLifecycleFenceReceipt, len(canonicalParticipants))
	a.expectedFenceHashes = expectedHashes
	a.phase = spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING
	return nil
}

func (a *LifecycleAggregate) RecordFenceReceipt(receipt *commonv1.SpaceLifecycleFenceReceipt) error {
	if receipt == nil {
		return errors.New("fence receipt is required")
	}
	if !isCanonicalParticipant(receipt.GetParticipantId()) {
		return fmt.Errorf("participant %d is not canonical", receipt.GetParticipantId())
	}

	expectedState, ok := a.expectedFenceState()
	if !ok {
		return fmt.Errorf("fence receipt is not accepted in phase %s", a.phase)
	}
	if err := a.validateFenceReceipt(receipt, expectedState); err != nil {
		return err
	}
	if previous, exists := a.fenceReceipts[receipt.GetParticipantId()]; exists {
		if proto.Equal(previous, receipt) {
			return nil
		}
		return fmt.Errorf("participant %s already has a different fence receipt", receipt.GetParticipantId())
	}
	a.fenceReceipts[receipt.GetParticipantId()] = proto.Clone(receipt).(*commonv1.SpaceLifecycleFenceReceipt)
	return nil
}

func (a *LifecycleAggregate) CompleteSchedule(scheduledAt time.Time) (LifecycleOutboxRecord, error) {
	if a.phase == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULED && a.scheduleEvent != nil {
		return *a.scheduleEvent, nil
	}
	if a.phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING {
		return LifecycleOutboxRecord{}, fmt.Errorf("complete schedule from phase %s", a.phase)
	}
	if scheduledAt.IsZero() {
		return LifecycleOutboxRecord{}, errors.New("scheduled time is required")
	}
	if !a.hasEveryFenceReceipt() {
		return LifecycleOutboxRecord{}, errors.New("schedule fence barrier is incomplete")
	}

	a.scheduledAt = scheduledAt
	a.purgeAfter = scheduledAt.Add(deletionRecoveryWindow)
	a.phase = spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULED
	event := a.newOutboxRecord("space.deletion_scheduled", scheduledAt)
	a.scheduleEvent = &event
	return event, nil
}

func (a *LifecycleAggregate) DecideRecovery(decidedAt time.Time, generation uint64) (spacev1.SpaceDeletionPhase, error) {
	if a.phase == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED ||
		a.phase == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED {
		if generation == a.generation {
			return a.phase, nil
		}
		return a.phase, fmt.Errorf("recovery decision already committed at generation %d", a.generation)
	}
	if a.phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULED {
		return a.phase, fmt.Errorf("decide recovery from phase %s", a.phase)
	}
	if decidedAt.IsZero() {
		return a.phase, errors.New("decision time is required")
	}
	if generation == 0 || generation <= a.generation {
		return a.phase, fmt.Errorf("decision generation %d must be greater than %d", generation, a.generation)
	}

	nextPhase := spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED
	nextFenceState := commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	if decidedAt.Before(a.purgeAfter) {
		nextPhase = spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED
		nextFenceState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE
	}
	expectedHashes, err := a.fenceRequestHashes(generation, nextFenceState, a.manifest)
	if err != nil {
		return a.phase, err
	}

	a.generation = generation
	a.fenceReceipts = make(map[commonv1.ParticipantId]*commonv1.SpaceLifecycleFenceReceipt, len(canonicalParticipants))
	a.expectedFenceHashes = expectedHashes
	a.phase = nextPhase
	if nextPhase == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED {
		a.purgeDecidedAt = decidedAt
	}
	return a.phase, nil
}

func (a *LifecycleAggregate) CompleteRestore(restoredAt time.Time) (LifecycleOutboxRecord, error) {
	if a.phase == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE && a.restoreEvent != nil {
		return *a.restoreEvent, nil
	}
	if a.phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED {
		return LifecycleOutboxRecord{}, fmt.Errorf("complete restore from phase %s", a.phase)
	}
	if restoredAt.IsZero() {
		return LifecycleOutboxRecord{}, errors.New("restored time is required")
	}
	if !a.hasEveryFenceReceipt() {
		return LifecycleOutboxRecord{}, errors.New("restore fence barrier is incomplete")
	}

	a.phase = spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE
	event := a.newOutboxRecord("space.restored", restoredAt)
	a.restoreEvent = &event
	return event, nil
}

func (a *LifecycleAggregate) BeginPurging() error {
	if a.phase == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING {
		return nil
	}
	if a.phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED {
		return fmt.Errorf("begin purging from phase %s", a.phase)
	}
	if !a.hasEveryFenceReceipt() {
		return errors.New("purge decision fence barrier is incomplete")
	}
	expectedPurgeHashes, expectedRoleHash, err := a.purgeRequestHashes()
	if err != nil {
		return err
	}

	a.purgeReceipts = make(map[commonv1.ParticipantId]*commonv1.SpacePurgeReceipt, len(canonicalParticipants)-1)
	a.expectedPurgeHashes = expectedPurgeHashes
	a.expectedRoleHash = expectedRoleHash
	a.phase = spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING
	return nil
}

func (a *LifecycleAggregate) RecordPurgeReceipt(receipt *commonv1.SpacePurgeReceipt) error {
	if a.phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING {
		return fmt.Errorf("purge receipt is not accepted in phase %s", a.phase)
	}
	if receipt == nil {
		return errors.New("purge receipt is required")
	}
	participantID := receipt.GetParticipantId()
	if !isCanonicalParticipant(participantID) {
		return fmt.Errorf("participant %d is not canonical", participantID)
	}
	if participantID == commonv1.ParticipantId_PARTICIPANT_ID_ROLE {
		return errors.New("Role purge completion requires a typed retirement receipt")
	}
	if a.roleReceipt == nil {
		return errors.New("Role retirement must complete before participant purge")
	}
	if err := a.validatePurgeReceipt(receipt); err != nil {
		return err
	}
	if previous, exists := a.purgeReceipts[participantID]; exists {
		if proto.Equal(previous, receipt) {
			return nil
		}
		return fmt.Errorf("participant %s already has a different purge receipt", participantID)
	}
	a.purgeReceipts[participantID] = proto.Clone(receipt).(*commonv1.SpacePurgeReceipt)
	return nil
}

func (a *LifecycleAggregate) RecordRoleRetirementReceipt(receipt *rolev1.RetireSpaceReceipt) error {
	if a.phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING {
		return fmt.Errorf("Role retirement receipt is not accepted in phase %s", a.phase)
	}
	if receipt == nil {
		return errors.New("Role retirement receipt is required")
	}
	if err := a.validateRoleRetirementReceipt(receipt); err != nil {
		return err
	}
	if a.roleReceipt != nil {
		if proto.Equal(a.roleReceipt, receipt) {
			return nil
		}
		return errors.New("a different Role retirement receipt is already recorded")
	}
	a.roleReceipt = proto.Clone(receipt).(*rolev1.RetireSpaceReceipt)
	return nil
}

func (a *LifecycleAggregate) RecordLocalPurgeCompleted() error {
	if a.phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING {
		return fmt.Errorf("local purge completion is not accepted in phase %s", a.phase)
	}
	if a.roleReceipt == nil {
		return errors.New("Role retirement receipt is missing")
	}
	for _, participantID := range canonicalParticipants[1:] {
		if a.purgeReceipts[participantID] == nil {
			return fmt.Errorf("participant %s purge receipt is missing", participantID)
		}
	}
	a.localPurgeCompleted = true
	return nil
}

func (a *LifecycleAggregate) CompletePurge(purgedAt time.Time) (LifecycleOutboxRecord, error) {
	if a.phase == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED && a.deletedEvent != nil {
		return *a.deletedEvent, nil
	}
	if a.phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING {
		return LifecycleOutboxRecord{}, fmt.Errorf("complete purge from phase %s", a.phase)
	}
	if purgedAt.IsZero() || purgedAt.Before(a.purgeAfter) {
		return LifecycleOutboxRecord{}, errors.New("purged time must be at or after purge eligibility")
	}
	if !a.localPurgeCompleted {
		return LifecycleOutboxRecord{}, errors.New("local purge is incomplete")
	}
	if a.roleReceipt == nil {
		return LifecycleOutboxRecord{}, errors.New("Role retirement receipt is missing")
	}
	for _, participantID := range canonicalParticipants[1:] {
		if a.purgeReceipts[participantID] == nil {
			return LifecycleOutboxRecord{}, fmt.Errorf("participant %s purge receipt is missing", participantID)
		}
	}

	// PostgreSQL TIMESTAMPTZ stores microseconds. Normalize before creating the
	// stable event so restart replay and the tombstone retain the same instant.
	purgedAt = purgedAt.UTC().Truncate(time.Microsecond)
	a.phase = spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED
	event := a.newOutboxRecord("space.deleted", purgedAt)
	a.deletedEvent = &event
	return event, nil
}

func (a *LifecycleAggregate) expectedFenceState() (commonv1.LifecycleFenceState, bool) {
	switch a.phase {
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING:
		return commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, true
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED:
		return commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, true
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED:
		return commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED, true
	default:
		return commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_UNSPECIFIED, false
	}
}

func (a *LifecycleAggregate) validateFenceReceipt(receipt *commonv1.SpaceLifecycleFenceReceipt, expectedState commonv1.LifecycleFenceState) error {
	if receipt.GetProtocolVersion() != 1 || receipt.GetReceiptId() == "" {
		return errors.New("fence receipt protocol version and receipt ID are required")
	}
	if receipt.GetSpaceId() != a.spaceID || receipt.GetDeletionOperationId() != a.deletionOperationID {
		return errors.New("fence receipt operation binding does not match")
	}
	if receipt.GetGeneration() != a.generation || receipt.GetAppliedState() != expectedState {
		return errors.New("fence receipt generation or applied state does not match")
	}
	if !bytes.Equal(receipt.GetRequestSha256(), a.expectedFenceHashes[receipt.GetParticipantId()]) ||
		!bytes.Equal(receipt.GetManifestSha256(), a.manifest.GetManifestSha256()) {
		return errors.New("fence receipt hash binding does not match")
	}
	if receipt.GetAppliedAt() == nil || receipt.GetAppliedAt().CheckValid() != nil {
		return errors.New("fence receipt applied time is invalid")
	}
	return nil
}

func (a *LifecycleAggregate) validatePurgeReceipt(receipt *commonv1.SpacePurgeReceipt) error {
	if receipt.GetProtocolVersion() != 1 || receipt.GetReceiptId() == "" {
		return errors.New("purge receipt protocol version and receipt ID are required")
	}
	if receipt.GetSpaceId() != a.spaceID || receipt.GetDeletionOperationId() != a.deletionOperationID {
		return errors.New("purge receipt operation binding does not match")
	}
	if receipt.GetGeneration() != a.generation || receipt.GetState() != commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED {
		return errors.New("purge receipt generation or completion state does not match")
	}
	if !bytes.Equal(receipt.GetRequestSha256(), a.expectedPurgeHashes[receipt.GetParticipantId()]) {
		return errors.New("purge receipt request hash binding does not match")
	}
	if receipt.GetCompletedAt() == nil || receipt.GetCompletedAt().CheckValid() != nil {
		return errors.New("purge receipt completion time is invalid")
	}
	return nil
}

func (a *LifecycleAggregate) validateRoleRetirementReceipt(receipt *rolev1.RetireSpaceReceipt) error {
	if receipt.GetProtocolVersion() != 1 || receipt.GetReceiptId() == "" {
		return errors.New("Role retirement receipt protocol version and receipt ID are required")
	}
	if receipt.GetSpaceId() != a.spaceID || receipt.GetDeletionOperationId() != a.deletionOperationID {
		return errors.New("Role retirement receipt operation binding does not match")
	}
	if receipt.GetGeneration() != a.generation || receipt.GetState() != rolev1.RoleRetirementState_ROLE_RETIREMENT_STATE_RETIRED {
		return errors.New("Role retirement receipt generation or state does not match")
	}
	if !bytes.Equal(receipt.GetRequestSha256(), a.expectedRoleHash) ||
		!bytes.Equal(receipt.GetManifestSha256(), a.manifest.GetManifestSha256()) {
		return errors.New("Role retirement receipt hash binding does not match")
	}
	if receipt.GetRetiredAt() == nil || receipt.GetRetiredAt().CheckValid() != nil {
		return errors.New("Role retirement time is invalid")
	}
	return nil
}

func (a *LifecycleAggregate) hasEveryFenceReceipt() bool {
	for _, participantID := range canonicalParticipants {
		if a.fenceReceipts[participantID] == nil {
			return false
		}
	}
	return true
}

func (a *LifecycleAggregate) newOutboxRecord(eventType string, occurredAt time.Time) LifecycleOutboxRecord {
	return LifecycleOutboxRecord{
		EventID:             uuid.NewSHA1(uuid.MustParse(a.deletionOperationID), []byte(fmt.Sprintf("%s\x00%s\x00%d", a.spaceID, eventType, a.generation))).String(),
		EventType:           eventType,
		SpaceID:             a.spaceID,
		DeletionOperationID: a.deletionOperationID,
		Generation:          a.generation,
		OccurredAt:          occurredAt,
	}
}

func (a *LifecycleAggregate) fenceRequestHashes(generation uint64, state commonv1.LifecycleFenceState, manifest *commonv1.ManifestBinding) (map[commonv1.ParticipantId][]byte, error) {
	hashes := make(map[commonv1.ParticipantId][]byte, len(canonicalParticipants))
	request := &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion:     1,
		SpaceId:             a.spaceID,
		DeletionOperationId: a.deletionOperationID,
		Generation:          generation,
		DesiredState:        state,
		Manifest:            proto.Clone(manifest).(*commonv1.ManifestBinding),
	}
	for _, participantID := range canonicalParticipants {
		hash, err := computeWrappedRequestSHA256(participantRequestPackages[participantID]+".ApplySpaceLifecycleFenceRequest", request)
		if err != nil {
			return nil, fmt.Errorf("hash participant %s fence request: %w", participantID, err)
		}
		hashes[participantID] = hash
	}
	return hashes, nil
}

func (a *LifecycleAggregate) purgeRequestHashes() (map[commonv1.ParticipantId][]byte, []byte, error) {
	hashes := make(map[commonv1.ParticipantId][]byte, len(canonicalParticipants)-1)
	for _, participantID := range canonicalParticipants[1:] {
		request := &commonv1.SpacePurgeRequest{
			ProtocolVersion:     1,
			SpaceId:             a.spaceID,
			DeletionOperationId: a.deletionOperationID,
			Generation:          a.generation,
			PurgeDecidedAt:      timestamppb.New(a.purgeDecidedAt),
			ParticipantId:       participantID,
			Manifest:            proto.Clone(a.manifest).(*commonv1.ManifestBinding),
		}
		hash, err := computeWrappedRequestSHA256(participantRequestPackages[participantID]+".PurgeSpaceRequest", request)
		if err != nil {
			return nil, nil, fmt.Errorf("hash participant %s purge request: %w", participantID, err)
		}
		hashes[participantID] = hash
	}

	roleRequest := &rolev1.RetireSpaceRequest{
		ProtocolVersion:     1,
		SpaceId:             a.spaceID,
		DeletionOperationId: a.deletionOperationID,
		Generation:          a.generation,
		PurgeDecidedAt:      timestamppb.New(a.purgeDecidedAt),
		Manifest:            proto.Clone(a.manifest).(*commonv1.ManifestBinding),
	}
	roleHash, err := computeDeterministicMessageSHA256(roleRequest)
	if err != nil {
		return nil, nil, fmt.Errorf("hash Role retirement request: %w", err)
	}
	return hashes, roleHash, nil
}

func computeWrappedRequestSHA256(wrapperFQN string, request proto.Message) ([]byte, error) {
	requestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return nil, err
	}
	wrapperBytes := protowire.AppendTag(nil, 1, protowire.BytesType)
	wrapperBytes = protowire.AppendBytes(wrapperBytes, requestBytes)
	return computeDomainSeparatedSHA256(wrapperFQN, wrapperBytes), nil
}

func computeDeterministicMessageSHA256(message proto.Message) ([]byte, error) {
	messageBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	if err != nil {
		return nil, err
	}
	return computeDomainSeparatedSHA256(string(message.ProtoReflect().Descriptor().FullName()), messageBytes), nil
}

func computeDomainSeparatedSHA256(fqn string, deterministicWire []byte) []byte {
	payload := make([]byte, 0, len(fqn)+1+len(deterministicWire))
	payload = append(payload, fqn...)
	payload = append(payload, 0)
	payload = append(payload, deterministicWire...)
	hash := sha256.Sum256(payload)
	return bytes.Clone(hash[:])
}

func validateManifest(manifest *commonv1.ManifestBinding) error {
	if manifest == nil {
		return errors.New("manifest binding is required")
	}
	if manifest.GetManifestId() == "" {
		return errors.New("manifest ID is required")
	}
	if len(manifest.GetManifestSha256()) != sha256.Size {
		return errors.New("manifest SHA-256 must contain 32 bytes")
	}
	if len(manifest.ProtoReflect().GetUnknown()) != 0 {
		return errors.New("manifest binding contains unknown fields")
	}
	return nil
}

func validateCanonicalUUID(label, value string) error {
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil || parsed.String() != value {
		return fmt.Errorf("%s must be a canonical non-zero UUID", label)
	}
	return nil
}

func isCanonicalParticipant(participantID commonv1.ParticipantId) bool {
	switch participantID {
	case commonv1.ParticipantId_PARTICIPANT_ID_ROLE,
		commonv1.ParticipantId_PARTICIPANT_ID_CHAT,
		commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING,
		commonv1.ParticipantId_PARTICIPANT_ID_FILE,
		commonv1.ParticipantId_PARTICIPANT_ID_VOICE,
		commonv1.ParticipantId_PARTICIPANT_ID_MATCHMAKING,
		commonv1.ParticipantId_PARTICIPANT_ID_SEARCH,
		commonv1.ParticipantId_PARTICIPANT_ID_SUBSCRIPTION,
		commonv1.ParticipantId_PARTICIPANT_ID_BOT,
		commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION:
		return true
	default:
		return false
	}
}

// LifecycleRetention contains the database-time eligibility predicates for
// bounded coordinator evidence and the minimal Space tombstone.
type LifecycleRetention struct{}

func CanonicalLifecycleRetention() LifecycleRetention {
	return LifecycleRetention{}
}

func (LifecycleRetention) KeepOperationEvidence(now time.Time, completedAt *time.Time) bool {
	return keepBoundedEvidence(now, completedAt)
}

func (LifecycleRetention) KeepCoordinatorParticipantEvidence(now time.Time, purgedAt *time.Time) bool {
	return keepBoundedEvidence(now, purgedAt)
}

func (LifecycleRetention) KeepDeliveredOutboxEvidence(now time.Time, deliveredAt *time.Time) bool {
	return keepBoundedEvidence(now, deliveredAt)
}

func (LifecycleRetention) KeepTombstone(now time.Time, phase spacev1.SpaceDeletionPhase, purgedAt *time.Time) bool {
	return phase == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED &&
		purgedAt != nil && now.Before(purgedAt.Add(tombstoneLifetime))
}

func keepBoundedEvidence(now time.Time, terminalAnchor *time.Time) bool {
	return terminalAnchor == nil || now.Before(terminalAnchor.Add(fullEvidenceLifetime))
}
