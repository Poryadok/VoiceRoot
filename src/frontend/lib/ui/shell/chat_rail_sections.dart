import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/chats_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_navigation_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/shell_providers.dart';
import '../../theme/voice_colors.dart';

/// Folder icons in desktop rail (§1.1b) — functional, icon + tooltip.
class ChatRailFoldersSection extends ConsumerWidget {
  const ChatRailFoldersSection({super.key});

  static const sectionKey = Key('chat_rail_folders');
  static Key folderKey(String folderId) => Key('chat_rail_folder_$folderId');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final foldersAsync = ref.watch(chatFoldersProvider);
    final selectedId = ref.watch(selectedChatFolderIdProvider);
    final voice = VoiceColors.of(context);

    return foldersAsync.when(
      data: (data) {
        if (data.folders.isEmpty) return const SizedBox.shrink();
        return Column(
          key: sectionKey,
          mainAxisSize: MainAxisSize.min,
          children: [
            const Divider(height: 1),
            for (final folder in data.folders)
              _RailFolderButton(
                key: folderKey(folder.id),
                folder: folder,
                selected: selectedId == folder.id,
                onPressed: () => _selectFolder(ref, folder.id),
              ),
            const Divider(height: 1),
          ],
        );
      },
      loading: () => Padding(
        padding: const EdgeInsets.symmetric(vertical: 8),
        child: SizedBox(
          width: 20,
          height: 20,
          child: CircularProgressIndicator(
            strokeWidth: 2,
            color: voice.textSecondary,
          ),
        ),
      ),
      error: (_, _) => const SizedBox.shrink(),
    );
  }

  void _selectFolder(WidgetRef ref, String folderId) {
    final current = ref.read(selectedChatFolderIdProvider);
    ref.read(selectedChatFolderIdProvider.notifier).state =
        current == folderId ? null : folderId;
    ref.read(chatListControllerProvider.notifier).loadInitial();
  }
}

class _RailFolderButton extends StatelessWidget {
  const _RailFolderButton({
    super.key,
    required this.folder,
    required this.selected,
    required this.onPressed,
  });

  final VoiceFolder folder;
  final bool selected;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    return Tooltip(
      message: folder.name,
      child: Material(
        color: selected ? voice.elevated : Colors.transparent,
        child: InkWell(
          onTap: onPressed,
          child: SizedBox(
            width: double.infinity,
            height: 40,
            child: Icon(
              _iconForFolder(folder),
              size: 20,
              color: selected ? voice.profileAccent : voice.textSecondary,
            ),
          ),
        ),
      ),
    );
  }

  IconData _iconForFolder(VoiceFolder folder) {
    final name = folder.name.toLowerCase();
    if (name.contains('request')) return Icons.mark_email_unread_outlined;
    if (name.contains('direct') || name == 'dm' || name.contains('лич')) {
      return Icons.person_outline;
    }
    if (name.contains('group')) return Icons.groups_outlined;
    if (name.contains('channel')) return Icons.campaign_outlined;
    if (name.contains('space')) return Icons.hub_outlined;
    if (name.contains('all') || name.contains('все')) {
      return Icons.inbox_outlined;
    }
    return Icons.folder_outlined;
  }
}

/// Quick Access shortcuts in rail (§1.1c) — up to 15 chat_id entries.
class ChatRailQuickAccessSection extends ConsumerWidget {
  const ChatRailQuickAccessSection({super.key});

  static const sectionKey = Key('chat_rail_quick_access');
  static Key itemKey(String chatId) => Key('chat_rail_qa_$chatId');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final qaAsync = ref.watch(quickAccessListProvider);
    final voice = VoiceColors.of(context);
    final shellNav = ref.read(shellNavigationProvider);

    return qaAsync.when(
      data: (data) {
        if (data.items.isEmpty) return const SizedBox.shrink();
        return Column(
          key: sectionKey,
          mainAxisSize: MainAxisSize.min,
          children: [
            Tooltip(
              message: l10n.chatQuickAccessTitle,
              child: Padding(
                padding: const EdgeInsets.only(top: 4, bottom: 2),
                child: Icon(Icons.star_outline, size: 14, color: voice.textSecondary),
              ),
            ),
            for (final item in data.items)
              _QuickAccessRailButton(
                key: itemKey(item.chatId),
                item: item,
                onTap: () => shellNav.selectChatFromHome(item.chatId),
                onLongPress: () => _removeQuickAccess(context, ref, item.chatId),
              ),
          ],
        );
      },
      loading: () => const SizedBox.shrink(),
      error: (_, _) => const SizedBox.shrink(),
    );
  }

  Future<void> _removeQuickAccess(
    BuildContext context,
    WidgetRef ref,
    String chatId,
  ) async {
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    final result = await ref.read(voiceChatsClientProvider).removeQuickAccess(
          authorization: auth,
          chatId: chatId,
        );
    if (!context.mounted) return;
    switch (result) {
      case ChatsApiOk<void>():
        invalidateChatNavigationData(ref);
      case ChatsApiFailure(:final message):
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
    }
  }
}

class _QuickAccessRailButton extends StatelessWidget {
  const _QuickAccessRailButton({
    super.key,
    required this.item,
    required this.onTap,
    required this.onLongPress,
  });

  final VoiceQuickAccessItem item;
  final VoidCallback onTap;
  final VoidCallback onLongPress;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final chat = item.chat;
    final label = chat?.name ?? item.chatId;
    return Tooltip(
      message: label,
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onTap,
          onLongPress: onLongPress,
          child: Padding(
            padding: const EdgeInsets.symmetric(vertical: 2),
            child: CircleAvatar(
              radius: 14,
              backgroundColor: voice.elevated,
              backgroundImage:
                  chat?.avatarUrl != null ? NetworkImage(chat!.avatarUrl!) : null,
              child: chat?.avatarUrl == null
                  ? Icon(Icons.star, size: 14, color: voice.profileAccent)
                  : null,
            ),
          ),
        ),
      ),
    );
  }
}
