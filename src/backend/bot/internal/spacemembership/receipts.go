// Receipt validation ported from the published Bot producer 5c5c94df4.
// Only the native space_uninstall caller is connected by this package.
package spacemembership

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"math"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
)

var errDeletionEvidence = errors.New("invalid frozen Bot participant receipt")

func deletionUUID(v string) bool {
	id, e := uuid.Parse(v)
	return e == nil && id != uuid.Nil && id.String() == v
}
func roleRemovalEvidence(request *rolev1.ApplyBotInstallationRemovalRequest) (proto.Message, string, error) {
	if request == nil || len(request.ProtoReflect().GetUnknown()) != 0 {
		return nil, "", errDeletionEvidence
	}
	var evidence proto.Message
	var kind string
	switch e := request.Evidence.(type) {
	case *rolev1.ApplyBotInstallationRemovalRequest_Native:
		evidence, kind = e.Native, "native"
	case *rolev1.ApplyBotInstallationRemovalRequest_LegacyInstallation:
		evidence, kind = e.LegacyInstallation, "legacy_installation"
	case *rolev1.ApplyBotInstallationRemovalRequest_OrphanWhitelist:
		evidence, kind = e.OrphanWhitelist, "orphan_whitelist"
	default:
		return nil, "", errDeletionEvidence
	}
	if evidence == nil || !evidence.ProtoReflect().IsValid() || len(evidence.ProtoReflect().GetUnknown()) != 0 {
		return nil, "", errDeletionEvidence
	}
	return evidence, kind, nil
}

func validateRoleDeletionReceipt(request *rolev1.ApplyBotInstallationRemovalRequest, response *rolev1.ApplyBotInstallationRemovalResponse) error {
	evidence, kind, err := roleRemovalEvidence(request)
	if err != nil || response == nil || len(response.ProtoReflect().GetUnknown()) != 0 || response.Receipt == nil {
		return errDeletionEvidence
	}
	r := response.Receipt
	if len(r.ProtoReflect().GetUnknown()) != 0 || r.ProtocolVersion != 1 || !deletionUUID(r.ReceiptId) || r.OperationId != request.OperationId || r.DecisionId != request.DecisionId || r.Purpose != request.Purpose || r.BotId != request.BotId || r.ActorProfileId != request.ActorProfileId || r.SpaceId != request.SpaceId || r.EvidenceKind != kind || r.Participant != "role" || r.Outcome != "COMPLETED" || !bytes.Equal(r.FrozenScopeSha256, request.FrozenScopeSha256) || r.CompletedAt == nil || r.CompletedAt.CheckValid() != nil || r.CompletedAt.Seconds <= 0 || len(r.CompletedAt.ProtoReflect().GetUnknown()) != 0 || r.DeletedRoleCount > math.MaxInt64 || r.RemovedAssignmentCount > math.MaxInt64 {
		return errDeletionEvidence
	}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return errDeletionEvidence
	}
	requestSHA := sha256.Sum256(body)
	body, err = (proto.MarshalOptions{Deterministic: true}).Marshal(evidence)
	if err != nil {
		return errDeletionEvidence
	}
	evidenceSHA := sha256.Sum256(body)
	if !bytes.Equal(requestSHA[:], r.RequestSha256) || !bytes.Equal(evidenceSHA[:], r.EvidenceSha256) {
		return errDeletionEvidence
	}
	return nil
}

func messagingAttachmentDigest(request *messagingv1.ApplyBotChatPinRemovalRequest, attachment *messagingv1.BotChatPinRemovalAttachment) ([]byte, error) {
	projection := proto.Clone(request).(*messagingv1.ApplyBotChatPinRemovalRequest)
	projection.OperationId, projection.DecisionId, projection.Purpose = "", "", ""
	projection.DecisionSha256, projection.FrozenScopeSha256 = nil, nil
	one := proto.Clone(attachment).(*messagingv1.BotChatPinRemovalAttachment)
	one.CurrentScope = nil
	one.CoveringOperationId = ""
	projection.Attachments = []*messagingv1.BotChatPinRemovalAttachment{one}
	if e := projection.GetLegacyInstallation(); e != nil {
		e.SnapshotId = ""
		e.SnapshotSha256 = nil
	}
	if e := projection.GetOrphanWhitelist(); e != nil {
		e.SnapshotId = ""
		e.SnapshotSha256 = nil
	}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(projection)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(append([]byte("voice.bot-pin-attachment.v1\x00"), body...))
	return digest[:], nil
}

func validateMessagingDeletionReceipt(request *messagingv1.ApplyBotChatPinRemovalRequest, response *messagingv1.ApplyBotChatPinRemovalResponse) error {
	if request == nil || response == nil || len(response.ProtoReflect().GetUnknown()) != 0 || response.Receipt == nil {
		return errDeletionEvidence
	}
	r := response.Receipt
	if len(r.ProtoReflect().GetUnknown()) != 0 || r.ProtocolVersion != 1 || !deletionUUID(r.ReceiptId) || r.OperationId != request.OperationId || r.DecisionId != request.DecisionId || r.Purpose != request.Purpose || r.BotId != request.BotId || r.ActorProfileId != request.ActorProfileId || r.SpaceId != request.SpaceId || r.ChatId != request.ChatId || r.Participant != "messaging" || r.Outcome != "COMPLETED" || !bytes.Equal(r.FrozenScopeSha256, request.FrozenScopeSha256) || r.RemovedPinCount > math.MaxInt64 || r.ExecutionCleanupRevision > math.MaxInt64 || r.CompletedAt == nil || r.CompletedAt.CheckValid() != nil || r.CompletedAt.Seconds <= 0 || len(r.CompletedAt.ProtoReflect().GetUnknown()) != 0 || len(request.Attachments) == 0 || len(r.Targets) != len(request.Attachments) {
		return errDeletionEvidence
	}
	var evidence proto.Message
	var kind string
	switch {
	case request.GetNative() != nil:
		evidence, kind = request.GetNative(), "native"
	case request.GetLegacyInstallation() != nil:
		evidence, kind = request.GetLegacyInstallation(), "legacy_installation"
	case request.GetOrphanWhitelist() != nil:
		evidence, kind = request.GetOrphanWhitelist(), "orphan_whitelist"
	default:
		return errDeletionEvidence
	}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return errDeletionEvidence
	}
	digest := sha256.Sum256(body)
	ebody, err := (proto.MarshalOptions{Deterministic: true}).Marshal(evidence)
	if err != nil {
		return errDeletionEvidence
	}
	edigest := sha256.Sum256(ebody)
	if r.EvidenceKind != kind || !bytes.Equal(r.EvidenceSha256, edigest[:]) || !bytes.Equal(r.RequestSha256, digest[:]) {
		return errDeletionEvidence
	}
	first := 0
	for i, target := range r.Targets {
		if target == nil || len(target.ProtoReflect().GetUnknown()) != 0 || target.AttachmentId != request.Attachments[i].AttachmentId || target.CleanupRevision == 0 || target.CleanupRevision > math.MaxInt64 {
			return errDeletionEvidence
		}
		want, err := messagingAttachmentDigest(request, request.Attachments[i])
		if err != nil || !bytes.Equal(want, target.EffectTargetSha256) {
			return errDeletionEvidence
		}
		if target.CoveringOperationId == "" {
			if target.CoveringReceiptId != "" || len(target.CoveringResponseSha256) != 0 || r.ExecutionCleanupRevision == 0 || target.CleanupRevision != r.ExecutionCleanupRevision {
				return errDeletionEvidence
			}
			first++
		} else if !deletionUUID(target.CoveringOperationId) || !deletionUUID(target.CoveringReceiptId) || target.CoveringOperationId == request.OperationId || len(target.CoveringResponseSha256) != 32 {
			return errDeletionEvidence
		}
	}
	if (first == 0) != (r.ExecutionCleanupRevision == 0) || (first == 0 && r.RemovedPinCount != 0) {
		return errDeletionEvidence
	}
	return nil
}
