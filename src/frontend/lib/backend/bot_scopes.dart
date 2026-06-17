import 'dart:convert' show JsonDecoder;

/// Human-readable labels for bot manifest scopes — docs/features/bots.md.
abstract final class BotScopeLabels {
  static const privilegedScopes = {
    'TEXT_CHAT_READ_HISTORY',
    'SPACE_MANAGE_ROLES',
  };

  static const _labels = <String, String>{
    'TEXT_CHAT_SEND_MESSAGES': 'Send messages in allowed text chats',
    'DM_SEND': 'Send direct messages (reply only)',
    'SPACE_VIEW_MEMBER_LIST': 'View space member list',
    'MEMBER_ASSIGN_ROLES': 'Assign roles below the bot',
    'TEXT_CHAT_CREATE_IN_SPACE': 'Create text chats in the space',
    'TEXT_CHAT_READ_HISTORY': 'Read message history (privileged)',
    'SPACE_MANAGE_ROLES': 'Create and manage roles below the bot (privileged)',
  };

  static String labelFor(String scope) => _labels[scope] ?? scope;

  static List<String> parseScopesJson(String scopesJson) {
    try {
      if (scopesJson.startsWith('[')) {
        final list = (const JsonDecoder().convert(scopesJson) as List<dynamic>)
            .map((e) => e.toString())
            .toList(growable: false);
        return list;
      }
    } catch (_) {}
    return const [];
  }
}
