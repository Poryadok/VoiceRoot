import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/bots_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/bot_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_state_panel.dart';
import '../a11y/voice_focus_return.dart';

class SlashCommandMenuSheet extends ConsumerStatefulWidget {
  const SlashCommandMenuSheet({
    super.key,
    required this.chatId,
    required this.onSelected,
    this.filter = '',
  });

  static const Key sheetKey = Key('slash_command_menu_sheet');
  static const Key emptyStateKey = Key('slash_commands_empty_panel');
  static const Key noMatchStateKey = Key('slash_commands_no_match_panel');
  static const Key helpFooterKey = Key('slash_commands_help_footer');
  static const Key searchFieldKey = Key('slash_commands_search_field');

  final String chatId;
  final ValueChanged<BotSlashCommand> onSelected;
  final String filter;

  @override
  @override
  ConsumerState<SlashCommandMenuSheet> createState() =>
      _SlashCommandMenuSheetState();
}

class _SlashCommandMenuSheetState extends ConsumerState<SlashCommandMenuSheet> {
  late final TextEditingController _searchController;

  @override
  void initState() {
    super.initState();
    _searchController = TextEditingController(text: widget.filter);
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final commandsAsync = ref.watch(
      slashCommandsForChatProvider(widget.chatId),
    );
    final spaceId = ref.watch(spaceIdForChatProvider(widget.chatId));
    final normalizedFilter = _searchController.text.trim().toLowerCase();

    return SafeArea(
      key: SlashCommandMenuSheet.sheetKey,
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
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 4, 16, 12),
            child: TextField(
              key: SlashCommandMenuSheet.searchFieldKey,
              controller: _searchController,
              autofocus: true,
              decoration: InputDecoration(
                labelText: l10n.socialTabSearch,
                prefixIcon: const Icon(Icons.search),
              ),
              onChanged: (_) => setState(() {}),
            ),
          ),
          const Divider(height: 1),
          commandsAsync.when(
            loading: () => const Padding(
              padding: EdgeInsets.all(24),
              child: Center(child: CircularProgressIndicator()),
            ),
            error: (error, _) => VoiceStatePanel(
              title: l10n.slashCommandsTitle,
              message: l10n.slashCommandsLoadError,
              icon: Icons.cloud_off_outlined,
              actionLabel: l10n.commonRetry,
              onAction: () =>
                  ref.invalidate(slashCommandsForChatProvider(widget.chatId)),
            ),
            data: (commands) {
              final filtered = normalizedFilter.isEmpty
                  ? commands
                  : commands
                        .where(
                          (cmd) =>
                              cmd.fullCommandName.toLowerCase().contains(
                                normalizedFilter,
                              ) ||
                              cmd.botName.toLowerCase().contains(
                                normalizedFilter,
                              ),
                        )
                        .toList(growable: false);
              if (filtered.isEmpty) {
                final isFilterMiss =
                    normalizedFilter.isNotEmpty && commands.isNotEmpty;
                return VoiceStatePanel(
                  key: isFilterMiss
                      ? SlashCommandMenuSheet.noMatchStateKey
                      : SlashCommandMenuSheet.emptyStateKey,
                  title: isFilterMiss
                      ? l10n.slashCommandsNoMatch
                      : l10n.slashCommandsEmpty,
                  message: isFilterMiss
                      ? l10n.slashCommandsNoMatchHint
                      : _emptyHint(l10n, spaceId),
                  icon: isFilterMiss
                      ? Icons.search_off_outlined
                      : Icons.smart_toy_outlined,
                );
              }
              final grouped = _groupCommands(filtered);
              return Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  ListView.builder(
                    shrinkWrap: true,
                    itemCount: grouped.length,
                    itemBuilder: (context, index) {
                      final entry = grouped[index];
                      if (entry is _SlashMenuHeader) {
                        return Padding(
                          padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
                          child: Text(
                            entry.label,
                            style: Theme.of(context).textTheme.labelLarge
                                ?.copyWith(
                                  color: Theme.of(
                                    context,
                                  ).colorScheme.onSurfaceVariant,
                                ),
                          ),
                        );
                      }
                      final cmd = (entry as _SlashMenuItem).command;
                      final offline = !cmd.online;
                      return Tooltip(
                        message: offline ? l10n.botUnavailableTooltip : '',
                        child: ListTile(
                          key: ValueKey(
                            'slash_command_${cmd.botId}_${cmd.fullCommandName}',
                          ),
                          enabled: !offline,
                          leading: Icon(
                            Icons.terminal,
                            color: offline
                                ? voice.textDisabled
                                : voice.profileAccent,
                          ),
                          title: Text(
                            cmd.displayName,
                            style: offline
                                ? TextStyle(color: voice.textDisabled)
                                : null,
                          ),
                          subtitle: Text(
                            cmd.description.isEmpty
                                ? cmd.botName
                                : cmd.description,
                            maxLines: 2,
                            overflow: TextOverflow.ellipsis,
                            style: offline
                                ? TextStyle(color: voice.textDisabled)
                                : null,
                          ),
                          onTap: offline ? null : () => widget.onSelected(cmd),
                        ),
                      );
                    },
                  ),
                  const Divider(height: 1),
                  _SlashCommandsHelpFooter(
                    key: SlashCommandMenuSheet.helpFooterKey,
                    message: l10n.slashCommandsHelp,
                  ),
                ],
              );
            },
          ),
        ],
      ),
    );
  }

  String _emptyHint(AppLocalizations l10n, String? spaceId) {
    final inSpace = spaceId != null && spaceId.isNotEmpty;
    return inSpace
        ? l10n.slashCommandsEmptyHint
        : l10n.slashCommandsDmEmptyHint;
  }

  List<_SlashMenuEntry> _groupCommands(List<BotSlashCommand> commands) {
    final sorted = [...commands]
      ..sort((a, b) {
        final bot = a.botName.compareTo(b.botName);
        if (bot != 0) return bot;
        final group = (a.groupName ?? '').compareTo(b.groupName ?? '');
        if (group != 0) return group;
        return a.name.compareTo(b.name);
      });
    final out = <_SlashMenuEntry>[];
    String? lastHeader;
    for (final cmd in sorted) {
      final header = cmd.groupName != null && cmd.groupName!.isNotEmpty
          ? '${cmd.botName} / ${cmd.groupName}'
          : cmd.botName;
      if (header != lastHeader) {
        out.add(_SlashMenuHeader(header));
        lastHeader = header;
      }
      out.add(_SlashMenuItem(cmd));
    }
    return out;
  }
}

sealed class _SlashMenuEntry {}

class _SlashMenuHeader extends _SlashMenuEntry {
  _SlashMenuHeader(this.label);
  final String label;
}

class _SlashMenuItem extends _SlashMenuEntry {
  _SlashMenuItem(this.command);
  final BotSlashCommand command;
}

class _SlashCommandsHelpFooter extends StatelessWidget {
  const _SlashCommandsHelpFooter({super.key, required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 10, 16, 12),
      child: Text(
        message,
        textAlign: TextAlign.center,
        style: Theme.of(
          context,
        ).textTheme.bodySmall?.copyWith(color: voice.textSecondary),
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
  final focusReturn = VoiceFocusReturn.capture();
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
  ).whenComplete(focusReturn.restore);
}
