import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/deep_links_client.dart';
import '../backend/users_client.dart';
import '../routing/app_router.dart';
import '../routing/deep_link_parser.dart';
import '../routing/deep_link_urls.dart';
import '../ui/bots/bot_install_page.dart';
import '../ui/social/profile_detail_sheet.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';
import 'shared_media_providers.dart';
import 'shell_providers.dart';
import 'social_providers.dart';
import 'space_providers.dart';

final voiceDeepLinksClientProvider = Provider<VoiceDeepLinksClient>((ref) {
  return VoiceDeepLinksClient(gateway: ref.watch(gatewayHttpClientProvider));
});

void _scrollToMessage(Ref ref, String chatId, String messageId) {
  ref.read(pendingChatMessageScrollProvider(chatId).notifier).state =
      messageId;
  ref.read(pendingChatMessageHighlightProvider(chatId).notifier).state =
      messageId;
}

void _ensureChatsSectionForPush(Ref ref, DeepLinkKind kind) {
  switch (kind) {
    case DeepLinkKind.chat:
    case DeepLinkKind.chatMessage:
    case DeepLinkKind.spaceChat:
    case DeepLinkKind.spaceMessage:
    case DeepLinkKind.dm:
      ref.read(navigationSectionProvider.notifier).state =
          NavigationSection.chats;
    default:
      break;
  }
}

/// Applies resolved deep link targets to shell navigation (Ref-safe for push/WS).
class DeepLinkNavigator {
  DeepLinkNavigator(this._ref);

  final Ref _ref;

  Future<void> apply(DeepLinkTarget target) async {
    _ensureChatsSectionForPush(_ref, target.kind);
    switch (target.kind) {
      case DeepLinkKind.invite:
        final code = target.inviteCode;
        if (code == null) return;
        await _ref.read(spaceInviteActionsProvider).joinByInvite(code: code);
      case DeepLinkKind.space:
        final spaceId = target.spaceId;
        if (spaceId != null) {
          _ref.read(shellNavigationProvider).selectSpace(spaceId);
        }
      case DeepLinkKind.spaceChat:
      case DeepLinkKind.spaceMessage:
        final spaceId = target.spaceId;
        final chatId = target.chatId;
        if (spaceId != null) {
          _ref.read(shellNavigationProvider).selectSpace(spaceId);
        }
        if (chatId != null) {
          _ref.read(chatActionsProvider).selectChat(chatId);
        }
        final messageId = target.messageId;
        if (messageId != null && chatId != null) {
          _scrollToMessage(_ref, chatId, messageId);
        }
      case DeepLinkKind.chat:
      case DeepLinkKind.chatMessage:
        final chatId = target.chatId;
        if (chatId != null) {
          _ref.read(chatActionsProvider).selectChat(chatId);
        }
        final messageId = target.messageId;
        if (messageId != null && chatId != null) {
          _scrollToMessage(_ref, chatId, messageId);
        }
      case DeepLinkKind.voiceRoom:
        final spaceId = target.spaceId;
        if (spaceId != null) {
          _ref.read(shellNavigationProvider).selectSpace(spaceId);
        }
      case DeepLinkKind.bot:
        final slug = target.botSlug;
        if (slug == null || slug.isEmpty) return;
        final ctx = rootNavigatorKey.currentContext;
        if (ctx == null || !ctx.mounted) return;
        await Navigator.of(ctx).push<void>(
          MaterialPageRoute<void>(
            builder: (context) => BotInstallPage(slug: slug),
          ),
        );
      case DeepLinkKind.profile:
        await _openProfileDeepLink(_ref, target);
      case DeepLinkKind.dm:
        await _openDmDeepLink(_ref, target);
    }
  }
}

final deepLinkNavigatorProvider = Provider<DeepLinkNavigator>(
  (ref) => DeepLinkNavigator(ref),
);

/// Widget-tree entry point for deep link navigation.
Future<void> applyDeepLinkNavigation(
  WidgetRef ref,
  DeepLinkTarget target,
) =>
    ref.read(deepLinkNavigatorProvider).apply(target);

Future<void> _openDmDeepLink(Ref ref, DeepLinkTarget target) async {
  final userId = target.userId;
  if (userId == null || userId.isEmpty) return;
  ref.read(navigationSectionProvider.notifier).state = NavigationSection.chats;
  await ref.read(chatActionsProvider).openDmWithProfile(userId);
}

Future<void> _openProfileDeepLink(Ref ref, DeepLinkTarget target) async {
  final username = target.username?.trim();
  if (username == null || username.isEmpty) return;
  final auth = ref.read(authControllerProvider);
  if (!auth.isAuthenticated || auth.session == null) return;

  final client = ref.read(voiceUsersClientProvider);
  final result = await client.searchProfiles(
    authorization: 'Bearer ${auth.session!.accessToken}',
    query: username,
    pageSize: 8,
  );
  String? profileId;
  if (result case UsersApiOk(:final data)) {
    for (final profile in data.profiles) {
      if (profile.username.toLowerCase() == username.toLowerCase()) {
        profileId = profile.id;
        break;
      }
    }
    profileId ??= data.profiles.firstOrNull?.id;
  }
  if (profileId == null) return;

  ref.read(navigationSectionProvider.notifier).state = NavigationSection.social;
  final ctx = rootNavigatorKey.currentContext;
  if (ctx == null || !ctx.mounted) return;
  await showModalBottomSheet<void>(
    context: ctx,
    isScrollControlled: true,
    builder: (sheetContext) => UncontrolledProviderScope(
      container: ProviderScope.containerOf(ctx),
      child: ProfileDetailSheet(profileId: profileId!),
    ),
  );
}

/// Resolves authenticated deep link via Gateway API when possible.
Future<void> resolveAndNavigateDeepLink(
  WidgetRef ref,
  DeepLinkTarget target,
) async {
  final auth = ref.read(authControllerProvider);
  if (!auth.isAuthenticated || auth.session == null) return;

  final client = ref.read(voiceDeepLinksClientProvider);
  final result = await client.resolve(
    authorization: 'Bearer ${auth.session!.accessToken}',
    url: target.rawUrl,
  );
  final navigator = ref.read(deepLinkNavigatorProvider);
  if (result case DeepLinksApiOk(:final data)) {
    if (data.kind == 'invite' && data.inviteCode != null) {
      await navigator.apply(
        DeepLinkTarget(
          kind: DeepLinkKind.invite,
          inviteCode: data.inviteCode,
          rawUrl: target.rawUrl,
        ),
      );
      return;
    }
    if (data.spaceId != null && data.kind == 'space') {
      await navigator.apply(
        DeepLinkTarget(
          kind: DeepLinkKind.space,
          spaceId: data.spaceId,
          rawUrl: target.rawUrl,
        ),
      );
      return;
    }
  }
  await navigator.apply(target);
}

/// Builds a share URL for the current chat/message context.
String? shareUrlForChat({
  required String chatId,
  String? spaceId,
  String? messageId,
}) {
  if (messageId != null && messageId.isNotEmpty) {
    if (spaceId != null && spaceId.isNotEmpty) {
      return spaceMessageShareUrl(
        spaceId: spaceId,
        chatId: chatId,
        messageId: messageId,
      );
    }
    return chatMessageShareUrl(chatId: chatId, messageId: messageId);
  }
  if (spaceId != null && spaceId.isNotEmpty) {
    return spaceChatShareUrl(spaceId: spaceId, chatId: chatId);
  }
  return chatShareUrl(chatId);
}
