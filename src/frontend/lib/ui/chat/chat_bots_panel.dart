import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/bots_client.dart';
import '../../backend/space_permissions.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/bot_providers.dart';
import '../../state/space_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_layout.dart';
import '../api_error_messages.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';

/// Panel / Chat / Bots — docs/design/screen-controls.md §53. Opened from the
/// Bots summary row of Chat Info: wide layouts swap the info screen for this
/// panel (the channel-settings lineage) and narrow layouts show it as a
/// bottom sheet with a back control.
class ChatBotsPanel extends ConsumerWidget {
  const ChatBotsPanel({
    super.key,
    required this.chatId,
    required this.spaceId,
    this.onClose,
  });

  static const Key panelKey = Key('chat_bots_panel');
  static const Key closeKey = Key('chat_bots_panel_close');
  static const Key backKey = Key('chat_bots_panel_back');
  static const Key installedRetryKey = Key('chat_bot_status_retry');
  static const Key emptyKey = Key('chat_bots_panel_empty');

  final String chatId;
  final String spaceId;
  final VoidCallback? onClose;

  static Future<void> show(
    BuildContext context, {
    required String chatId,
    required String spaceId,
  }) {
    final container = ProviderScope.containerOf(context);
    return showVoiceBottomSheet<void>(
      context: context,
      scrollable: false,
      child: UncontrolledProviderScope(
        container: container,
        child: ChatBotsPanel(chatId: chatId, spaceId: spaceId),
      ),
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final canManage =
        ref
            .watch(
              spacePermissionProvider((
                spaceId: spaceId,
                permission: SpacePermissions.spaceManageBots,
                chatId: null,
                voiceRoomId: null,
              )),
            )
            .valueOrNull ??
        false;
    if (!canManage) return const SizedBox.shrink();

    final botsAsync = ref.watch(
      botsInChatProvider((chatId: chatId, spaceId: spaceId)),
    );
    final installedBotsAsync = ref.watch(installedBotsProvider(spaceId));
    final installedBotsById = {
      for (final installed in installedBotsAsync.valueOrNull ?? const [])
        installed.bot.id: installed,
    };
    final isNarrow = VoiceLayout.isNarrow(MediaQuery.sizeOf(context).width);
    final dismiss =
        onClose ??
        () {
          if (!context.mounted) return;
          Navigator.of(context).pop();
        };

    return SafeArea(
      child: Column(
        key: panelKey,
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              if (isNarrow)
                IconButton(
                  key: backKey,
                  tooltip: l10n.chatInfoTitle,
                  onPressed: dismiss,
                  icon: const Icon(Icons.arrow_back),
                ),
              Expanded(
                child: Text(
                  l10n.chatBotsSectionTitle,
                  style: Theme.of(context).textTheme.titleMedium,
                ),
              ),
              IconButton(
                key: closeKey,
                tooltip: l10n.chatInfoTitle,
                onPressed: dismiss,
                icon: const Icon(Icons.close),
              ),
            ],
          ),
          Divider(height: 1, color: voice.borderDefault),
          if (installedBotsAsync.hasError)
            Padding(
              padding: const EdgeInsetsDirectional.fromSTEB(16, 0, 16, 8),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      l10n.chatBotsLoadError,
                      style: TextStyle(color: voice.textSecondary),
                    ),
                  ),
                  TextButton.icon(
                    key: installedRetryKey,
                    onPressed: () =>
                        ref.invalidate(installedBotsProvider(spaceId)),
                    icon: const Icon(Icons.refresh),
                    label: Text(l10n.commonRetry),
                  ),
                ],
              ),
            ),
          botsAsync.when(
            loading: () => const Padding(
              padding: EdgeInsets.all(16),
              child: VoiceListSkeleton(rowCount: 2),
            ),
            error: (error, _) => Padding(
              padding: const EdgeInsets.all(16),
              child: VoiceStatePanel(
                title: l10n.chatBotsSectionTitle,
                message: l10n.chatBotsLoadError,
                icon: Icons.cloud_off_outlined,
                actionLabel: l10n.commonRetry,
                onAction: () => ref.invalidate(
                  botsInChatProvider((chatId: chatId, spaceId: spaceId)),
                ),
              ),
            ),
            data: (bots) {
              if (bots.isEmpty) {
                return Padding(
                  key: emptyKey,
                  padding: const EdgeInsets.all(16),
                  child: Text(l10n.chatBotsEmpty),
                );
              }
              return Column(
                children: [
                  for (final entry in bots)
                    Row(
                      children: [
                        const Icon(Icons.smart_toy_outlined),
                        const SizedBox(width: 8),
                        Expanded(
                          child: SwitchListTile(
                            key: Key('chat_bot_toggle_${entry.bot.id}'),
                            title: Text(
                              entry.bot.name,
                              style:
                                  installedBotsById[entry.bot.id]?.online ==
                                      false
                                  ? TextStyle(color: voice.textDisabled)
                                  : null,
                            ),
                            subtitle: !entry.whitelisted
                                ? Text(l10n.spaceBotsSelectChats)
                                : entry.bot.description.isEmpty
                                ? null
                                : Text(
                                    entry.bot.description,
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                  ),
                            secondary:
                                installedBotsById[entry.bot.id]?.online == false
                                ? Tooltip(
                                    key: Key(
                                      'chat_bot_offline_indicator_${entry.bot.id}',
                                    ),
                                    message: l10n.botUnavailableTooltip,
                                    child: DecoratedBox(
                                      decoration: BoxDecoration(
                                        color: voice.textDisabled.withValues(
                                          alpha: 0.12,
                                        ),
                                        borderRadius: BorderRadius.all(
                                          Radius.circular(9),
                                        ),
                                      ),
                                      child: Padding(
                                        padding: const EdgeInsets.symmetric(
                                          horizontal: 8,
                                          vertical: 3,
                                        ),
                                        child: Text(
                                          l10n.botOfflineStatus,
                                          style: TextStyle(
                                            fontSize: 11,
                                            color: voice.textDisabled,
                                          ),
                                        ),
                                      ),
                                    ),
                                  )
                                : null,
                            value: entry.enabled && entry.whitelisted,
                            onChanged: (enabled) async {
                              final auth = ref.read(
                                authorizationHeaderProvider,
                              );
                              if (auth == null) return;
                              final chatType =
                                  ref.read(chatTypeForChatProvider(chatId)) ??
                                  'CHAT_TYPE_CHANNEL';
                              final result = await ref
                                  .read(voiceBotsClientProvider)
                                  .setBotChatEnabled(
                                    authorization: auth,
                                    botId: entry.bot.id,
                                    chatId: chatId,
                                    chatType: chatType,
                                    spaceId: spaceId,
                                    enabled: enabled,
                                  );
                              if (!context.mounted) return;
                              switch (result) {
                                case BotsApiOk():
                                  ref.invalidate(
                                    botsInChatProvider((
                                      chatId: chatId,
                                      spaceId: spaceId,
                                    )),
                                  );
                                  ref.invalidate(
                                    slashCommandsForChatProvider(chatId),
                                  );
                                case BotsApiFailure(
                                  :final message,
                                  :final statusCode,
                                ):
                                  ScaffoldMessenger.of(context).showSnackBar(
                                    SnackBar(
                                      content: Text(
                                        chatRoomErrorMessage(
                                          l10n,
                                          message,
                                          statusCode: statusCode,
                                        ),
                                      ),
                                    ),
                                  );
                              }
                            },
                          ),
                        ),
                      ],
                    ),
                ],
              );
            },
          ),
        ],
      ),
    );
  }
}
