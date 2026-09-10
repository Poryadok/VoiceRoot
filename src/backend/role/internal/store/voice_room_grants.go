package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"voice/backend/role/permissions"
)

// VoiceRoomGrantDecision is Role's authoritative, epoch-bound decision for a
// Voice participant. CanSubscribe intentionally shares VOICE_JOIN with CanJoin.
type VoiceRoomGrantDecision struct {
	CanJoin               bool
	CanPublishAudio       bool
	CanPublishVideo       bool
	CanPublishScreenShare bool
	CanSubscribe          bool
	CanMuteOthers         bool
	CanDeafenOthers       bool
	CanMoveOthers         bool
	CanUsePTT             bool
	PrioritySpeaker       bool
	PolicyEpoch           uint64
}

// ResolveVoiceRoomGrants reads the effective Voice permission mask and its
// authority epoch in the same fenced, per-Space transaction.
func (s *RoleStore) ResolveVoiceRoomGrants(ctx context.Context, spaceID, voiceRoomID, profileID uuid.UUID) (VoiceRoomGrantDecision, error) {
	return spaceValue(ctx, s, spaceID, func(scoped *RoleStore) (VoiceRoomGrantDecision, error) {
		mask, err := scoped.unscopedGetEffectiveMask(ctx, spaceID, profileID, nil, &voiceRoomID)
		if err != nil {
			return VoiceRoomGrantDecision{}, err
		}
		var epoch int64
		if err := scoped.db().QueryRow(ctx, `
SELECT policy_epoch
FROM role_voice_policy_epochs
WHERE space_id = $1
`, spaceID).Scan(&epoch); err != nil {
			return VoiceRoomGrantDecision{}, fmt.Errorf("resolve Voice policy epoch: %w", err)
		}
		if epoch <= 0 {
			return VoiceRoomGrantDecision{}, fmt.Errorf("resolve Voice policy epoch: non-positive epoch %d", epoch)
		}

		join := hasVoicePermission(mask, permissions.VoiceJoin)
		return VoiceRoomGrantDecision{
			CanJoin:               join,
			CanPublishAudio:       hasVoicePermission(mask, permissions.VoiceSpeak),
			CanPublishVideo:       hasVoicePermission(mask, permissions.VoiceVideo),
			CanPublishScreenShare: hasVoicePermission(mask, permissions.VoiceScreenShare),
			CanSubscribe:          join,
			CanMuteOthers:         hasVoicePermission(mask, permissions.VoiceMuteOthers),
			CanDeafenOthers:       hasVoicePermission(mask, permissions.VoiceDeafenOthers),
			CanMoveOthers:         hasVoicePermission(mask, permissions.VoiceMoveOthers),
			CanUsePTT:             hasVoicePermission(mask, permissions.VoiceUsePTT),
			PrioritySpeaker:       hasVoicePermission(mask, permissions.VoicePrioritySpeaker),
			PolicyEpoch:           uint64(epoch),
		}, nil
	})
}

func hasVoicePermission(mask uint64, permission string) bool {
	bit, err := permissions.MaskFor(permission)
	return err == nil && mask&bit != 0
}
