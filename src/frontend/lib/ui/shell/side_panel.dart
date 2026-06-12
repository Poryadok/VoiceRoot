import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/chat_providers.dart';
import '../../state/shell_providers.dart';
import '../../state/space_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_layout.dart';
import '../chat/group_members_sheet.dart';
import '../space/space_members_sheet.dart';

/// Right-side panel host for members and emoji picker (desktop).
class SidePanelHost extends ConsumerWidget {
  const SidePanelHost({super.key, this.onEmojiSelected});

  static const Key hostKey = Key('side_panel_host');
  static const Key emojiPickerKey = Key('side_panel_emoji_picker');

  final void Function(String emoji)? onEmojiSelected;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final panel = ref.watch(shellSidePanelProvider);

    if (panel == ShellSidePanel.none) {
      return const SizedBox.shrink();
    }

    final shellNav = ref.read(shellNavigationProvider);
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);

    Widget body;
    String title;
    switch (panel) {
      case ShellSidePanel.members:
        title = _membersTitle(ref, l10n);
        body = _MembersPanelBody();
      case ShellSidePanel.emoji:
        title = l10n.chatMessageAddReaction;
        body = _EmojiPickerBody(onSelected: onEmojiSelected);
      case ShellSidePanel.none:
        return const SizedBox.shrink();
    }

    return Column(
      key: hostKey,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Material(
          color: voice.surface,
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 4),
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    title,
                    style: Theme.of(context).textTheme.titleSmall,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                IconButton(
                  tooltip: l10n.commonCancel,
                  onPressed: shellNav.closeSidePanel,
                  icon: const Icon(Icons.close, size: 20),
                ),
              ],
            ),
          ),
        ),
        Divider(height: 1, color: voice.borderDefault),
        Expanded(child: body),
      ],
    );
  }

  String _membersTitle(WidgetRef ref, AppLocalizations l10n) {
    final chatId = ref.watch(selectedChatIdProvider);
    final spaceId = ref.watch(selectedSpaceIdProvider);
    if (chatId != null) {
      for (final item in ref.watch(chatListControllerProvider).items) {
        if (item.chatId == chatId && item.chat.isGroup) {
          return item.chat.name ?? l10n.chatGroupMembersTitle;
        }
      }
    }
    if (spaceId != null) {
      final space = ref.watch(spaceProvider(spaceId)).valueOrNull;
      if (space != null) return space.name;
    }
    return l10n.chatGroupMembersTitle;
  }
}

class _MembersPanelBody extends ConsumerWidget {
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final chatId = ref.watch(selectedChatIdProvider);
    final spaceId = ref.watch(selectedSpaceIdProvider);

    if (chatId != null) {
      String? groupName;
      for (final item in ref.watch(chatListControllerProvider).items) {
        if (item.chatId == chatId && item.chat.isGroup) {
          groupName = item.chat.name;
          break;
        }
      }
      return Padding(
        padding: const EdgeInsets.fromLTRB(8, 8, 8, 8),
        child: GroupMembersContent(
          chatId: chatId,
          groupName: groupName,
          showHeader: false,
        ),
      );
    }
    if (spaceId != null) {
      return Padding(
        padding: const EdgeInsets.fromLTRB(8, 8, 8, 8),
        child: SpaceMembersContent(spaceId: spaceId, showHeader: false),
      );
    }
    return const SizedBox.shrink();
  }
}

class _EmojiPickerBody extends StatelessWidget {
  const _EmojiPickerBody({this.onSelected});

  final void Function(String emoji)? onSelected;

  static const _choices = ['👍', '❤️', '😂', '😮', '😢'];

  @override
  Widget build(BuildContext context) {
    return GridView.count(
      key: SidePanelHost.emojiPickerKey,
      crossAxisCount: 5,
      padding: const EdgeInsets.all(12),
      children: [
        for (final emoji in _choices)
          InkWell(
            onTap: () => onSelected?.call(emoji),
            child: Center(
              child: Text(emoji, style: const TextStyle(fontSize: 28)),
            ),
          ),
      ],
    );
  }
}

/// Opens members in side panel (desktop) or bottom sheet (narrow).
void openMembersPanel(
  BuildContext context,
  WidgetRef ref, {
  required String chatId,
  String? groupName,
}) {
  final narrow = VoiceLayout.isNarrow(MediaQuery.sizeOf(context).width);
  if (narrow) {
    GroupMembersSheet.show(context, chatId: chatId, groupName: groupName);
    return;
  }
  ref.read(shellNavigationProvider).toggleSidePanel(ShellSidePanel.members);
}

/// Opens emoji picker in side panel (desktop) or bottom sheet (narrow).
Future<void> openEmojiPanel(
  BuildContext context,
  WidgetRef ref, {
  required void Function(String emoji) onSelected,
}) async {
  final narrow = VoiceLayout.isNarrow(MediaQuery.sizeOf(context).width);
  if (narrow) {
    final emoji = await showModalBottomSheet<String>(
      context: context,
      builder: (ctx) => SafeArea(
        child: Wrap(
          alignment: WrapAlignment.center,
          spacing: 8,
          children: [
            for (final e in _EmojiPickerBody._choices)
              IconButton(
                onPressed: () => Navigator.of(ctx).pop(e),
                icon: Text(e, style: const TextStyle(fontSize: 28)),
              ),
          ],
        ),
      ),
    );
    if (emoji != null) onSelected(emoji);
    return;
  }
  final current = ref.read(shellSidePanelProvider);
  if (current == ShellSidePanel.emoji) {
    ref.read(shellNavigationProvider).closeSidePanel();
  } else {
    ref.read(shellNavigationProvider).toggleSidePanel(ShellSidePanel.emoji);
  }
}
