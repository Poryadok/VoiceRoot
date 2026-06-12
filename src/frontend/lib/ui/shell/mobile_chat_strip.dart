import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/chats_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/shell_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_avatar.dart';
import '../core/voice_badge.dart';

/// Horizontal mini-icon strip with unread badges when a chat is open on mobile
/// ([docs/features/navigation.md]).
class MobileChatStrip extends ConsumerWidget {
  const MobileChatStrip({super.key});

  static const stripKey = Key('mobile_chat_strip');
  static Key tileKey(String chatId) => Key('mobile_chat_strip_tile_$chatId');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final chats = ref.watch(chatListControllerProvider);
    final inbox = ref.watch(chatInboxProvider);
    final selectedId = ref.watch(selectedChatIdProvider);
    final activeProfileId = ref.watch(authControllerProvider).activeProfileId;
    final shellNav = ref.read(shellNavigationProvider);

    final items = chats.items
        .where((item) => (item.inbox ?? 'main') == inbox)
        .take(30)
        .toList();

    if (items.isEmpty) {
      return ColoredBox(
        key: stripKey,
        color: voice.muted,
        child: const SizedBox.expand(),
      );
    }

    return Material(
      key: stripKey,
      color: voice.muted,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: 4),
        itemCount: items.length,
        separatorBuilder: (_, _) => const SizedBox(width: 2),
        itemBuilder: (context, index) {
          final item = items[index];
          final title = _stripTitle(l10n, item);
          final selected = item.chatId == selectedId;
          return Semantics(
            button: true,
            selected: selected,
            label: item.unreadCount > 0
                ? '$title, ${l10n.chatListUnreadCount(item.unreadCount)}'
                : title,
            child: _StripChatIcon(
              key: tileKey(item.chatId),
              item: item,
              title: title,
              selected: selected,
              onTap: () => shellNav.selectChatFromHome(item.chatId),
              activeProfileId: activeProfileId,
            ),
          );
        },
      ),
    );
  }

  static String _stripTitle(AppLocalizations l10n, ChatListItem item) {
    return item.chat.name ?? l10n.chatListDmFallback(_shortChatId(item.chatId));
  }

  static String _shortChatId(String chatId) {
    if (chatId.length <= 8) return chatId;
    return chatId.substring(0, 8);
  }
}

class _StripChatIcon extends StatelessWidget {
  const _StripChatIcon({
    super.key,
    required this.item,
    required this.title,
    required this.selected,
    required this.onTap,
    required this.activeProfileId,
  });

  final ChatListItem item;
  final String title;
  final bool selected;
  final VoidCallback onTap;
  final String? activeProfileId;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: InkWell(
        onTap: onTap,
        customBorder: const CircleBorder(),
        child: SizedBox(
          width: 44,
          height: 40,
          child: Stack(
            clipBehavior: Clip.none,
            alignment: Alignment.center,
            children: [
              DecoratedBox(
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  border: selected
                      ? Border.all(color: voice.profileAccent, width: 2)
                      : null,
                ),
                child: VoiceAvatar(
                  imageUrl: item.chat.avatarUrl,
                  label: title,
                  radius: 16,
                ),
              ),
              if (item.unreadCount > 0)
                Positioned(
                  top: -2,
                  right: -2,
                  child: VoiceBadge(
                    count: item.unreadCount,
                    semanticLabel: AppLocalizations.of(
                      context,
                    )!.chatListUnreadCount(item.unreadCount),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}
