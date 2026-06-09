import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/chats_client.dart';
import '../../backend/messages_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/social_providers.dart';
import '../api_error_messages.dart';
import '../core/voice_avatar.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_list_row.dart';
import '../core/voice_state_panel.dart';

/// Picks a target chat and optionally adds commentary before forwarding.
class ForwardMessageSheet extends ConsumerStatefulWidget {
  const ForwardMessageSheet({
    super.key,
    required this.sourceMessage,
    required this.sourceChatId,
  });

  static const Key sheetKey = Key('forward_message_sheet');
  static const Key searchFieldKey = Key('forward_message_search');

  static Key chatTileKey(String chatId) => Key('forward_chat_$chatId');

  final VoiceMessage sourceMessage;
  final String sourceChatId;

  static Future<void> show(
    BuildContext context, {
    required VoiceMessage sourceMessage,
    required String sourceChatId,
  }) {
    final container = ProviderScope.containerOf(context);
    return showVoiceBottomSheet<void>(
      context: context,
      scrollable: false,
      child: UncontrolledProviderScope(
        container: container,
        child: ForwardMessageSheet(
          sourceMessage: sourceMessage,
          sourceChatId: sourceChatId,
        ),
      ),
    );
  }

  @override
  ConsumerState<ForwardMessageSheet> createState() =>
      _ForwardMessageSheetState();
}

class _ForwardMessageSheetState extends ConsumerState<ForwardMessageSheet> {
  final _searchController = TextEditingController();
  var _forwarding = false;

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  String _shortChatId(String id) {
    if (id.length <= 8) return id;
    return '${id.substring(0, 8)}…';
  }

  String _chatTitleFallback(AppLocalizations l10n, ChatListItem item) {
    return item.chat.name ?? l10n.chatListDmFallback(_shortChatId(item.chatId));
  }

  List<ChatListItem> _filteredChats(
    List<ChatListItem> items,
    AppLocalizations l10n,
  ) {
    final query = _searchController.text.trim().toLowerCase();
    final eligible = items
        .where((item) => item.chatId != widget.sourceChatId)
        .toList(growable: false);
    if (query.isEmpty) return eligible;
    return eligible
        .where((item) {
          final title = _chatTitleFallback(l10n, item);
          return title.toLowerCase().contains(query);
        })
        .toList(growable: false);
  }

  Future<void> _onChatSelected(ChatListItem item) async {
    if (_forwarding) return;
    final commentary = await _promptCommentary();
    if (!mounted || commentary == null) return;

    setState(() => _forwarding = true);
    final l10n = AppLocalizations.of(context)!;
    final err = await ref.read(chatActionsProvider).forwardMessage(
      sourceMessageId: widget.sourceMessage.id,
      targetChatId: item.chatId,
      commentary: commentary.isEmpty ? null : commentary,
    );
    if (!mounted) return;
    setState(() => _forwarding = false);
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(chatActionErrorMessage(l10n, err))),
      );
      return;
    }
    if (!mounted) return;
    Navigator.of(context).pop();
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(l10n.chatForwardSuccess)),
    );
    ref.read(chatActionsProvider).selectChat(item.chatId);
  }

  Future<String?> _promptCommentary() {
    return showDialog<String?>(
      context: context,
      builder: (context) => const _ForwardCommentaryDialog(),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final listState = ref.watch(chatListControllerProvider);
    final activeProfileId = ref.watch(authControllerProvider).activeProfileId;
    final peerMap = ref.watch(dmPeerProfileByChatIdProvider);
    final chats = _filteredChats(listState.items, l10n);

    return SafeArea(
      key: ForwardMessageSheet.sheetKey,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(l10n.chatForwardTitle, style: theme.textTheme.titleLarge),
            const SizedBox(height: 12),
            TextField(
              key: ForwardMessageSheet.searchFieldKey,
              controller: _searchController,
              decoration: InputDecoration(
                hintText: l10n.chatForwardSearchHint,
                prefixIcon: const Icon(Icons.search),
              ),
              onChanged: (_) => setState(() {}),
            ),
            const SizedBox(height: 8),
            if (_forwarding)
              const LinearProgressIndicator(minHeight: 2),
            Expanded(
              child: listState.isLoading && listState.items.isEmpty
                  ? Center(child: Text(l10n.commonLoading))
                  : chats.isEmpty
                  ? VoiceStatePanel(
                      title: l10n.chatForwardEmpty,
                      icon: Icons.chat_bubble_outline,
                    )
                  : ListView.builder(
                      itemCount: chats.length,
                      itemBuilder: (context, index) {
                        final item = chats[index];
                        final peerId = resolveDmPeerProfileId(
                          item: item,
                          knownPeerId: peerMap[item.chatId],
                          activeProfileId: activeProfileId,
                        );
                        final profileName = peerId != null
                            ? ref
                                  .watch(profileProvider(peerId))
                                  .valueOrNull
                                  ?.displayName
                            : null;
                        final title =
                            (profileName != null && profileName.isNotEmpty)
                            ? profileName
                            : _chatTitleFallback(l10n, item);
                        return VoiceListRow(
                          key: ForwardMessageSheet.chatTileKey(item.chatId),
                          title: title,
                          subtitle: item.lastMessagePreview,
                          leading: item.chat.isGroup
                              ? VoiceAvatar(
                                  imageUrl: item.chat.avatarUrl,
                                  label: title,
                                )
                              : null,
                          onTap: _forwarding
                              ? null
                              : () => _onChatSelected(item),
                        );
                      },
                    ),
            ),
          ],
        ),
      ),
    );
  }
}

class _ForwardCommentaryDialog extends StatefulWidget {
  const _ForwardCommentaryDialog();

  @override
  State<_ForwardCommentaryDialog> createState() =>
      _ForwardCommentaryDialogState();
}

class _ForwardCommentaryDialogState extends State<_ForwardCommentaryDialog> {
  final _controller = TextEditingController();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return AlertDialog(
      title: Text(l10n.chatForwardCommentaryTitle),
      content: TextField(
        controller: _controller,
        autofocus: true,
        decoration: InputDecoration(hintText: l10n.chatForwardCommentaryHint),
        maxLines: 3,
        textCapitalization: TextCapitalization.sentences,
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l10n.commonCancel),
        ),
        FilledButton(
          onPressed: () => Navigator.of(context).pop(_controller.text),
          child: Text(l10n.chatMessageForward),
        ),
      ],
    );
  }
}
