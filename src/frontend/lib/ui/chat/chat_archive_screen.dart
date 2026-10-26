import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../backend/api_errors.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/inbox_reconciler.dart';
import '../../state/shell_providers.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';
import '../api_error_messages.dart';
import '../core/chat_author_label.dart';
import '../core/voice_avatar.dart';
import '../core/voice_badge.dart';
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

    final list = Builder(
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
                item.dmPeerDisplayName ??
                item.chat.name ??
                l10n.chatListDmFallback(_shortChatId(item.chatId));
            final row = VoiceListRow(
              key: ChatArchiveScreen.tileKey(item.chatId),
              title: title,
              leading: VoiceAvatar(
                imageUrl: profile?.avatarUrl ?? item.chat.avatarUrl,
                label: title,
                radius: 20,
              ),
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
                  : Text(
                      title,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
              subtitle: item.lastMessagePreview ?? '',
              trailing: item.unreadCount > 0
                  ? VoiceBadge(
                      count: item.unreadCount,
                      semanticLabel: l10n.chatListUnreadCount(item.unreadCount),
                    )
                  : null,
              onTap: () {
                ref
                    .read(shellNavigationProvider)
                    .selectChatFromHome(item.chatId);
                _returnToChats(context);
              },
            );
            final accessibleRow = Semantics(
              customSemanticsActions: {
                CustomSemanticsAction(label: l10n.chatArchiveUnarchive): () =>
                    _unarchive(context, item.chatId),
              },
              child: Focus(
                skipTraversal: true,
                onKeyEvent: (node, event) {
                  if (event is KeyDownEvent &&
                      (event.logicalKey == LogicalKeyboardKey.contextMenu ||
                          (event.logicalKey == LogicalKeyboardKey.f10 &&
                              HardwareKeyboard.instance.isShiftPressed))) {
                    final box = node.context?.findRenderObject();
                    _showActions(
                      context,
                      item.chatId,
                      position: box is RenderBox
                          ? box.localToGlobal(Offset(16, box.size.height))
                          : null,
                    );
                    return KeyEventResult.handled;
                  }
                  return KeyEventResult.ignored;
                },
                child: GestureDetector(
                  onLongPressStart: (details) => _showActions(
                    context,
                    item.chatId,
                    position: details.globalPosition,
                  ),
                  onSecondaryTapUp: (details) => _showActions(
                    context,
                    item.chatId,
                    position: details.globalPosition,
                  ),
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(minHeight: 64),
                    child: Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 4),
                      child: row,
                    ),
                  ),
                ),
              ),
            );
            final platform = Theme.of(context).platform;
            if (platform != TargetPlatform.android &&
                platform != TargetPlatform.iOS &&
                MediaQuery.sizeOf(context).width >= 600) {
              return accessibleRow;
            }
            return Dismissible(
              key: Key('chat_archive_swipe_${item.chatId}'),
              direction: DismissDirection.endToStart,
              background: Container(
                color: Theme.of(context).colorScheme.primaryContainer,
                alignment: Alignment.centerRight,
                padding: const EdgeInsets.symmetric(horizontal: 24),
                child: Text(
                  l10n.chatArchiveUnarchive,
                  style: TextStyle(
                    color: Theme.of(context).colorScheme.onPrimaryContainer,
                  ),
                ),
              ),
              confirmDismiss: (_) async {
                await _unarchive(context, item.chatId);
                // The archive state removes the row only after the API succeeds.
                return false;
              },
              child: accessibleRow,
            );
          },
        );
      },
    );
    return Scaffold(
      key: ChatArchiveScreen.screenKey,
      body: SafeArea(
        child: LayoutBuilder(
          builder: (context, constraints) {
            final wide = constraints.maxWidth >= 600;
            final pane = SizedBox(
              key: const Key('chat_archive_list_pane'),
              width: wide ? 320 : constraints.maxWidth,
              child: Column(
                children: [
                  AppBar(
                    automaticallyImplyLeading: false,
                    leading: BackButton(
                      onPressed: () => _returnToChats(context),
                    ),
                    backgroundColor: VoiceColors.of(context).canvas,
                    titleSpacing: 0,
                    title: Text(l10n.chatArchiveTitle),
                    toolbarHeight: 56,
                  ),
                  Divider(
                    height: 1,
                    color: VoiceColors.of(context).borderDefault,
                  ),
                  Expanded(child: list),
                ],
              ),
            );
            return Row(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                pane,
                if (wide) ...[
                  VerticalDivider(
                    width: 1,
                    color: VoiceColors.of(context).borderDefault,
                  ),
                  Expanded(
                    child: ColoredBox(
                      key: const Key('chat_archive_detail_placeholder'),
                      color: VoiceColors.of(context).canvas,
                      child: Center(
                        child: Padding(
                          padding: const EdgeInsets.all(24),
                          child: Column(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Container(
                                width: 64,
                                height: 64,
                                decoration: BoxDecoration(
                                  color: VoiceColors.of(context).elevated,
                                  borderRadius: BorderRadius.circular(20),
                                ),
                                child: const Icon(
                                  Icons.unarchive_outlined,
                                  size: 25,
                                ),
                              ),
                              const SizedBox(height: 8),
                              Text(
                                l10n.chatArchivePlaceholderTitle,
                                textAlign: TextAlign.center,
                                style: Theme.of(context).textTheme.bodyMedium
                                    ?.copyWith(fontWeight: FontWeight.w600),
                              ),
                              const SizedBox(height: 8),
                              Text(
                                l10n.chatArchivePlaceholderHint,
                                textAlign: TextAlign.center,
                                style: Theme.of(context).textTheme.bodySmall
                                    ?.copyWith(
                                      color: VoiceColors.of(
                                        context,
                                      ).textSecondary,
                                    ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    ),
                  ),
                ],
              ],
            );
          },
        ),
      ),
    );
  }

  void _returnToChats(BuildContext context) {
    if (Navigator.of(context).canPop()) {
      Navigator.of(context).pop();
    } else {
      GoRouter.maybeOf(context)?.go('/');
    }
  }

  Future<void> _showActions(
    BuildContext context,
    String chatId, {
    Offset? position,
  }) async {
    final l10n = AppLocalizations.of(context)!;
    final session = ref.read(authControllerProvider).session;
    final overlay =
        Overlay.of(context).context.findRenderObject()! as RenderBox;
    final point = position ?? overlay.localToGlobal(const Offset(24, 100));
    final action = await showMenu<String>(
      context: context,
      color: VoiceColors.of(context).canvas,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(8),
        side: BorderSide(color: VoiceColors.of(context).borderDefault),
      ),
      menuPadding: const EdgeInsets.all(6),
      position: RelativeRect.fromRect(
        Rect.fromLTWH(point.dx, point.dy, 0, 0),
        Offset.zero & overlay.size,
      ),
      items: [
        PopupMenuItem(
          value: 'unarchive',
          height: 40,
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(Icons.unarchive_outlined, size: 20),
              const SizedBox(width: 10),
              Text(l10n.chatArchiveUnarchive),
            ],
          ),
        ),
        PopupMenuItem(
          value: 'mark_read',
          height: 40,
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(Icons.done_all, size: 20),
              const SizedBox(width: 10),
              Text(l10n.chatArchiveMarkRead),
            ],
          ),
        ),
      ],
    );
    if (action == 'unarchive' &&
        context.mounted &&
        ref.read(authControllerProvider).session == session) {
      await _unarchive(context, chatId);
    }
    if (action == 'mark_read' &&
        context.mounted &&
        ref.read(authControllerProvider).session == session) {
      await _markRead(context, chatId);
    }
  }

  Future<void> _markRead(BuildContext context, String chatId) async {
    final error = await ref
        .read(chatArchiveListControllerProvider.notifier)
        .markChatRead(chatId);
    if (!context.mounted || error == null || error == kChatActionStaleContext) {
      return;
    }
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(commonActionErrorMessage(AppLocalizations.of(context)!)),
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
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(commonActionErrorMessage(l10n))));
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
