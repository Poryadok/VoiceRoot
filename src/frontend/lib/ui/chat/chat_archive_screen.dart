import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/api_errors.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/inbox_reconciler.dart';
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
      final profileId = ref.read(authControllerProvider).activeProfileId;
      final reconcilerScope = profileId == null
          ? null
          : ref
                .read(inboxReconcilerProvider)
                .profileSnapshots[profileId]
                ?.scopes[InboxScope.archive];
      if (reconcilerScope != null) return;
      ref.read(chatArchiveListControllerProvider.notifier).loadInitial();
    });
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final archive = ref.watch(chatArchiveListControllerProvider);
    final activeProfileId = ref.watch(authControllerProvider).activeProfileId;
    final reconciler = ref.watch(inboxReconcilerProvider);
    final reconcilerScope = activeProfileId != null
        ? reconciler.profileSnapshots[activeProfileId]?.scopes[InboxScope
              .archive]
        : null;

    return Scaffold(
      key: ChatArchiveScreen.screenKey,
      appBar: AppBar(title: Text(l10n.chatArchiveTitle)),
      body: Builder(
        builder: (context) {
          final items = reconcilerScope == null
              ? activeProfileId != null && archive.profileId == activeProfileId
                    ? archive.items
                    : const []
              : reconcilerScope.isComplete
              ? reconcilerScope.items
              : archive.profileId == activeProfileId
              ? mergeInboxRows(archive.items, reconcilerScope.items)
              : reconcilerScope.items;
          final legacyMatchesProfile =
              activeProfileId != null && archive.profileId == activeProfileId;
          final isLoading =
              reconcilerScope?.isLoading ??
              (legacyMatchesProfile
                  ? archive.isLoading
                  : activeProfileId != null);
          final errorMessage =
              reconcilerScope?.errorMessage ??
              (legacyMatchesProfile ? archive.errorMessage : null);
          final errorStatusCode =
              reconcilerScope?.errorStatusCode ??
              (legacyMatchesProfile ? archive.errorStatusCode : null);
          final hasReconcilerError = reconcilerScope?.errorMessage != null;
          if (isLoading && items.isEmpty) {
            return const VoiceListSkeleton();
          }
          if (errorMessage != null && items.isEmpty) {
            return VoiceStatePanel(
              title: l10n.chatArchiveLoadError,
              message: chatListErrorMessage(
                l10n,
                isBackendUnavailable(errorStatusCode)
                    ? const BackendUnavailableException()
                    : Exception(errorMessage),
              ),
              icon: Icons.cloud_off_outlined,
              actionLabel: l10n.commonRetry,
              onAction: reconcilerScope != null
                  ? () => ref
                        .read(inboxReconcilerProvider.notifier)
                        .retry(InboxScope.archive)
                  : () => ref
                        .read(chatArchiveListControllerProvider.notifier)
                        .loadInitial(),
            );
          }
          if (items.isEmpty) {
            return VoiceStatePanel(
              title: l10n.chatArchiveEmpty,
              message: l10n.chatArchiveEmptyHint,
              icon: Icons.archive_outlined,
            );
          }
          final hasFooter = reconcilerScope != null
              ? isLoading ||
                    reconcilerScope.nextCursor != null ||
                    hasReconcilerError
              : archive.hasMore || archive.isLoadingMore;
          return ListView.builder(
            itemCount: items.length + (hasFooter ? 1 : 0),
            itemBuilder: (context, index) {
              if (index == items.length) {
                if (hasReconcilerError) {
                  return VoiceStatePanel(
                    title: l10n.chatArchiveLoadError,
                    message: chatListErrorMessage(
                      l10n,
                      isBackendUnavailable(errorStatusCode)
                          ? const BackendUnavailableException()
                          : Exception(errorMessage),
                    ),
                    icon: Icons.cloud_off_outlined,
                    actionLabel: l10n.commonRetry,
                    onAction: () => ref
                        .read(inboxReconcilerProvider.notifier)
                        .retry(InboxScope.archive),
                  );
                }
                return Center(
                  child: reconcilerScope != null || archive.isLoadingMore
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
              final profileAsync = peerId != null
                  ? ref.watch(profileProvider(peerId))
                  : null;
              final profile = profileAsync?.valueOrNull;
              final title =
                  profile?.displayName ??
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
                  ref
                      .read(shellNavigationProvider)
                      .selectChatFromHome(item.chatId);
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
    final session = ref.read(authControllerProvider).session;
    if (session == null) return;
    final err = await ref
        .read(chatArchiveListControllerProvider.notifier)
        .unarchiveChat(chatId);
    if (!context.mounted) return;
    if (err == kChatActionStaleContext) return;
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(err)));
      return;
    }
    ref
        .read(inboxReconcilerProvider.notifier)
        .removeChat(
          InboxScope.archive,
          chatId,
          expectedProfileId: session.activeProfileId,
          expectedAuthorization: session.authorizationHeader,
        );
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(l10n.chatArchiveUnarchived)));
  }

  String _shortChatId(String chatId) {
    if (chatId.length <= 8) return chatId;
    return chatId.substring(0, 8);
  }
}
