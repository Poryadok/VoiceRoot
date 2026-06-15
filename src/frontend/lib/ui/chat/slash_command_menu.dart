import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/bots_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/bot_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_state_panel.dart';

class SlashCommandMenuSheet extends ConsumerWidget {
  const SlashCommandMenuSheet({
    super.key,
    required this.chatId,
    required this.onSelected,
    this.filter = '',
  });

  static const Key sheetKey = Key('slash_command_menu_sheet');

  final String chatId;
  final ValueChanged<BotSlashCommand> onSelected;
  final String filter;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final commandsAsync = ref.watch(slashCommandsForChatProvider(chatId));
    final normalizedFilter = filter.trim().toLowerCase();

    return SafeArea(
      key: sheetKey,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
            child: Text(
              l10n.slashCommandsTitle,
              style: Theme.of(context).textTheme.titleMedium,
            ),
          ),
          const Divider(height: 1),
          commandsAsync.when(
            loading: () => const Padding(
              padding: EdgeInsets.all(24),
              child: Center(child: CircularProgressIndicator()),
            ),
            error: (_, _) => VoiceStatePanel(
              title: l10n.slashCommandsTitle,
              message: l10n.chatRoomEmptyHint,
              icon: Icons.smart_toy_outlined,
            ),
            data: (commands) {
              final filtered = normalizedFilter.isEmpty
                  ? commands
                  : commands
                        .where(
                          (cmd) =>
                              cmd.name.toLowerCase().contains(normalizedFilter) ||
                              cmd.botName.toLowerCase().contains(
                                normalizedFilter,
                              ),
                        )
                        .toList(growable: false);
              if (filtered.isEmpty) {
                return VoiceStatePanel(
                  title: l10n.slashCommandsTitle,
                  message: l10n.chatRoomEmptyHint,
                  icon: Icons.smart_toy_outlined,
                );
              }
              return ListView.builder(
                shrinkWrap: true,
                itemCount: filtered.length,
                itemBuilder: (context, index) {
                  final cmd = filtered[index];
                  return ListTile(
                    key: ValueKey('slash_command_${cmd.botId}_${cmd.name}'),
                    leading: Icon(Icons.terminal, color: voice.profileAccent),
                      title: Text(cmd.displayName),
                      subtitle: Text(
                        cmd.description.isEmpty ? cmd.botName : cmd.description,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                      ),
                    onTap: () => onSelected(cmd),
                  );
                },
              );
            },
          ),
        ],
      ),
    );
  }
}

Future<void> showSlashCommandMenu({
  required BuildContext context,
  required WidgetRef ref,
  required String chatId,
  String filter = '',
  required Future<void> Function(BotSlashCommand command) onSelected,
}) {
  return showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    builder: (ctx) => Consumer(
      builder: (context, ref, _) {
        return SlashCommandMenuSheet(
          chatId: chatId,
          filter: filter,
          onSelected: (command) async {
            Navigator.pop(ctx);
            await onSelected(command);
          },
        );
      },
    ),
  );
}
