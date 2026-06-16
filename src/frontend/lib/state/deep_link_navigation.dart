import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/deep_links_client.dart';
import '../routing/deep_link_parser.dart';
import 'auth_providers.dart';
import 'gateway_providers.dart';
import 'chat_providers.dart';
import 'shell_providers.dart';
import 'space_providers.dart';

final voiceDeepLinksClientProvider = Provider<VoiceDeepLinksClient>((ref) {
  return VoiceDeepLinksClient(gateway: ref.watch(gatewayHttpClientProvider));
});

/// Applies resolved deep link targets to shell navigation.
Future<void> applyDeepLinkNavigation(
  WidgetRef ref,
  DeepLinkTarget target,
) async {
  switch (target.kind) {
    case DeepLinkKind.invite:
      final code = target.inviteCode;
      if (code == null) return;
      await ref.read(spaceInviteActionsProvider).joinByInvite(code: code);
    case DeepLinkKind.space:
      final spaceId = target.spaceId;
      if (spaceId != null) {
        ref.read(shellNavigationProvider).selectSpace(spaceId);
      }
    case DeepLinkKind.spaceChat:
    case DeepLinkKind.spaceMessage:
      final spaceId = target.spaceId;
      final chatId = target.chatId;
      if (spaceId != null) {
        ref.read(shellNavigationProvider).selectSpace(spaceId);
      }
      if (chatId != null) {
        ref.read(chatActionsProvider).selectChat(chatId);
      }
    case DeepLinkKind.chat:
    case DeepLinkKind.chatMessage:
      final chatId = target.chatId;
      if (chatId != null) {
        ref.read(chatActionsProvider).selectChat(chatId);
      }
    case DeepLinkKind.voiceRoom:
      final spaceId = target.spaceId;
      if (spaceId != null) {
        ref.read(shellNavigationProvider).selectSpace(spaceId);
      }
    case DeepLinkKind.profile:
    case DeepLinkKind.dm:
      break;
  }
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
  if (result case DeepLinksApiOk(:final data)) {
    if (data.kind == 'invite' && data.inviteCode != null) {
      await applyDeepLinkNavigation(
        ref,
        DeepLinkTarget(
          kind: DeepLinkKind.invite,
          inviteCode: data.inviteCode,
          rawUrl: target.rawUrl,
        ),
      );
      return;
    }
    if (data.spaceId != null && data.kind == 'space') {
      await applyDeepLinkNavigation(
        ref,
        DeepLinkTarget(
          kind: DeepLinkKind.space,
          spaceId: data.spaceId,
          rawUrl: target.rawUrl,
        ),
      );
      return;
    }
  }
  await applyDeepLinkNavigation(ref, target);
}
