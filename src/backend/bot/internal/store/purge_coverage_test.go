package store

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func purgeCoverageSample(t *testing.T) (BotPurgeCoverageProof, []string) {
	t.Helper()
	digest := sha256.Sum256([]byte("sealed source manifest"))
	p := &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: uuid.NewString(), DeletionOperationId: uuid.NewString(), Generation: 4, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_BOT, PurgeDecidedAt: timestamppb.Now(), Manifest: &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: digest[:], ItemCount: 1}}
	read := &spacev1.GetBotPurgeHandoffRequest{ProtocolVersion: 1, RequestId: uuid.NewString(), SpaceId: p.SpaceId, DeletionOperationId: p.DeletionOperationId, Generation: p.Generation, Manifest: proto.Clone(p.Manifest).(*commonv1.ManifestBinding)}
	roleq := &rolev1.RetireSpaceRequest{ProtocolVersion: 1, SpaceId: p.SpaceId, DeletionOperationId: p.DeletionOperationId, Generation: p.Generation, PurgeDecidedAt: proto.Clone(p.PurgeDecidedAt).(*timestamppb.Timestamp), Manifest: proto.Clone(p.Manifest).(*commonv1.ManifestBinding)}
	rolebytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(roleq)
	require.NoError(t, err)
	rolehash := lifecycleHash("voice.role.v1.RetireSpaceRequest", rolebytes)
	roler := &rolev1.RetireSpaceReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: p.SpaceId, DeletionOperationId: p.DeletionOperationId, Generation: p.Generation, State: rolev1.RoleRetirementState_ROLE_RETIREMENT_STATE_RETIRED, RequestSha256: rolehash[:], ManifestSha256: p.Manifest.ManifestSha256, RetiredAt: timestamppb.Now()}
	rolerbytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(roler)
	require.NoError(t, err)
	rolerhash := lifecycleHash("voice.role.v1.RetireSpaceReceipt", rolerbytes)
	msgq := proto.Clone(p).(*commonv1.SpacePurgeRequest)
	msgq.ParticipantId = commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING
	msgbytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(&messagingv1.PurgeSpaceRequest{Purge: msgq})
	require.NoError(t, err)
	msghash := lifecycleHash("voice.messaging.v1.PurgeSpaceRequest", msgbytes)
	msgr := &commonv1.SpacePurgeReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: p.SpaceId, DeletionOperationId: p.DeletionOperationId, Generation: p.Generation, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING, State: commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, RequestSha256: msghash[:], CompletedAt: timestamppb.Now()}
	msgrbytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(msgr)
	require.NoError(t, err)
	msgrhash := lifecycleHash("voice.common.v1.SpacePurgeReceipt", msgrbytes)
	response := &spacev1.GetBotPurgeHandoffResponse{ProtocolVersion: 1, RequestId: read.RequestId, SpaceId: p.SpaceId, DeletionOperationId: p.DeletionOperationId, Generation: p.Generation, Manifest: proto.Clone(p.Manifest).(*commonv1.ManifestBinding), Role: &spacev1.BotPurgeRoleHandoff{OriginalReceipt: roler, OriginalRequestBytes: rolebytes, OriginalRequestSha256: rolehash[:], OriginalReceiptBytes: rolerbytes, OriginalReceiptSha256: rolerhash[:]}, Messaging: &spacev1.BotPurgeMessagingHandoff{OriginalReceipt: msgr, OriginalRequestBytes: msgbytes, OriginalRequestSha256: msghash[:], OriginalReceiptBytes: msgrbytes, OriginalReceiptSha256: msgrhash[:]}}
	chats := []string{uuid.NewString()}
	cq := &messagingv1.GetBotPurgeChatCoverageRequest{ProtocolVersion: 1, RequestId: uuid.NewString(), SpaceId: p.SpaceId, DeletionOperationId: p.DeletionOperationId, Generation: p.Generation, Manifest: proto.Clone(p.Manifest).(*commonv1.ManifestBinding), ChatIds: chats}
	h, err := principal.RequestHash(cq)
	require.NoError(t, err)
	rawhash, err := hex.DecodeString(h[len("sha256:"):])
	require.NoError(t, err)
	cr := &messagingv1.GetBotPurgeChatCoverageResponse{ProtocolVersion: 1, RequestId: cq.RequestId, SpaceId: p.SpaceId, DeletionOperationId: p.DeletionOperationId, Generation: p.Generation, Manifest: proto.Clone(p.Manifest).(*commonv1.ManifestBinding), SourceScheduleGeneration: 7, Coverage: []*messagingv1.BotPurgeChatCoverage{{ChatId: chats[0], Covered: true}}, RequestSha256: rawhash}
	return BotPurgeCoverageProof{Purge: p, HandoffRequest: read, Handoff: response, CoverageRequest: cq, Coverage: cr}, chats
}

func TestBotPurgeCoverageRequiresExactOriginalsAndManifestMembership(t *testing.T) {
	sample, chats := purgeCoverageSample(t)
	owned, err := ValidateBotPurgeCoverage(sample, chats)
	require.NoError(t, err)
	original := proto.Clone(owned.Handoff)
	sample.Handoff.Role.OriginalRequestBytes[0] ^= 1
	sample.Handoff.Manifest.ManifestSha256[0] ^= 1
	sample.Coverage.Coverage[0].Covered = false
	require.True(t, proto.Equal(original, owned.Handoff))
	require.True(t, owned.Coverage.Coverage[0].Covered)
	for name, mutate := range map[string]func(*BotPurgeCoverageProof){
		"wrong operation":  func(p *BotPurgeCoverageProof) { p.Handoff.DeletionOperationId = uuid.NewString() },
		"wrong manifest":   func(p *BotPurgeCoverageProof) { p.Handoff.Manifest.ManifestSha256[0] ^= 1 },
		"corrupt original": func(p *BotPurgeCoverageProof) { p.Handoff.Role.OriginalRequestBytes[0] ^= 1 },
		"receipt differs":  func(p *BotPurgeCoverageProof) { p.Handoff.Role.OriginalReceipt.ReceiptId = uuid.NewString() },
		"receipt domain": func(p *BotPurgeCoverageProof) {
			h := lifecycleHash("voice.messaging.v1.SpacePurgeReceipt", p.Handoff.Messaging.OriginalReceiptBytes)
			p.Handoff.Messaging.OriginalReceiptSha256 = h[:]
		},
		"inner Messaging body": func(p *BotPurgeCoverageProof) {
			q := new(messagingv1.PurgeSpaceRequest)
			require.NoError(t, proto.Unmarshal(p.Handoff.Messaging.OriginalRequestBytes, q))
			raw, e := (proto.MarshalOptions{Deterministic: true}).Marshal(q.Purge)
			require.NoError(t, e)
			p.Handoff.Messaging.OriginalRequestBytes = raw
			h := lifecycleHash("voice.messaging.v1.PurgeSpaceRequest", raw)
			p.Handoff.Messaging.OriginalRequestSha256 = h[:]
		},
		"unknown original": func(p *BotPurgeCoverageProof) {
			p.Handoff.Role.OriginalReceipt.ProtoReflect().SetUnknown([]byte{0x78, 1})
		},
		"foreign chat":           func(p *BotPurgeCoverageProof) { p.Coverage.Coverage[0].ChatId = uuid.NewString() },
		"wrong observation hash": func(p *BotPurgeCoverageProof) { p.Coverage.RequestSha256[0] ^= 1 },
		"schedule absent":        func(p *BotPurgeCoverageProof) { p.Coverage.SourceScheduleGeneration = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			p, chats := purgeCoverageSample(t)
			mutate(&p)
			_, err := ValidateBotPurgeCoverage(p, chats)
			require.ErrorIs(t, err, ErrSpaceLifecycleConflict)
		})
	}
	for name, mutate := range map[string]func(*BotPurgeCoverageProof){
		"Role absent":       func(p *BotPurgeCoverageProof) { p.Handoff.Role = nil },
		"Messaging absent":  func(p *BotPurgeCoverageProof) { p.Handoff.Messaging = nil },
		"membership absent": func(p *BotPurgeCoverageProof) { p.Coverage = nil },
		"not covered":       func(p *BotPurgeCoverageProof) { p.Coverage.Coverage[0].Covered = false },
	} {
		t.Run(name, func(t *testing.T) {
			p, chats := purgeCoverageSample(t)
			mutate(&p)
			_, err := ValidateBotPurgeCoverage(p, chats)
			require.ErrorIs(t, err, ErrDeletionPending)
		})
	}
	p, _ := purgeCoverageSample(t)
	p.CoverageRequest = nil
	p.Coverage = nil
	p.Handoff.Messaging = nil
	_, err = ValidateBotPurgeCoverage(p, nil)
	require.NoError(t, err, "Role-only source has no Messaging target to invent")
}
