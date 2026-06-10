package permissions

import "fmt"

// Canonical permission names — docs/microservices/role-service.md.
const (
	SpaceView               = "SPACE_VIEW"
	SpaceManageSettings     = "SPACE_MANAGE_SETTINGS"
	SpaceManageRoles        = "SPACE_MANAGE_ROLES"
	SpaceManageInvites      = "SPACE_MANAGE_INVITES"
	SpaceViewAuditLog       = "SPACE_VIEW_AUDIT_LOG"
	SpaceManageCustomEmojis = "SPACE_MANAGE_CUSTOM_EMOJIS"
	SpaceManageBots         = "SPACE_MANAGE_BOTS"
	SpaceManageMatchmaking  = "SPACE_MANAGE_MATCHMAKING"
	SpaceViewMemberList     = "SPACE_VIEW_MEMBER_LIST"

	MemberKick            = "MEMBER_KICK"
	MemberBan             = "MEMBER_BAN"
	MemberManageNicknames = "MEMBER_MANAGE_NICKNAMES"
	MemberAssignRoles     = "MEMBER_ASSIGN_ROLES"

	TextChatCreateInSpace   = "TEXT_CHAT_CREATE_IN_SPACE"
	TextChatView            = "TEXT_CHAT_VIEW"
	TextChatManageSettings  = "TEXT_CHAT_MANAGE_SETTINGS"
	TextChatSendMessages    = "TEXT_CHAT_SEND_MESSAGES"
	TextChatManageMessages  = "TEXT_CHAT_MANAGE_MESSAGES"
	TextChatSetSlowMode     = "TEXT_CHAT_SET_SLOW_MODE"

	ModerationTimeoutMembers = "MODERATION_TIMEOUT_MEMBERS"

	TextChatMentionAllOnline = "TEXT_CHAT_MENTION_ALL_ONLINE"
	TextChatMentionAllInChat = "TEXT_CHAT_MENTION_ALL_IN_CHAT"

	VoiceJoin       = "VOICE_JOIN"
	VoiceSpeak      = "VOICE_SPEAK"
	VoiceMuteOthers = "VOICE_MUTE_OTHERS"
)

// System role names seeded per space — docs/features/roles.md.
const (
	RoleOwner     = "Owner"
	RoleAdmin     = "Admin"
	RoleModerator = "Moderator"
	RoleMember    = "Member"
	RoleGuest     = "Guest"
)

// Bit positions are frozen in role_db v1 migrations.
var permissionBits = map[string]uint64{
	SpaceView:               1 << 0,
	SpaceManageSettings:     1 << 1,
	SpaceManageRoles:        1 << 2,
	SpaceManageInvites:      1 << 3,
	SpaceViewAuditLog:       1 << 4,
	SpaceManageCustomEmojis: 1 << 5,
	SpaceManageBots:         1 << 6,
	SpaceManageMatchmaking:  1 << 7,
	SpaceViewMemberList:     1 << 8,
	MemberKick:              1 << 9,
	MemberBan:               1 << 10,
	MemberManageNicknames:   1 << 11,
	MemberAssignRoles:       1 << 12,
	TextChatCreateInSpace:    1 << 13,
	TextChatView:             1 << 14,
	TextChatSendMessages:     1 << 15,
	TextChatManageMessages:   1 << 16,
	VoiceJoin:                1 << 17,
	VoiceSpeak:               1 << 18,
	VoiceMuteOthers:          1 << 19,
	TextChatManageSettings:   1 << 20,
	TextChatSetSlowMode:      1 << 21,
	ModerationTimeoutMembers: 1 << 22,
	TextChatMentionAllOnline:  1 << 23,
	TextChatMentionAllInChat:  1 << 24,
}

// MaskFor returns the bitmask bit for a permission name.
func MaskFor(name string) (uint64, error) {
	bit, ok := permissionBits[name]
	if !ok {
		return 0, fmt.Errorf("unknown permission %q", name)
	}
	return bit, nil
}

// NamesFor expands a bitmask to permission names.
func NamesFor(mask uint64) []string {
	var names []string
	for name, bit := range permissionBits {
		if mask&bit != 0 {
			names = append(names, name)
		}
	}
	return names
}

// AllMask returns the union of every defined permission bit (owner bypass).
func AllMask() (uint64, error) {
	var mask uint64
	for _, bit := range permissionBits {
		mask |= bit
	}
	return mask, nil
}

// SystemRoleSpec defines a seeded system role.
type SystemRoleSpec struct {
	Name     string
	Position int32
	Mask     uint64
}

// SystemRoles returns the five default roles for a space.
func SystemRoles() ([]SystemRoleSpec, error) {
	all, err := AllMask()
	if err != nil {
		return nil, err
	}
	memberMask, err := memberDefaultMask()
	if err != nil {
		return nil, err
	}
	modMask, err := moderatorDefaultMask()
	if err != nil {
		return nil, err
	}
	guestMask, err := guestDefaultMask()
	if err != nil {
		return nil, err
	}
	return []SystemRoleSpec{
		{Name: RoleOwner, Position: 4, Mask: all},
		{Name: RoleAdmin, Position: 3, Mask: all},
		{Name: RoleModerator, Position: 2, Mask: modMask},
		{Name: RoleMember, Position: 1, Mask: memberMask},
		{Name: RoleGuest, Position: 0, Mask: guestMask},
	}, nil
}

func memberDefaultMask() (uint64, error) {
	parts := []string{
		SpaceView, SpaceViewMemberList, TextChatView, TextChatSendMessages,
		TextChatCreateInSpace, VoiceJoin, VoiceSpeak,
	}
	var mask uint64
	for _, p := range parts {
		bit, err := MaskFor(p)
		if err != nil {
			return 0, err
		}
		mask |= bit
	}
	return mask, nil
}

func moderatorDefaultMask() (uint64, error) {
	member, err := memberDefaultMask()
	if err != nil {
		return 0, err
	}
	for _, p := range []string{
		MemberKick, MemberBan, MemberAssignRoles, TextChatManageMessages, VoiceMuteOthers,
		ModerationTimeoutMembers, TextChatSetSlowMode,
		TextChatMentionAllOnline, TextChatMentionAllInChat,
	} {
		bit, err := MaskFor(p)
		if err != nil {
			return 0, err
		}
		member |= bit
	}
	return member, nil
}

func guestDefaultMask() (uint64, error) {
	return MaskFor(SpaceView)
}

// HasPermission reports whether mask includes permission name.
func HasPermission(mask uint64, name string) (bool, error) {
	bit, err := MaskFor(name)
	if err != nil {
		return false, err
	}
	return mask&bit != 0, nil
}
