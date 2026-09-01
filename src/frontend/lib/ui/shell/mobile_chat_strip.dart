import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/chats_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/mobile_opened_chat_strip.dart';
import '../../state/shell_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_layout.dart';
import '../core/voice_avatar.dart';
import '../core/voice_badge.dart';

/// Horizontal mini-icon strip of **opened** chats (LRU) when a chat is open on
/// mobile ([docs/features/navigation.md] § Active strip).
class MobileChatStrip extends ConsumerStatefulWidget {
  const MobileChatStrip({super.key});

  static const stripKey = Key('mobile_chat_strip');
  static Key tileKey(String chatId) => Key('mobile_chat_strip_tile_$chatId');
  static Key removeOverlayKey(String chatId) =>
      Key('mobile_chat_strip_remove_$chatId');

  @override
  ConsumerState<MobileChatStrip> createState() => _MobileChatStripState();
}

class _MobileChatStripState extends ConsumerState<MobileChatStrip> {
  String? _removeOverlayChatId;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final openedIds = ref.watch(mobileOpenedChatStripProvider);
    final chats = ref.watch(chatListControllerProvider);
    final selectedId = ref.watch(selectedChatIdProvider);
    final activeProfileId = ref.watch(authControllerProvider).activeProfileId;
    final shellNav = ref.read(shellNavigationProvider);

    ref.listen(mobileStripEvictionNoticeProvider, (prev, next) {
      if (next == (prev ?? 0)) return;
      final messenger = ScaffoldMessenger.maybeOf(context);
      if (messenger == null) return;
      messenger.showSnackBar(
        SnackBar(content: Text(l10n.mobileStripLimitReached)),
      );
    });

    final byId = {for (final item in chats.items) item.chatId: item};
    final items = openedIds
        .map((id) => byId[id])
        .whereType<ChatListItem>()
        .take(kMobileOpenedChatStripVisibleCap)
        .toList();

    if (items.isEmpty) {
      return ColoredBox(
        key: MobileChatStrip.stripKey,
        color: voice.muted,
        child: const SizedBox.expand(),
      );
    }

    return Material(
      key: MobileChatStrip.stripKey,
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
          final showRemove = _removeOverlayChatId == item.chatId;
          return Semantics(
            button: true,
            selected: selected,
            label: item.unreadCount > 0
                ? '$title, ${l10n.chatListUnreadCount(item.unreadCount)}'
                : title,
            child: _StripChatIcon(
              key: MobileChatStrip.tileKey(item.chatId),
              item: item,
              title: title,
              selected: selected,
              showRemoveOverlay: showRemove,
              onTap: () {
                if (showRemove) {
                  setState(() => _removeOverlayChatId = null);
                  return;
                }
                shellNav.selectStripChat(
                  item.chatId,
                  inSpace: item.chat.spaceId != null &&
                      item.chat.spaceId!.isNotEmpty,
                );
              },
              onLongPress: () =>
                  setState(() => _removeOverlayChatId = item.chatId),
              onRemove: () {
                ref
                    .read(mobileOpenedChatStripProvider.notifier)
                    .removeChat(item.chatId);
                setState(() => _removeOverlayChatId = null);
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text(l10n.mobileStripRemoved)),
                );
              },
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
    required this.onLongPress,
    required this.onRemove,
    required this.activeProfileId,
    this.showRemoveOverlay = false,
  });

  final ChatListItem item;
  final String title;
  final bool selected;
  final bool showRemoveOverlay;
  final VoidCallback onTap;
  final VoidCallback onLongPress;
  final VoidCallback onRemove;
  final String? activeProfileId;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final touchSize = VoiceLayout.minTouchTarget;
    final l10n = AppLocalizations.of(context)!;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: InkWell(
        onTap: onTap,
        onLongPress: onLongPress,
        customBorder: const CircleBorder(),
        child: SizedBox(
          width: touchSize,
          height: touchSize,
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
              if (item.unreadCount > 0 && !showRemoveOverlay)
                Positioned(
                  top: -2,
                  right: -2,
                  child: VoiceBadge(
                    count: item.unreadCount,
                    semanticLabel: l10n.chatListUnreadCount(item.unreadCount),
                  ),
                ),
              if (showRemoveOverlay)
                Positioned.fill(
                  child: Material(
                    key: MobileChatStrip.removeOverlayKey(item.chatId),
                    color: Colors.black54,
                    shape: const CircleBorder(),
                    child: IconButton(
                      tooltip: l10n.mobileStripRemoveFromStrip,
                      onPressed: onRemove,
                      icon: const Icon(Icons.close, color: Colors.white, size: 20),
                    ),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}
