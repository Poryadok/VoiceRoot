package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"math"
	"sort"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

func frozenMessagingObservation(saved store.ChatScopeCleanupObservation, original bool) (*messagingv1.BotRemovalChatScopeObservation, error) {
	request := new(chatv1.GetBotChatScopeAuthorityRequest)
	response := new(chatv1.GetBotChatScopeAuthorityResponse)
	if proto.Unmarshal(saved.Request, request) != nil || proto.Unmarshal(saved.Response, response) != nil || request.OperationId != saved.OperationID.String() {
		return nil, errDeletionEvidence
	}
	requestSHA, responseSHA := sha256.Sum256(saved.Request), sha256.Sum256(saved.Response)
	if !bytes.Equal(requestSHA[:], saved.RequestSHA) || !bytes.Equal(responseSHA[:], saved.ResponseSHA) {
		return nil, errDeletionEvidence
	}
	q, qerr := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	r, rerr := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	if qerr != nil || rerr != nil || !bytes.Equal(q, saved.Request) || !bytes.Equal(r, saved.Response) {
		return nil, errDeletionEvidence
	}
	if _, err := store.NewChatScopeObservation(request, response, original); err != nil {
		return nil, errDeletionEvidence
	}
	return &messagingv1.BotRemovalChatScopeObservation{Request: request, Response: response}, nil
}

func buildMessagingDeletionRequest(op *store.BotDeletion, effect store.BotDeletionEffect) (*messagingv1.ApplyBotChatPinRemovalRequest, error) {
	if effect.Kind != "MESSAGING_UNPIN" || effect.ChatID == nil || *effect.ChatID == [16]byte{} {
		return nil, errDeletionEvidence
	}
	// Reuse the exact root/evidence checks, never a mutable installation lookup.
	roleEffect := effect
	roleEffect.Kind, roleEffect.ChatID = "ROLE_REMOVE", nil
	role, err := buildRoleDeletionRequest(op, roleEffect)
	if err != nil {
		return nil, err
	}
	request := &messagingv1.ApplyBotChatPinRemovalRequest{ProtocolVersion: 1, OperationId: role.OperationId, DecisionId: role.DecisionId, DecisionSha256: role.DecisionSha256, Purpose: role.Purpose, BotId: role.BotId, ActorProfileId: role.ActorProfileId, SpaceId: role.SpaceId, ChatId: effect.ChatID.String(), FrozenScopeSha256: role.FrozenScopeSha256}
	switch {
	case role.GetNative() != nil:
		e := role.GetNative()
		request.Evidence = &messagingv1.ApplyBotChatPinRemovalRequest_Native{Native: &messagingv1.BotRemovalNativeEvidence{InstallationId: e.InstallationId, InstallOperationId: e.InstallOperationId, Generation: e.Generation}}
	case role.GetLegacyInstallation() != nil:
		e := role.GetLegacyInstallation()
		request.Evidence = &messagingv1.ApplyBotChatPinRemovalRequest_LegacyInstallation{LegacyInstallation: &messagingv1.BotRemovalLegacyInstallationEvidence{InstallationId: e.InstallationId, SnapshotId: e.SnapshotId, SnapshotSha256: e.SnapshotSha256}}
	case role.GetOrphanWhitelist() != nil:
		e := role.GetOrphanWhitelist()
		request.Evidence = &messagingv1.ApplyBotChatPinRemovalRequest_OrphanWhitelist{OrphanWhitelist: &messagingv1.BotRemovalOrphanWhitelistEvidence{SnapshotId: e.SnapshotId, SnapshotSha256: e.SnapshotSha256}}
	default:
		return nil, errDeletionEvidence
	}
	seen := map[string]bool{}
	for _, source := range effect.Target.ChatSources {
		if source.ChatID != *effect.ChatID {
			continue
		}
		if source.AttachmentID == [16]byte{} || source.InstallationID != effect.Target.InstallationID || seen[source.AttachmentID.String()] || (source.IntentOperationID != nil && *source.IntentOperationID != effect.Target.InstallOperationID) {
			return nil, errDeletionEvidence
		}
		seen[source.AttachmentID.String()] = true
		a := &messagingv1.BotChatPinRemovalAttachment{AttachmentId: source.AttachmentID.String()}
		if source.OriginalUnqualified {
			if len(source.Original.Request) != 0 || len(source.Original.Response) != 0 || source.Original.OperationID != [16]byte{} || len(source.Original.RequestSHA) != 0 || len(source.Original.ResponseSHA) != 0 {
				return nil, errDeletionEvidence
			}
			a.OriginalScopeEvidence = &messagingv1.BotChatPinRemovalAttachment_OriginalScopeUnqualified{OriginalScopeUnqualified: true}
		} else {
			observation, err := frozenMessagingObservation(source.Original, true)
			if err != nil {
				return nil, err
			}
			a.OriginalScopeEvidence = &messagingv1.BotChatPinRemovalAttachment_OriginalScope{OriginalScope: observation}
		}
		if source.Current != nil {
			a.CurrentScope, err = frozenMessagingObservation(*source.Current, false)
			if err != nil {
				return nil, err
			}
		}
		for _, observation := range []*messagingv1.BotRemovalChatScopeObservation{a.GetOriginalScope(), a.CurrentScope} {
			if observation != nil && (observation.Request.ChatId != request.ChatId || observation.Request.ExpectedSpaceId != request.SpaceId) {
				return nil, errDeletionEvidence
			}
		}
		if a.GetOriginalScope() != nil && a.CurrentScope != nil && a.GetOriginalScope().Response.Scope.ChatScopeIncarnation != a.CurrentScope.Response.Scope.ChatScopeIncarnation {
			return nil, errDeletionEvidence
		}
		request.Attachments = append(request.Attachments, a)
	}
	if len(request.Attachments) == 0 {
		return nil, errDeletionEvidence
	}
	sort.Slice(request.Attachments, func(i, j int) bool { return request.Attachments[i].AttachmentId < request.Attachments[j].AttachmentId })
	return request, nil
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

func (s *BotGRPC) applyMessagingDeletion(ctx context.Context, claim *store.BotDeletion, effect store.BotDeletionEffect) error {
	if s == nil || s.Store == nil || s.CleanupMessaging == nil || s.PrincipalIssuer == nil {
		return status.Error(codes.Unavailable, "protected Messaging cleanup authority unavailable")
	}
	request, err := buildMessagingDeletionRequest(claim, effect)
	if err != nil {
		return status.Error(codes.FailedPrecondition, "frozen Bot chat cleanup evidence is not qualified")
	}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return err
	}
	if len(effect.RequestBytes) != 0 && !bytes.Equal(effect.RequestBytes, body) {
		return store.ErrInstallationConflict
	}
	if err = s.Store.SealBotDeletionRequest(ctx, claim, effect.EffectID, body); err != nil {
		return err
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return err
	}
	token, err := s.PrincipalIssuer.IssueService(principal.ServiceInput{Audience: "messaging", RPC: messagingv1.MessagingService_ApplyBotChatPinRemoval_FullMethodName, RequestID: request.OperationId, RequestHash: hash})
	if err != nil {
		return status.Error(codes.Unavailable, "Bot cleanup principal unavailable")
	}
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	callCtx = metadata.NewOutgoingContext(callCtx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", request.OperationId))
	response, err := s.CleanupMessaging.ApplyBotChatPinRemoval(callCtx, request)
	if err != nil {
		return err
	}
	if validateMessagingDeletionReceipt(request, response) != nil {
		return status.Error(codes.Unavailable, "Messaging cleanup acknowledgement is not qualified")
	}
	receipt, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	if err != nil {
		return err
	}
	return s.Store.SaveBotDeletionReceipt(ctx, claim, effect.EffectID, body, receipt)
}
