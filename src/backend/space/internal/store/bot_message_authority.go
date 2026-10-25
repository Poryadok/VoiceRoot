package store

import (
	"context"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"math"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
)

func validateBotMessageSelector(req *spacev1.GetMemberAdmissionAuthorityRequest) error {
	b := req.GetBotMessageSendBinding()
	if b == nil {
		return nil
	}
	if b.ProtocolVersion != 1 || len(b.ProtoReflect().GetUnknown()) != 0 || b.InstallationGeneration == 0 || b.InstallationGeneration > math.MaxInt64 || req.ExpectedGeneration != nil || req.ExpectedAdmissionOperationId != nil || req.ChatId == nil || b.SpaceId != req.SpaceId || b.ActorProfileId != req.ProfileId || b.ChatId != req.GetChatId() {
		return ErrMemberAuthorityUnavailable
	}
	for _, id := range []string{b.BotId, b.ActorProfileId, b.SpaceId, b.InstallationId, b.InstallOperationId, b.ChatId} {
		if _, err := canonicalLifecycleUUID(id); err != nil {
			return ErrMemberAuthorityUnavailable
		}
	}
	return nil
}

// The ordinary reader has already authenticated the native publication bytes,
// digest, Bot producer witness and current membership. This final selector
// check binds that proof to Messaging's requested installation incarnation.
func qualifyBotMessageAuthority(ctx context.Context, tx pgx.Tx, b *commonv1.BotMessageSendBinding, r *spacev1.GetMemberAdmissionAuthorityResponse) error {
	if r.GetLifecycle().GetState() != spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE || r.GetOwnership().GetState() != spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE || r.GetOrdinary().GetState() != spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE || r.GetOrdinary().GetExplicitLegacy() || int32(r.GetOrdinary().GetPublicationKind()) != 6 || r.GetOrdinary().GetNativePublication().GetOperationId() != b.InstallOperationId || !r.GetTreeRelation().GetPresent() || r.GetTreeRelation().GetChatId() != b.ChatId {
		return ErrMemberAuthorityUnavailable
	}
	var current bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_bot_installation_heads h
 WHERE h.space_id=$1 AND h.bot_id=$2 AND h.profile_id=$3 AND h.installation_id=$4
 AND h.install_operation_id=$5 AND h.generation=$6 AND h.membership_generation=$7
 AND h.state='INSTALLED' AND h.retired_through<h.generation)
 AND EXISTS(SELECT 1 FROM space_tree_nodes n LEFT JOIN categories c ON c.id=n.category_id
 WHERE n.space_id=$1 AND n.chat_id=$8 AND n.kind='text_chat'
 AND (n.category_id IS NULL OR c.space_id=n.space_id))`, b.SpaceId, b.BotId, b.ActorProfileId, b.InstallationId, b.InstallOperationId, b.InstallationGeneration, r.Ordinary.HeadGeneration, b.ChatId).Scan(&current)
	if err != nil || !current {
		return ErrMemberAuthorityUnavailable
	}
	r.BotMessageSendBinding = proto.Clone(b).(*commonv1.BotMessageSendBinding)
	return nil
}
