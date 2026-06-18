import 'dart:convert' show JsonDecoder;

import 'package:flutter/widgets.dart';

import '../l10n/app_localizations.dart';

/// Human-readable labels for bot manifest scopes — docs/features/bots.md.
abstract final class BotScopeLabels {
  static const privilegedScopes = {
    'TEXT_CHAT_READ_HISTORY',
    'SPACE_MANAGE_ROLES',
  };

  static String labelFor(BuildContext context, String scope) {
    final l10n = AppLocalizations.of(context)!;
    return switch (scope) {
      'TEXT_CHAT_SEND_MESSAGES' => l10n.botScopeTextChatSendMessages,
      'DM_SEND' => l10n.botScopeDmSend,
      'SPACE_VIEW_MEMBER_LIST' => l10n.botScopeSpaceViewMemberList,
      'MEMBER_ASSIGN_ROLES' => l10n.botScopeMemberAssignRoles,
      'TEXT_CHAT_CREATE_IN_SPACE' => l10n.botScopeTextChatCreateInSpace,
      'TEXT_CHAT_READ_HISTORY' => l10n.botScopeTextChatReadHistory,
      'SPACE_MANAGE_ROLES' => l10n.botScopeSpaceManageRoles,
      _ => scope,
    };
  }

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
