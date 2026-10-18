import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/bots_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/bot_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_layout.dart';
import '../core/voice_avatar.dart';
import '../core/voice_state_panel.dart';
import '../a11y/focus_trap.dart';
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
      child: SingleChildScrollView(
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
                      physics: const NeverScrollableScrollPhysics(),
                      itemCount: grouped.length,
                      itemBuilder: (context, index) {
                        final entry = grouped[index];
                        if (entry is _SlashMenuHeader) {
                          return Padding(
                            padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
                            child: Semantics(
                              header: true,
                              label: entry.label,
                              excludeSemantics: true,
                              child: Row(
                                children: [
                                  VoiceAvatar(label: entry.botName, radius: 10),
                                  const SizedBox(width: 8),
                                  Expanded(
                                    child: Text(
                                      entry.label,
                                      style: Theme.of(context)
                                          .textTheme
                                          .labelLarge
                                          ?.copyWith(
                                            color: Theme.of(
                                              context,
                                            ).colorScheme.onSurfaceVariant,
                                          ),
                                    ),
                                  ),
                                ],
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
                            onTap: offline
                                ? null
                                : () => widget.onSelected(cmd),
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
        out.add(_SlashMenuHeader(header, cmd.botName));
        lastHeader = header;
      }
      out.add(_SlashMenuItem(cmd));
    }
    return out;
  }
}

sealed class _SlashMenuEntry {}

class _SlashMenuHeader extends _SlashMenuEntry {
  _SlashMenuHeader(this.label, this.botName);
  final String label;
  final String botName;
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
  BuildContext? anchorContext,
  String filter = '',
  required Future<void> Function(BotSlashCommand command) onSelected,
}) async {
  final focusReturn = VoiceFocusReturn.capture();
  Widget menu(BuildContext routeContext) => SlashCommandMenuSheet(
    chatId: chatId,
    filter: filter,
    onSelected: (command) => Navigator.of(routeContext).pop(command),
  );
  try {
    final BotSlashCommand? selected;
    if (VoiceLayout.isNarrow(MediaQuery.sizeOf(context).width)) {
      selected = await showModalBottomSheet<BotSlashCommand>(
        context: context,
        isScrollControlled: true,
        builder: (routeContext) {
          final media = MediaQuery.of(routeContext);
          return Padding(
            padding: EdgeInsets.only(bottom: media.viewInsets.bottom),
            child: ConstrainedBox(
              constraints: BoxConstraints(
                maxHeight: (media.size.height - media.viewInsets.bottom) * 0.7,
              ),
              child: menu(routeContext),
            ),
          );
        },
      );
    } else {
      // Typing the slash may wrap the composer and move its top edge.
      // Measure after that layout, rather than inside the onChanged callback.
      final navigator = Navigator.of(context, rootNavigator: true);
      final triggerContext = anchorContext ?? context;
      await WidgetsBinding.instance.endOfFrame;
      if (!navigator.mounted) return;
      if (!triggerContext.mounted) return;
      final overlay =
          navigator.overlay!.context.findRenderObject()! as RenderBox;
      final trigger = triggerContext.findRenderObject()! as RenderBox;
      final anchor =
          trigger.localToGlobal(Offset.zero, ancestor: overlay) & trigger.size;
      if (!context.mounted) return;
      selected = await showDialog<BotSlashCommand>(
        context: context,
        useSafeArea: false,
        barrierColor: Colors.transparent,
        builder: (routeContext) => CustomSingleChildLayout(
          delegate: _SlashMenuLayout(
            anchor,
            MediaQuery.paddingOf(routeContext),
          ),
          child: Material(
            color:
                Theme.of(routeContext).dialogTheme.backgroundColor ??
                Theme.of(routeContext).colorScheme.surface,
            elevation: 8,
            borderRadius: BorderRadius.circular(8),
            clipBehavior: Clip.antiAlias,
            child: VoiceFocusTrap(
              onEscape: () => Navigator.of(routeContext).pop(),
              child: menu(routeContext),
            ),
          ),
        ),
      );
    }
    if (selected != null) await onSelected(selected);
  } finally {
    focusReturn.restore();
  }
}

/// Reference desktop popup:430px wide,8px above the actual composer.
class _SlashMenuLayout extends SingleChildLayoutDelegate {
  const _SlashMenuLayout(this.anchor, this.padding);

  final Rect anchor;
  final EdgeInsets padding;
  static const margin = 8.0;

  @override
  BoxConstraints getConstraintsForChild(BoxConstraints constraints) {
    final width = (constraints.maxWidth - padding.horizontal - margin * 2)
        .clamp(0.0, 430.0);
    final screenHeight = (constraints.maxHeight - padding.vertical - margin * 2)
        .clamp(0.0, double.infinity);
    final above = anchor.top - padding.top - margin * 2;
    return BoxConstraints(
      minWidth: width,
      maxWidth: width,
      maxHeight: above >= 160 ? above.clamp(0.0, screenHeight) : screenHeight,
    );
  }

  @override
  Offset getPositionForChild(Size size, Size childSize) => Offset(
    anchor.left.clamp(
      padding.left + margin,
      size.width - padding.right - margin - childSize.width,
    ),
    (anchor.top - margin - childSize.height).clamp(
      padding.top + margin,
      size.height - padding.bottom - margin - childSize.height,
    ),
  );

  @override
  bool shouldRelayout(_SlashMenuLayout oldDelegate) =>
      anchor != oldDelegate.anchor || padding != oldDelegate.padding;
}
