/// Canonical space permission names — docs/microservices/role-service.md.
abstract final class SpacePermissions {
  static const spaceView = 'SPACE_VIEW';
  static const spaceManageSettings = 'SPACE_MANAGE_SETTINGS';
  static const spaceManageRoles = 'SPACE_MANAGE_ROLES';
  static const spaceManageInvites = 'SPACE_MANAGE_INVITES';
  static const memberKick = 'MEMBER_KICK';
  static const memberBan = 'MEMBER_BAN';
  static const memberAssignRoles = 'MEMBER_ASSIGN_ROLES';
  static const textChatSendMessages = 'TEXT_CHAT_SEND_MESSAGES';
  static const textChatSetSlowMode = 'TEXT_CHAT_SET_SLOW_MODE';
  static const textChatManageSettings = 'TEXT_CHAT_MANAGE_SETTINGS';
  static const voiceJoin = 'VOICE_JOIN';
  static const voiceSpeak = 'VOICE_SPEAK';
  static const voiceMuteOthers = 'VOICE_MUTE_OTHERS';
  static const moderationTimeoutMembers = 'MODERATION_TIMEOUT_MEMBERS';

  static const all = <String, int>{
    spaceView: 1 << 0,
    spaceManageSettings: 1 << 1,
    spaceManageRoles: 1 << 2,
    spaceManageInvites: 1 << 3,
    'SPACE_VIEW_AUDIT_LOG': 1 << 4,
    'SPACE_MANAGE_CUSTOM_EMOJIS': 1 << 5,
    'SPACE_MANAGE_BOTS': 1 << 6,
    'SPACE_MANAGE_MATCHMAKING': 1 << 7,
    'SPACE_VIEW_MEMBER_LIST': 1 << 8,
    memberKick: 1 << 9,
    memberBan: 1 << 10,
    'MEMBER_MANAGE_NICKNAMES': 1 << 11,
    memberAssignRoles: 1 << 12,
    'TEXT_CHAT_CREATE_IN_SPACE': 1 << 13,
    'TEXT_CHAT_VIEW': 1 << 14,
    textChatSendMessages: 1 << 15,
    'TEXT_CHAT_MANAGE_MESSAGES': 1 << 16,
    voiceJoin: 1 << 17,
    voiceSpeak: 1 << 18,
    voiceMuteOthers: 1 << 19,
    textChatManageSettings: 1 << 20,
    textChatSetSlowMode: 1 << 21,
    moderationTimeoutMembers: 1 << 22,
    'TEXT_CHAT_MENTION_ALL_ONLINE': 1 << 23,
    'TEXT_CHAT_MENTION_ALL_IN_CHAT': 1 << 24,
    'TEXT_CHAT_PIN_MESSAGES': 1 << 25,
    'TEXT_CHAT_SEND_MEDIA': 1 << 26,
    'TEXT_CHAT_EMBED_LINKS': 1 << 27,
    'TEXT_CHAT_ATTACH_FILES': 1 << 28,
    'TEXT_CHAT_ADD_REACTIONS': 1 << 29,
    'TEXT_CHAT_USE_EXTERNAL_EMOJIS': 1 << 30,
    'TEXT_CHAT_READ_HISTORY': 1 << 31,
    'TEXT_CHAT_CREATE_THREADS': 1 << 32,
    'TEXT_CHAT_SEND_IN_THREADS': 1 << 33,
    'TEXT_CHAT_MANAGE_THREADS': 1 << 34,
    'VOICE_VIDEO': 1 << 35,
    'VOICE_SCREEN_SHARE': 1 << 36,
    'VOICE_DEAFEN_OTHERS': 1 << 37,
    'VOICE_MOVE_OTHERS': 1 << 38,
    'VOICE_USE_PTT': 1 << 39,
    'VOICE_PRIORITY_SPEAKER': 1 << 40,
    'MODERATION_MANAGE_REPORTS': 1 << 41,
  };

  static const editableGroups = <String, List<String>>{
    'Space': [
      spaceView,
      spaceManageSettings,
      spaceManageRoles,
      spaceManageInvites,
      'SPACE_VIEW_MEMBER_LIST',
    ],
    'Members': [memberKick, memberBan, memberAssignRoles, 'MEMBER_MANAGE_NICKNAMES'],
    'Text chat': [
      'TEXT_CHAT_VIEW',
      textChatSendMessages,
      'TEXT_CHAT_SEND_MEDIA',
      'TEXT_CHAT_ATTACH_FILES',
      textChatManageSettings,
      textChatSetSlowMode,
      'TEXT_CHAT_MANAGE_MESSAGES',
    ],
    'Voice': [voiceJoin, voiceSpeak, voiceMuteOthers, 'VOICE_VIDEO'],
    'Moderation': [moderationTimeoutMembers],
  };

  static bool hasPermission(int mask, String name) {
    final bit = all[name];
    if (bit == null) return false;
    return mask & bit != 0;
  }

  static int setPermission(int mask, String name, bool enabled) {
    final bit = all[name];
    if (bit == null) return mask;
    if (enabled) return mask | bit;
    return mask & ~bit;
  }
}
