import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/shell_providers.dart';
import '../../state/social_providers.dart';
import '../api_error_messages.dart';
import '../core/chat_author_label.dart';
import '../core/voice_list_row.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';

/// Archived chats list per navigation.md §1.10 / Screen/Chat/Archive.
class ChatArchiveScreen extends ConsumerStatefulWidget {
  const ChatArchiveScreen({super.key});

  static const Key screenKey = Key('chat_archive_screen');
  static Key tileKey(String chatId) => Key('chat_archive_tile_$chatId');
  static const Key loadMoreKey = Key('chat_archive_load_more');

  @override
  ConsumerState<ChatArchiveScreen> createState() => _ChatArchiveScreenState();
}

class _ChatArchiveScreenState extends ConsumerState<ChatArchiveScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(chatArchiveListControllerProvider.notifier).loadInitial();
    });
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final archive = ref.watch(chatArchiveListControllerProvider);
    final activeProfileId = ref.watch(authControllerProvider).activeProfileId;

    return Scaffold(
      key: ChatArchiveScreen.screenKey,
      appBar: AppBar(title: Text(l10n.chatArchiveTitle)),
      body: Builder(
        builder: (context) {
          final items = archive.items;
          if (archive.isLoading && items.isEmpty) {
            return const VoiceListSkeleton();
          }
          if (archive.errorMessage != null && items.isEmpty) {
            return VoiceStatePanel(
              title: l10n.chatArchiveLoadError,
              message: chatListErrorMessage(
                l10n,
                Exception(archive.errorMessage),
              ),
              icon: Icons.cloud_off_outlined,
              actionLabel: l10n.commonRetry,
              onAction: () =>
                  ref.read(chatArchiveListControllerProvider.notifier).loadInitial(),
            );
          }
          if (items.isEmpty) {
            return VoiceStatePanel(
              title: l10n.chatArchiveEmpty,
              message: l10n.chatArchiveEmptyHint,
              icon: Icons.archive_outlined,
            );
          }
          final hasFooter = archive.hasMore || archive.isLoadingMore;
          return ListView.builder(
            itemCount: items.length + (hasFooter ? 1 : 0),
            itemBuilder: (context, index) {
              if (index == items.length) {
                return Center(
                  child: archive.isLoadingMore
                      ? const Padding(
                          padding: EdgeInsets.all(8),
                          child: SizedBox(
                            width: 24,
                            height: 24,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          ),
                        )
                      : TextButton.icon(
                          key: ChatArchiveScreen.loadMoreKey,
                          icon: const Icon(Icons.expand_more),
                          label: Text(l10n.chatListLoadMore),
                          onPressed: () => ref
                              .read(chatArchiveListControllerProvider.notifier)
                              .loadMore(),
                        ),
                );
              }
              final item = items[index];
              final peerId = resolveDmPeerProfileId(
                item: item,
                knownPeerId: null,
                activeProfileId: activeProfileId,
              );
              final profileAsync =
                  peerId != null ? ref.watch(profileProvider(peerId)) : null;
              final profile = profileAsync?.valueOrNull;
              final title = profile?.displayName ??
                  item.chat.name ??
                  l10n.chatListDmFallback(_shortChatId(item.chatId));
              return VoiceListRow(
                key: ChatArchiveScreen.tileKey(item.chatId),
                title: title,
                titleWidget: peerId != null && !item.chat.isGroup
                    ? ChatAuthorLabel(
                        displayName: title,
                        isPremium: false,
                        verificationType: profile?.verificationType ?? 'none',
                        premiumBadgeSemanticLabel: l10n.premiumBadgeLabel,
                        verifiedBadgeSemanticLabel:
                            profile?.verificationType == 'organization'
                                ? l10n.verifiedBadgeOrganization
                                : l10n.verifiedBadgePersonal,
                      )
                    : null,
                subtitle: item.lastMessagePreview ?? '',
                trailing: TextButton(
                  onPressed: () => _unarchive(context, item.chatId),
                  child: Text(l10n.chatArchiveUnarchive),
                ),
                onTap: () {
                  ref.read(shellNavigationProvider).selectChatFromHome(item.chatId);
                  Navigator.of(context).pop();
                },
                onLongPress: () => _unarchive(context, item.chatId),
              );
            },
          );
        },
      ),
    );
  }

  Future<void> _unarchive(BuildContext context, String chatId) async {
    final l10n = AppLocalizations.of(context)!;
    final err = await ref
        .read(chatArchiveListControllerProvider.notifier)
        .unarchiveChat(chatId);
    if (!context.mounted) return;
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(err)),
      );
      return;
    }
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(l10n.chatArchiveUnarchived)),
    );
  }

  String _shortChatId(String chatId) {
    if (chatId.length <= 8) return chatId;
    return chatId.substring(0, 8);
  }
}
