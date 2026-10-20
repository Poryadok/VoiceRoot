package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"math"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

// BotPurgeCoverageProof is bounded pending-work evidence, not a new remote
// cleanup receipt. The immutable local target must independently supply chats.
// Native drain and global terminal obligations are deliberately separate.
type BotPurgeCoverageProof struct {
	Purge           *commonv1.SpacePurgeRequest
	HandoffRequest  *spacev1.GetBotPurgeHandoffRequest
	Handoff         *spacev1.GetBotPurgeHandoffResponse
	CoverageRequest *messagingv1.GetBotPurgeChatCoverageRequest
	Coverage        *messagingv1.GetBotPurgeChatCoverageResponse
}

// ValidateBotPurgeCoverage returns a deep owned snapshot only after BOTH exact
// originals and the requested-chat membership have been checked. It performs
// no journal transition and cannot authorize deletion of an installation.
func ValidateBotPurgeCoverage(input BotPurgeCoverageProof, chats []string) (*BotPurgeCoverageProof, error) {
	if input.Purge == nil || input.HandoffRequest == nil || input.Handoff == nil {
		return nil, ErrDeletionPending
	}
	// Own every protobuf before processing it; retained proof never aliases an RPC
	// response or a caller's request through a later database wait.
	p := BotPurgeCoverageProof{Purge: proto.Clone(input.Purge).(*commonv1.SpacePurgeRequest), HandoffRequest: proto.Clone(input.HandoffRequest).(*spacev1.GetBotPurgeHandoffRequest), Handoff: proto.Clone(input.Handoff).(*spacev1.GetBotPurgeHandoffResponse)}
	q, r, h := p.Purge, p.HandoffRequest, p.Handoff
	if !purgeProofKnown(q) || !purgeProofKnown(r) || !purgeProofKnown(h) || q.ProtocolVersion != 1 || q.ParticipantId != commonv1.ParticipantId_PARTICIPANT_ID_BOT || q.Generation == 0 || q.Generation > math.MaxInt64 || q.PurgeDecidedAt == nil || q.PurgeDecidedAt.CheckValid() != nil || q.PurgeDecidedAt.Seconds <= 0 || q.Manifest == nil || len(q.Manifest.ManifestSha256) != sha256.Size || q.Manifest.ItemCount > math.MaxInt64 {
		return nil, ErrSpaceLifecycleConflict
	}
	for _, id := range []string{q.SpaceId, q.DeletionOperationId, q.Manifest.ManifestId, r.RequestId} {
		if _, err := lifecycleID(id); err != nil {
			return nil, ErrSpaceLifecycleConflict
		}
	}
	if r.ProtocolVersion != 1 || r.SpaceId != q.SpaceId || r.DeletionOperationId != q.DeletionOperationId || r.Generation != q.Generation || !proto.Equal(r.Manifest, q.Manifest) || h.ProtocolVersion != r.ProtocolVersion || h.RequestId != r.RequestId || h.SpaceId != r.SpaceId || h.DeletionOperationId != r.DeletionOperationId || h.Generation != r.Generation || !proto.Equal(h.Manifest, r.Manifest) {
		return nil, ErrSpaceLifecycleConflict
	}
	if h.Role == nil {
		return nil, ErrDeletionPending
	}
	role := new(rolev1.RetireSpaceRequest)
	if !purgeProofCanonical(h.Role.OriginalRequestBytes, role) || !purgeProofSealed("voice.role.v1.RetireSpaceRequest", h.Role.OriginalRequestBytes, h.Role.OriginalRequestSha256) || !purgeProofReceipt(h.Role.OriginalReceipt, h.Role.OriginalReceiptBytes, "voice.role.v1.RetireSpaceReceipt", h.Role.OriginalReceiptSha256) {
		return nil, ErrSpaceLifecycleConflict
	}
	rr := h.Role.OriginalReceipt
	if role.ProtocolVersion != 1 || role.SpaceId != q.SpaceId || role.DeletionOperationId != q.DeletionOperationId || role.Generation != q.Generation || !proto.Equal(role.Manifest, q.Manifest) || !proto.Equal(role.PurgeDecidedAt, q.PurgeDecidedAt) || rr.ProtocolVersion != 1 || rr.SpaceId != q.SpaceId || rr.DeletionOperationId != q.DeletionOperationId || rr.Generation != q.Generation || rr.State != rolev1.RoleRetirementState_ROLE_RETIREMENT_STATE_RETIRED || !bytes.Equal(rr.RequestSha256, h.Role.OriginalRequestSha256) || !bytes.Equal(rr.ManifestSha256, q.Manifest.ManifestSha256) || rr.RetiredAt == nil || rr.RetiredAt.CheckValid() != nil || rr.RetiredAt.Seconds <= 0 {
		return nil, ErrSpaceLifecycleConflict
	}
	if _, err := lifecycleID(rr.ReceiptId); err != nil {
		return nil, ErrSpaceLifecycleConflict
	}
	// Validate a supplied Messaging original even when this target has no chats;
	// malformed optional entries are never silently ignored.
	if h.Messaging != nil {
		wrapper := new(messagingv1.PurgeSpaceRequest)
		if !purgeProofCanonical(h.Messaging.OriginalRequestBytes, wrapper) || !purgeProofSealed("voice.messaging.v1.PurgeSpaceRequest", h.Messaging.OriginalRequestBytes, h.Messaging.OriginalRequestSha256) || !purgeProofReceipt(h.Messaging.OriginalReceipt, h.Messaging.OriginalReceiptBytes, "voice.common.v1.SpacePurgeReceipt", h.Messaging.OriginalReceiptSha256) {
			return nil, ErrSpaceLifecycleConflict
		}
		original := wrapper.Purge
		receipt := h.Messaging.OriginalReceipt
		if original == nil || original.ProtocolVersion != 1 || original.ParticipantId != commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING || original.SpaceId != q.SpaceId || original.DeletionOperationId != q.DeletionOperationId || original.Generation != q.Generation || !proto.Equal(original.Manifest, q.Manifest) || !proto.Equal(original.PurgeDecidedAt, q.PurgeDecidedAt) || receipt.ProtocolVersion != 1 || receipt.SpaceId != q.SpaceId || receipt.DeletionOperationId != q.DeletionOperationId || receipt.Generation != q.Generation || receipt.ParticipantId != commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING || receipt.State != commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED || !bytes.Equal(receipt.RequestSha256, h.Messaging.OriginalRequestSha256) || receipt.CompletedAt == nil || receipt.CompletedAt.CheckValid() != nil || receipt.CompletedAt.Seconds <= 0 {
			return nil, ErrSpaceLifecycleConflict
		}
		if _, err := lifecycleID(receipt.ReceiptId); err != nil {
			return nil, ErrSpaceLifecycleConflict
		}
	}
	if len(chats) > 1000 {
		return nil, ErrSpaceLifecycleConflict
	}
	for i, id := range chats {
		if _, err := lifecycleID(id); err != nil {
			return nil, ErrSpaceLifecycleConflict
		}
		if i > 0 && chats[i-1] >= id {
			return nil, ErrSpaceLifecycleConflict
		}
	}
	if len(chats) == 0 {
		if input.CoverageRequest != nil || input.Coverage != nil {
			return nil, ErrSpaceLifecycleConflict
		}
		return &p, nil
	}
	if h.Messaging == nil || input.CoverageRequest == nil || input.Coverage == nil {
		return nil, ErrDeletionPending
	}
	p.CoverageRequest = proto.Clone(input.CoverageRequest).(*messagingv1.GetBotPurgeChatCoverageRequest)
	p.Coverage = proto.Clone(input.Coverage).(*messagingv1.GetBotPurgeChatCoverageResponse)
	c, v := p.CoverageRequest, p.Coverage
	if !purgeProofKnown(c) || !purgeProofKnown(v) || c.ProtocolVersion != 1 || c.SpaceId != q.SpaceId || c.DeletionOperationId != q.DeletionOperationId || c.Generation != q.Generation || !proto.Equal(c.Manifest, q.Manifest) || len(c.ChatIds) != len(chats) {
		return nil, ErrSpaceLifecycleConflict
	}
	if _, err := lifecycleID(c.RequestId); err != nil {
		return nil, ErrSpaceLifecycleConflict
	}
	for i, id := range chats {
		if c.ChatIds[i] != id {
			return nil, ErrSpaceLifecycleConflict
		}
	}
	if v.ProtocolVersion != 1 || v.RequestId != c.RequestId || v.SpaceId != c.SpaceId || v.DeletionOperationId != c.DeletionOperationId || v.Generation != c.Generation || !proto.Equal(v.Manifest, c.Manifest) || v.SourceScheduleGeneration == 0 || v.SourceScheduleGeneration > math.MaxInt64 || len(v.Coverage) != len(chats) {
		return nil, ErrSpaceLifecycleConflict
	}
	hash, err := principal.RequestHash(c)
	if err != nil || hash != "sha256:"+hex.EncodeToString(v.RequestSha256) {
		return nil, ErrSpaceLifecycleConflict
	}
	for i, id := range chats {
		if v.Coverage[i] == nil || v.Coverage[i].ChatId != id {
			return nil, ErrSpaceLifecycleConflict
		}
		if !v.Coverage[i].Covered {
			return nil, ErrDeletionPending
		}
	}
	return &p, nil
}

func purgeProofSealed(domain string, raw, digest []byte) bool {
	sum := lifecycleHash(domain, raw)
	return len(raw) > 0 && bytes.Equal(sum[:], digest)
}
func purgeProofCanonical(raw []byte, m proto.Message) bool {
	if len(raw) == 0 || len(raw) > 1024*1024 || proto.Unmarshal(raw, m) != nil || !purgeProofKnown(m) {
		return false
	}
	canonical, err := (proto.MarshalOptions{Deterministic: true}).Marshal(m)
	return err == nil && bytes.Equal(canonical, raw)
}
func purgeProofReceipt(m proto.Message, raw []byte, domain string, digest []byte) bool {
	if m == nil || !m.ProtoReflect().IsValid() || !purgeProofKnown(m) {
		return false
	}
	canonical, err := (proto.MarshalOptions{Deterministic: true}).Marshal(m)
	return err == nil && bytes.Equal(canonical, raw) && purgeProofSealed(domain, raw, digest)
}
func purgeProofKnown(m proto.Message) bool {
	if m == nil || !m.ProtoReflect().IsValid() {
		return false
	}
	var walk func(protoreflect.Message) bool
	walk = func(x protoreflect.Message) bool {
		if len(x.GetUnknown()) != 0 {
			return false
		}
		ok := true
		x.Range(func(f protoreflect.FieldDescriptor, v protoreflect.Value) bool {
			if f.Kind() != protoreflect.MessageKind && f.Kind() != protoreflect.GroupKind {
				return true
			}
			if f.IsList() {
				list := v.List()
				for i := 0; i < list.Len(); i++ {
					if !walk(list.Get(i).Message()) {
						ok = false
						return false
					}
				}
			} else if !walk(v.Message()) {
				ok = false
				return false
			}
			return true
		})
		return ok
	}
	return walk(m.ProtoReflect())
}
