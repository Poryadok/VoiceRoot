package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

type botPinAttachmentBinding struct {
	id         uuid.UUID
	projection []byte
	digest     [32]byte
	attachment *messagingv1.BotChatPinRemovalAttachment
}

func botPinKnownFields(message proto.Message) bool {
	if message == nil {
		return false
	}
	m := message.ProtoReflect()
	if !m.IsValid() || len(m.GetUnknown()) != 0 {
		return false
	}
	valid := true
	m.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.Message() == nil {
			return true
		}
		if field.IsList() {
			list := value.List()
			for i := 0; i < list.Len(); i++ {
				if !botPinKnownFields(list.Get(i).Message().Interface()) {
					valid = false
					return false
				}
			}
		} else if !field.IsMap() && !botPinKnownFields(value.Message().Interface()) {
			valid = false
			return false
		}
		return true
	})
	return valid
}

func validateBotPinScope(observation *messagingv1.BotRemovalChatScopeObservation, chat, space string, original bool) error {
	if !botPinKnownFields(observation) || observation.Request == nil || observation.Response == nil || observation.Response.Scope == nil {
		return ErrBotPinRemovalBinding
	}
	q, response := observation.Request, observation.Response
	scope := response.Scope
	if q.ProtocolVersion != 1 || q.ChatId != chat || q.ExpectedSpaceId != space {
		return ErrBotPinRemovalBinding
	}
	if _, err := botPinUUID(q.OperationId); err != nil {
		return err
	}
	if scope.ProtocolVersion != 1 || scope.ChatId != chat || scope.ExpectedSpaceId != space || scope.ActualSpaceId != space || scope.OperationId != q.OperationId || scope.ChatAuthorityRevision == 0 || scope.ChatAuthorityRevision > math.MaxInt64 || scope.LifecycleGeneration > math.MaxInt64 {
		return ErrBotPinRemovalBinding
	}
	if _, err := botPinUUID(scope.ChatScopeIncarnation); err != nil {
		return err
	}
	requestBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	if err != nil {
		return ErrBotPinRemovalBinding
	}
	digest := sha256.Sum256(requestBytes)
	if !bytes.Equal(digest[:], response.RequestSha256) {
		return ErrBotPinRemovalBinding
	}
	if scope.LifecycleState < chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE || scope.LifecycleState > chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_PURGED {
		return ErrBotPinRemovalBinding
	}
	if scope.Live != (scope.Exists && scope.LifecycleState == chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE) {
		return ErrBotPinRemovalBinding
	}
	if scope.Exists {
		if (scope.ChatType != chatv1.ChatType_CHAT_TYPE_GROUP && scope.ChatType != chatv1.ChatType_CHAT_TYPE_CHANNEL) || scope.ManagedKind != chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_ORDINARY {
			return ErrBotPinRemovalBinding
		}
	} else if scope.ChatType != chatv1.ChatType_CHAT_TYPE_UNSPECIFIED || scope.ManagedKind != chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_UNSPECIFIED {
		return ErrBotPinRemovalBinding
	}
	if original && (!scope.Exists || !scope.Live) {
		return ErrBotPinRemovalBinding
	}
	return nil
}

func bindBotPinAttachments(request *messagingv1.ApplyBotChatPinRemovalRequest) ([]botPinAttachmentBinding, error) {
	if len(request.Attachments) == 0 {
		return nil, ErrBotPinRemovalBinding
	}
	result := make([]botPinAttachmentBinding, 0, len(request.Attachments))
	previous := ""
	for _, attachment := range request.Attachments {
		if !botPinKnownFields(attachment) {
			return nil, ErrBotPinRemovalBinding
		}
		id, err := botPinUUID(attachment.AttachmentId)
		if err != nil || attachment.AttachmentId <= previous {
			return nil, ErrBotPinRemovalBinding
		}
		previous = attachment.AttachmentId
		if attachment.CoveringOperationId != "" {
			if _, err = botPinUUID(attachment.CoveringOperationId); err != nil {
				return nil, err
			}
		}
		switch origin := attachment.OriginalScopeEvidence.(type) {
		case *messagingv1.BotChatPinRemovalAttachment_OriginalScope:
			if err = validateBotPinScope(origin.OriginalScope, request.ChatId, request.SpaceId, true); err != nil {
				return nil, err
			}
		case *messagingv1.BotChatPinRemovalAttachment_OriginalScopeUnqualified:
			if !origin.OriginalScopeUnqualified {
				return nil, ErrBotPinRemovalBinding
			}
		default:
			return nil, ErrBotPinRemovalBinding
		}
		if attachment.CurrentScope != nil {
			if err = validateBotPinScope(attachment.CurrentScope, request.ChatId, request.SpaceId, false); err != nil {
				return nil, err
			}
			if original := attachment.GetOriginalScope(); original != nil && original.Response.Scope.ChatScopeIncarnation != attachment.CurrentScope.Response.Scope.ChatScopeIncarnation {
				return nil, ErrBotPinRemovalBinding
			}
		}
		projection := proto.Clone(request).(*messagingv1.ApplyBotChatPinRemovalRequest)
		projection.OperationId = ""
		projection.DecisionId = ""
		projection.DecisionSha256 = nil
		projection.Purpose = ""
		projection.FrozenScopeSha256 = nil
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
		raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(projection)
		if err != nil {
			return nil, ErrBotPinRemovalBinding
		}
		digest := sha256.Sum256(append([]byte("voice.bot-pin-attachment.v1\x00"), raw...))
		result = append(result, botPinAttachmentBinding{id: id, projection: raw, digest: digest, attachment: attachment})
	}
	return result, nil
}

// BotPinChatScopeLoader reads independent current Chat-owned facts. An accepted
// Bot observation remains point-in-time input, not a lease on this datasource.
type BotPinChatScopeLoader interface {
	LoadBotPinChatScope(context.Context, uuid.UUID, uuid.UUID) (*chatv1.GetSpaceChatScopeAuthorityResponse, error)
}

func (s *PinsStore) qualifyUncoveredBotPinAttachment(ctx context.Context, b *botPinRequestBinding, target botPinAttachmentBinding) error {
	if s.BotPinChatScope == nil {
		return ErrStoreNotConfigured
	}
	want := target.attachment.CurrentScope
	if want == nil {
		want = target.attachment.GetOriginalScope()
	}
	if want == nil {
		return ErrBotPinRemovalBinding
	}
	actual, err := s.BotPinChatScope.LoadBotPinChatScope(ctx, b.chat, b.space)
	if err != nil {
		return err
	}
	saved := want.Response.Scope
	if actual == nil || actual.ProtocolVersion != 1 || actual.ChatId != b.chat.String() || actual.ActualSpaceId != b.space.String() || actual.ExpectedSpaceId != b.space.String() || actual.ChatScopeIncarnation != saved.ChatScopeIncarnation || actual.ChatAuthorityRevision < saved.ChatAuthorityRevision || actual.ChatAuthorityRevision > math.MaxInt64 || actual.LifecycleState != chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE {
		return ErrBotPinRemovalConflict
	}
	if actual.Exists {
		if !actual.Live || actual.ChatType != saved.ChatType || actual.ManagedKind != chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_ORDINARY || !saved.Exists {
			return ErrBotPinRemovalConflict
		}
	} else if actual.Live || actual.ChatType != chatv1.ChatType_CHAT_TYPE_UNSPECIFIED || actual.ManagedKind != chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_UNSPECIFIED {
		return ErrBotPinRemovalConflict
	}
	return nil
}

func readBotPinAttachmentCoverage(ctx context.Context, tx pgx.Tx, b *botPinRequestBinding, target botPinAttachmentBinding) (*messagingv1.BotChatPinRemovalTargetReceipt, error) {
	var projection, digest, requestBytes []byte
	var operation uuid.UUID
	var revision int64
	err := tx.QueryRow(ctx, `SELECT c.projection_bytes,c.effect_target_sha256,c.origin_operation_id,c.cleanup_revision,r.request_bytes FROM bot_pin_removal_attachment_coverage c JOIN bot_pin_removal_receipts r ON r.operation_id=c.origin_operation_id WHERE c.attachment_id=$1`, target.id).Scan(&projection, &digest, &operation, &revision, &requestBytes)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(projection, target.projection) || !bytes.Equal(digest, target.digest[:]) || revision <= 0 {
		return nil, ErrBotPinRemovalConflict
	}
	if hint := target.attachment.CoveringOperationId; hint != "" && hint != operation.String() {
		return nil, ErrBotPinRemovalConflict
	}
	var original messagingv1.ApplyBotChatPinRemovalRequest
	if proto.Unmarshal(requestBytes, &original) != nil {
		return nil, ErrBotPinRemovalBinding
	}
	bound, err := bindBotPinRemovalRequest(&original)
	if err != nil {
		return nil, err
	}
	response, err := readBotPinRemovalReceipt(ctx, tx, bound)
	if err != nil {
		return nil, err
	}
	var found bool
	for _, covered := range response.Receipt.Targets {
		if covered.AttachmentId != target.id.String() {
			continue
		}
		if found || !bytes.Equal(covered.EffectTargetSha256, target.digest[:]) || covered.CleanupRevision != uint64(revision) || covered.CoveringOperationId != "" || covered.CoveringReceiptId != "" || len(covered.CoveringResponseSha256) != 0 || response.Receipt.ExecutionCleanupRevision != uint64(revision) {
			return nil, ErrBotPinRemovalBinding
		}
		found = true
	}
	if !found {
		return nil, ErrBotPinRemovalBinding
	}
	// Projection identity deliberately excludes point-in-time current reads.
	// An explicit read must still name the first effect's actual incarnation;
	// a different incarnation is a conflict, never an uncovered target fallback.
	var origin *messagingv1.BotRemovalChatScopeObservation
	for _, saved := range bound.attachments {
		if saved.id == target.id {
			origin = saved.attachment.CurrentScope
			if origin == nil {
				origin = saved.attachment.GetOriginalScope()
			}
			break
		}
	}
	if origin == nil || origin.Response == nil || origin.Response.Scope == nil {
		return nil, ErrBotPinRemovalBinding
	}
	if current := target.attachment.CurrentScope; current != nil && current.Response.Scope.ChatScopeIncarnation != origin.Response.Scope.ChatScopeIncarnation {
		return nil, ErrBotPinRemovalConflict
	}
	raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	if err != nil {
		return nil, err
	}
	responseSHA := sha256.Sum256(raw)
	return &messagingv1.BotChatPinRemovalTargetReceipt{AttachmentId: target.id.String(), EffectTargetSha256: target.digest[:], CleanupRevision: uint64(revision), CoveringOperationId: operation.String(), CoveringReceiptId: response.Receipt.ReceiptId, CoveringResponseSha256: responseSHA[:]}, nil
}

func missingBotPinCoverage(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

func validateBotPinTargetReceipts(b *botPinRequestBinding, response *messagingv1.ApplyBotChatPinRemovalResponse) error {
	r := response.Receipt
	if len(b.attachments) == 0 {
		if len(r.Targets) != 0 || r.ExecutionCleanupRevision != 0 {
			return ErrBotPinRemovalBinding
		}
		return nil // exact old request only; never a source for cross-op coverage
	}
	if !botPinKnownFields(response) || len(r.Targets) != len(b.attachments) || r.ExecutionCleanupRevision > math.MaxInt64 {
		return ErrBotPinRemovalBinding
	}
	firstEffects := 0
	for index, target := range r.Targets {
		want := b.attachments[index]
		if target.AttachmentId != want.id.String() || !bytes.Equal(target.EffectTargetSha256, want.digest[:]) || target.CleanupRevision == 0 || target.CleanupRevision > math.MaxInt64 {
			return ErrBotPinRemovalBinding
		}
		if target.CoveringOperationId == "" {
			if target.CoveringReceiptId != "" || len(target.CoveringResponseSha256) != 0 || r.ExecutionCleanupRevision == 0 || target.CleanupRevision != r.ExecutionCleanupRevision {
				return ErrBotPinRemovalBinding
			}
			firstEffects++
		} else {
			if _, err := botPinUUID(target.CoveringOperationId); err != nil {
				return err
			}
			if _, err := botPinUUID(target.CoveringReceiptId); err != nil {
				return err
			}
			if target.CoveringOperationId == r.OperationId || len(target.CoveringResponseSha256) != 32 {
				return ErrBotPinRemovalBinding
			}
		}
	}
	if (firstEffects == 0) != (r.ExecutionCleanupRevision == 0) || (firstEffects == 0 && r.RemovedPinCount != 0) {
		return ErrBotPinRemovalBinding
	}
	return nil
}
