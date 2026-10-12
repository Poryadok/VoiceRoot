package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/messagingprincipal"
	"voice/backend/pkg/privacy"
	"voice/backend/user/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	userv1 "voice.app/voice/user/v1"
)

func (s *UserGRPC) scheduledMessageDispatchPresence(ctx context.Context, req *userv1.GetScheduledMessageDispatchPresenceRequest) (*userv1.GetScheduledMessageDispatchPresenceResponse, error) {
	request, err := messagingprincipal.ValidateScheduledPresenceRequest(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid scheduled presence request")
	}
	if s == nil || s.Profiles == nil {
		return nil, status.Error(codes.Unavailable, "scheduled presence dependencies unavailable")
	}
	isDM := request.ChatType == chatv1.ChatType_CHAT_TYPE_DM
	if isDM && s.Blocks == nil {
		return nil, status.Error(codes.Unavailable, "scheduled block authority unavailable")
	}
	if request.Mode == userv1.ScheduledMessageDispatchMode_SCHEDULED_MESSAGE_DISPATCH_MODE_WHEN_ONLINE && s.Presence == nil {
		return nil, status.Error(codes.Unavailable, "scheduled presence dependencies unavailable")
	}
	profileIDs := []uuid.UUID{request.SenderProfileID}
	if isDM {
		profileIDs = append(profileIDs, request.RecipientProfileID)
	}
	profiles, err := s.Profiles.GetByIDs(ctx, profileIDs)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "scheduled profile authority unavailable")
	}
	profiles, err = s.filterDeletedAccountProfiles(ctx, profiles)
	if err != nil {
		return nil, deletedAccountCheckUnavailable(err)
	}
	byID := make(map[uuid.UUID]*store.ProfileRow, len(profiles))
	for _, profile := range profiles {
		if profile != nil && profile.DeletedAt == nil {
			byID[profile.ID] = profile
		}
	}
	sender := byID[request.SenderProfileID]
	if sender == nil || sender.AccountID != request.SenderAccountID {
		return nil, status.Error(codes.PermissionDenied, "scheduled sender authority invalid")
	}
	if !isDM {
		return scheduledDispatchDecision(true, sender.IsGuestAccount, false), nil
	}
	privacyStore := s.privacyStore()
	if privacyStore == nil {
		return nil, status.Error(codes.Unavailable, "scheduled privacy dependencies unavailable")
	}
	recipient := byID[request.RecipientProfileID]
	if recipient == nil {
		return scheduledDispatchDecision(false, sender.IsGuestAccount, false), nil
	}

	policy, err := privacyStore.GetByProfileID(ctx, recipient.ID)
	if err != nil || policy == nil {
		return nil, status.Error(codes.Unavailable, "scheduled privacy authority unavailable")
	}
	matcher := s.audienceMatcher()
	if !scheduledAudienceDependenciesReady(matcher, policy.AllowDM) {
		return nil, status.Error(codes.Unavailable, "scheduled privacy dependencies unavailable")
	}
	allowedToDM, err := matcher.Allowed(ctx, recipient.ID, sender.ID, policy.AllowDM, sender.IsGuestAccount)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "scheduled privacy decision unavailable")
	}
	if !allowedToDM || (sender.IsGuestAccount && !policy.AllowGuestDM) {
		return scheduledDispatchDecision(false, sender.IsGuestAccount, false), nil
	}
	blocked, err := s.Blocks.AccountPairBlocked(ctx, sender.AccountID, recipient.AccountID)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "scheduled block authority unavailable")
	}
	if blocked {
		return scheduledDispatchDecision(false, sender.IsGuestAccount, false), nil
	}

	if request.Mode == userv1.ScheduledMessageDispatchMode_SCHEDULED_MESSAGE_DISPATCH_MODE_AT {
		return scheduledDispatchDecision(true, sender.IsGuestAccount, false), nil
	}
	if !scheduledAudienceDependenciesReady(matcher, policy.ShowOnline) {
		return nil, status.Error(codes.Unavailable, "scheduled privacy dependencies unavailable")
	}
	visible, err := matcher.Allowed(ctx, recipient.ID, sender.ID, policy.ShowOnline, sender.IsGuestAccount)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "scheduled privacy decision unavailable")
	}
	if !visible {
		return scheduledDispatchDecision(true, sender.IsGuestAccount, false), nil
	}
	snapshot, err := s.Presence.Get(ctx, recipient.ID)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "scheduled presence unavailable")
	}
	online, err := scheduledRecipientOnline(snapshot)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "scheduled presence state unavailable")
	}
	return scheduledDispatchDecision(true, sender.IsGuestAccount, online), nil
}

func scheduledDispatchDecision(policyAllowsDispatch, senderIsGuest, recipientOnline bool) *userv1.GetScheduledMessageDispatchPresenceResponse {
	return &userv1.GetScheduledMessageDispatchPresenceResponse{
		UserPolicyAllowsDispatch: policyAllowsDispatch,
		SenderIsGuest:            senderIsGuest,
		RecipientOnline:          recipientOnline,
	}
}

func scheduledAudienceDependenciesReady(matcher privacy.Matcher, audience privacy.Audience) bool {
	if audience.IsEveryoneShortcut() {
		return true
	}
	if (audience.Friends || audience.FriendsOfFriends) && matcher.Social == nil {
		return false
	}
	if audience.SpaceMembers && matcher.Space == nil {
		return false
	}
	return true
}

func scheduledRecipientOnline(snapshot *store.PresenceSnapshot) (bool, error) {
	if snapshot == nil || !snapshot.Live {
		return false, nil
	}
	if snapshot.StatusEnum != 0 {
		switch userv1.PresenceOnlineStatus(snapshot.StatusEnum) {
		case userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_ONLINE:
			return true, nil
		case userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_IDLE,
			userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_DND,
			userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_INVISIBLE:
			return false, nil
		default:
			return false, errors.New("unknown live presence enum")
		}
	}
	switch strings.ToLower(strings.TrimSpace(snapshot.Status)) {
	case "online":
		return true, nil
	case "idle", "dnd", "offline", "invisible":
		return false, nil
	default:
		return false, errors.New("unknown live presence status")
	}
}
