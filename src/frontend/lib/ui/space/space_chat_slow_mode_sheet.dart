import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/chats_client.dart';
import '../../backend/space_permissions.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/e2e_providers.dart';
import '../../state/space_providers.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_compact_banner.dart';
import '../core/voice_disabled_action.dart';

/// Channel slow mode picker for space text chats (5s – 6h per docs).
class SpaceChatSlowModeSheet extends ConsumerWidget {
  const SpaceChatSlowModeSheet({
    super.key,
    required this.chatId,
    this.spaceId,
    this.currentSeconds = 0,
  });

  static const Key sheetKey = Key('space_chat_slow_mode_sheet');

  final String chatId;
  final String? spaceId;
  final int currentSeconds;

  static const _options = <int>[0, 5, 10, 30, 60, 300, 600, 3600, 21600];

  static Future<void> show(
    BuildContext context, {
    required String chatId,
    String? spaceId,
    int currentSeconds = 0,
  }) {
    final container = ProviderScope.containerOf(context);
    return showVoiceBottomSheet<void>(
      context: context,
      child: UncontrolledProviderScope(
        container: container,
        child: SpaceChatSlowModeSheet(
          chatId: chatId,
          spaceId: spaceId,
          currentSeconds: currentSeconds,
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final chatMeta = chatMetadataForId(
      ref.watch(chatListControllerProvider).items,
      chatId,
    );
    final resolvedSpaceId = spaceId ?? chatMeta?.spaceId;
    final ({bool allowed, String? deniedReason}) slowModePermission;
    if (resolvedSpaceId == null) {
      slowModePermission = (allowed: true, deniedReason: null);
    } else {
      slowModePermission = resolveSpacePermission(
        l10n,
        ref.watch(
          spacePermissionProvider((
            spaceId: resolvedSpaceId,
            permission: SpacePermissions.textChatSetSlowMode,
            chatId: chatId,
            voiceRoomId: null,
          )),
        ),
        SpacePermissions.textChatSetSlowMode,
      );
    }
    final canSetSlowMode = slowModePermission.allowed;

    return SafeArea(
      child: Padding(
        key: sheetKey,
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(l10n.spaceSlowMode, style: theme.textTheme.titleMedium),
            const SizedBox(height: 4),
            Text(l10n.spaceSlowModeSubtitle, style: theme.textTheme.bodySmall),
            if (slowModePermission.deniedReason != null) ...[
              const SizedBox(height: 8),
              VoiceCompactBanner(
                key: const Key('space_slow_mode_denied'),
                message: slowModePermission.deniedReason!,
                icon: Icons.lock_outline,
                tone: VoiceBannerTone.warning,
              ),
            ],
            const SizedBox(height: 12),
            for (final seconds in _options)
              VoiceDisabledAction(
                disabledReason: slowModePermission.deniedReason,
                child: ListTile(
                  key: Key('slow_mode_option_$seconds'),
                  enabled: canSetSlowMode,
                  title: Text(
                    seconds == 0
                        ? l10n.spaceSlowModeOff
                        : l10n.spaceSlowModeSeconds(seconds),
                  ),
                  trailing: seconds == currentSeconds
                      ? const Icon(Icons.check)
                      : null,
                  onTap: canSetSlowMode
                      ? () => _apply(context, ref, seconds)
                      : null,
                ),
              ),
          ],
        ),
      ),
    );
  }

  Future<void> _apply(BuildContext context, WidgetRef ref, int seconds) async {
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    final client = ref.read(voiceChatsClientProvider);
    final result = await client.updateGroup(
      authorization: auth,
      chatId: chatId,
      slowModeSeconds: seconds,
    );
    if (!context.mounted) return;
    if (result is ChatsApiFailure) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(result.message)),
      );
      return;
    }
    ref.invalidate(chatListControllerProvider);
    Navigator.of(context).pop();
  }
}
