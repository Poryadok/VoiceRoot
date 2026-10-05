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
import 'forward_message_contacts_provider.dart';

/// Picks a target chat and optionally adds commentary before forwarding.
/// When [withoutAttribution] is true (FW-03), copies as a regular message.
/// [sourceMessages] is the FW-05 multi-select batch; [sourceMessage] is the
/// single-message path.
class ForwardMessageSheet extends ConsumerStatefulWidget {
  const ForwardMessageSheet({
    super.key,
    this.sourceMessage,
    this.sourceMessages = const [],
    required this.sourceChatId,
    this.withoutAttribution = false,
  });

  static const Key sheetKey = Key('forward_message_sheet');
  static const Key searchFieldKey = Key('forward_message_search');
  static const Key copyAsNewSheetKey = Key('copy_as_new_message_sheet');
  static const Key closeButtonKey = Key('forward_message_close');
  static const Key cancelButtonKey = Key('forward_message_cancel');
  static const Key submitButtonKey = Key('forward_message_submit');
  static const Key commentFieldKey = Key('forward_message_comment');

  static Key chatTileKey(String chatId) => Key('forward_chat_$chatId');
  static Key contactTileKey(String profileId) =>
      Key('forward_contact_$profileId');

  final VoiceMessage? sourceMessage;
  final List<VoiceMessage> sourceMessages;
  final String sourceChatId;
  final bool withoutAttribution;

  List<VoiceMessage> get messagesToForward {
    if (sourceMessages.isNotEmpty) return sourceMessages;
    final single = sourceMessage;
    return single == null ? const [] : [single];
  }

  static Future<void> show(
    BuildContext context, {
    VoiceMessage? sourceMessage,
    List<VoiceMessage>? sourceMessages,
    required String sourceChatId,
    bool withoutAttribution = false,
  }) {
    final container = ProviderScope.containerOf(context);
    return showVoiceBottomSheet<void>(
      context: context,
      scrollable: false,
      child: UncontrolledProviderScope(
        container: container,
        child: ForwardMessageSheet(
          sourceMessage: sourceMessage,
          sourceMessages: sourceMessages ?? const [],
          sourceChatId: sourceChatId,
          withoutAttribution: withoutAttribution,
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
  final _commentController = TextEditingController();
  var _forwarding = false;
  String? _selectedChatId;
  String? _selectedFriendProfileId;
  String? _selectedTargetOwnerProfileId;

  @override
  void dispose() {
    _searchController.dispose();
    _commentController.dispose();
    super.dispose();
  }

  String _shortChatId(String id) {
    if (id.length <= 8) return id;
    return '${id.substring(0, 8)}…';
  }

  String _chatTitleFallback(AppLocalizations l10n, ChatListItem item) {
    return item.dmPeerDisplayName ??
        item.chat.name ??
        l10n.chatListDmFallback(_shortChatId(item.chatId));
  }

  List<ChatListItem> _filteredChats(
    List<ChatListItem> items,
    AppLocalizations l10n,
    Map<String, String> peerMap,
    String? activeProfileId,
  ) {
    final query = _searchController.text.trim().toLowerCase();
    final eligible = items
        .where((item) => item.chatId != widget.sourceChatId)
        .toList(growable: false);
    if (query.isEmpty) return eligible;
    return eligible
        .where((item) {
          final title = _chatTitleFallback(l10n, item);
          if (title.toLowerCase().contains(query)) return true;
          final peerId = resolveDmPeerProfileId(
            item: item,
            knownPeerId: peerMap[item.chatId],
            activeProfileId: activeProfileId,
          );
          if (peerId == null) return false;
          final peer = ref.watch(profileProvider(peerId)).valueOrNull;
          return '${peer?.displayName ?? ''} ${peer?.username ?? ''}'
              .toLowerCase()
              .contains(query);
        })
        .toList(growable: false);
  }

  void _retryContacts() {
    final ids = ref.read(forwardMessageAcceptedFriendIdsProvider).valueOrNull;
    if (ids != null) {
      for (final id in ids) {
        ref.invalidate(profileProvider(id));
      }
    }
    ref.invalidate(forwardMessageAcceptedFriendIdsProvider);
    ref.invalidate(forwardMessageContactsProvider);
  }

  void _onChatSelected(ChatListItem item, String? ownerProfileId) {
    if (_forwarding) return;
    setState(() {
      _selectedChatId = item.chatId;
      _selectedFriendProfileId = null;
      _selectedTargetOwnerProfileId = ownerProfileId;
    });
  }

  void _onContactSelected(ForwardMessageContact contact, String ownerId) {
    if (_forwarding || widget.messagesToForward.isEmpty) return;
    setState(() {
      _selectedChatId = null;
      _selectedFriendProfileId = contact.profileId;
      _selectedTargetOwnerProfileId = ownerId;
    });
  }

  bool get _hasTarget =>
      _selectedChatId != null || _selectedFriendProfileId != null;

  Future<void> _submitSelectedForward() async {
    if (_forwarding || !_hasTarget || widget.messagesToForward.isEmpty) return;

    final selectedChatId = _selectedChatId;
    final friendProfileId = _selectedFriendProfileId;
    final expectedOwner = _selectedTargetOwnerProfileId;
    final commentary = _commentController.text.trim();
    final l10n = AppLocalizations.of(context)!;
    setState(() => _forwarding = true);

    final beforeRequest = ref.read(authControllerProvider).session;
    final expectedAuthorization = beforeRequest?.authorizationHeader;
    String? err;
    var targetChatId = selectedChatId;
    if (beforeRequest == null ||
        expectedOwner == null ||
        beforeRequest.activeProfileId != expectedOwner) {
      err = kChatActionStaleContext;
    } else if (friendProfileId != null) {
      err = await ref
          .read(chatActionsProvider)
          .openDmWithProfile(friendProfileId);
      if (err == null) {
        final afterRequest = ref.read(authControllerProvider).session;
        if (afterRequest == null ||
            afterRequest.activeProfileId != expectedOwner ||
            afterRequest.authorizationHeader !=
                beforeRequest.authorizationHeader) {
          err = kChatActionStaleContext;
        } else {
          targetChatId = ref.read(selectedChatIdProvider);
          if (targetChatId == null || targetChatId.isEmpty) {
            err = kChatActionStaleContext;
          }
        }
      }
    }
    if (err == null && targetChatId != null) {
      final beforeForward = ref.read(authControllerProvider).session;
      if (beforeForward == null ||
          beforeForward.activeProfileId != expectedOwner ||
          (friendProfileId != null &&
              beforeForward.authorizationHeader != expectedAuthorization)) {
        err = kChatActionStaleContext;
      } else {
        err = await ref
            .read(chatActionsProvider)
            .forwardMessages(
              sourceMessageIds: widget.messagesToForward
                  .map((m) => m.id)
                  .toList(),
              targetChatId: targetChatId,
              commentary: commentary.isEmpty ? null : commentary,
              withoutAttribution: widget.withoutAttribution,
              expectedProfileId: expectedOwner,
              expectedAuthorization: expectedAuthorization,
            );
        final afterForward = ref.read(authControllerProvider).session;
        if (afterForward?.activeProfileId != expectedOwner ||
            afterForward?.authorizationHeader != expectedAuthorization) {
          err = kChatActionStaleContext;
        }
      }
    }
    if (!mounted) return;
    setState(() => _forwarding = false);
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(chatActionErrorMessage(l10n, err))),
      );
      return;
    }
    if (targetChatId == null) return;
    await _completeForward(targetChatId, l10n);
  }

  Future<void> _completeForward(
    String targetChatId,
    AppLocalizations l10n,
  ) async {
    if (!mounted) return;
    Navigator.of(context).pop();
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(
          widget.withoutAttribution
              ? l10n.chatCopyAsNewSuccess
              : l10n.chatForwardSuccess,
        ),
      ),
    );
    ref.read(chatActionsProvider).selectChat(targetChatId);
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final listState = ref.watch(chatListControllerProvider);
    final activeProfileId = ref.watch(authControllerProvider).activeProfileId;
    final peerMap = ref.watch(dmPeerProfileByChatIdProvider);
    final contactsState = ref.watch(forwardMessageContactsProvider);
    final existingDmPeerIds = listState.items
        .map(
          (item) => resolveDmPeerProfileId(
            item: item,
            knownPeerId: peerMap[item.chatId],
            activeProfileId: activeProfileId,
          ),
        )
        .whereType<String>()
        .toSet();
    final chats = _filteredChats(
      listState.items,
      l10n,
      peerMap,
      activeProfileId,
    );
    final query = _searchController.text.trim().toLowerCase();
    final contacts = contactsState.valueOrNull?.contacts
        .where((contact) => !existingDmPeerIds.contains(contact.profileId))
        .where((contact) => contact.searchText.contains(query))
        .toList(growable: false);

    return SafeArea(
      key: widget.withoutAttribution
          ? ForwardMessageSheet.copyAsNewSheetKey
          : ForwardMessageSheet.sheetKey,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    widget.withoutAttribution
                        ? l10n.chatCopyAsNewTitle
                        : l10n.chatForwardTitle,
                    style: theme.textTheme.titleLarge,
                  ),
                ),
                IconButton(
                  key: ForwardMessageSheet.closeButtonKey,
                  tooltip: l10n.commonCancel,
                  onPressed: _forwarding
                      ? null
                      : () => Navigator.of(context).pop(),
                  icon: const Icon(Icons.close),
                ),
              ],
            ),
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
            if (_forwarding) const LinearProgressIndicator(minHeight: 2),
            Expanded(
              child:
                  (listState.isLoading && listState.items.isEmpty ||
                      contactsState.isLoading && chats.isEmpty)
                  ? Center(child: Text(l10n.commonLoading))
                  : chats.isEmpty &&
                        (contacts == null || contacts.isEmpty) &&
                        contactsState.hasError
                  ? VoiceStatePanel(
                      title: l10n.socialFriendsLoadError,
                      icon: Icons.people_outline,
                      actionLabel: l10n.commonRetry,
                      onAction: _forwarding ? null : _retryContacts,
                    )
                  : chats.isEmpty &&
                        (contacts == null || contacts.isEmpty) &&
                        !contactsState.isLoading
                  ? VoiceStatePanel(
                      title: l10n.chatForwardEmpty,
                      icon: Icons.chat_bubble_outline,
                    )
                  : ListView(
                      children: [
                        for (final item in chats)
                          _buildChatRow(
                            item: item,
                            peerMap: peerMap,
                            activeProfileId: activeProfileId,
                            l10n: l10n,
                          ),
                        if (contactsState.isLoading && chats.isNotEmpty)
                          const Padding(
                            padding: EdgeInsets.all(16),
                            child: Center(child: CircularProgressIndicator()),
                          ),
                        if (contactsState.hasError && chats.isNotEmpty)
                          VoiceListRow(
                            title: l10n.socialFriendsLoadError,
                            leading: const Icon(Icons.people_outline),
                            trailing: TextButton(
                              onPressed: _forwarding ? null : _retryContacts,
                              child: Text(l10n.commonRetry),
                            ),
                          ),
                        if (contacts != null && contacts.isNotEmpty) ...[
                          Padding(
                            padding: const EdgeInsets.fromLTRB(12, 12, 12, 4),
                            child: Text(
                              l10n.socialTabFriends,
                              style: theme.textTheme.titleSmall,
                            ),
                          ),
                          for (final contact in contacts)
                            Semantics(
                              selected:
                                  _selectedFriendProfileId == contact.profileId,
                              child: VoiceListRow(
                                key: ForwardMessageSheet.contactTileKey(
                                  contact.profileId,
                                ),
                                title: contact.displayName,
                                subtitle:
                                    contact.profile?.username.isNotEmpty == true
                                    ? '@${contact.profile!.username}'
                                    : null,
                                leading: VoiceAvatar(
                                  imageUrl: contact.profile?.avatarUrl,
                                  label: contact.displayName,
                                ),
                                selected:
                                    _selectedFriendProfileId ==
                                    contact.profileId,
                                trailing:
                                    _selectedFriendProfileId ==
                                        contact.profileId
                                    ? const Icon(Icons.check_circle_outline)
                                    : null,
                                onTap: _forwarding
                                    ? null
                                    : () => _onContactSelected(
                                        contact,
                                        contactsState.value!.ownerProfileId,
                                      ),
                              ),
                            ),
                        ],
                      ],
                    ),
            ),
            if (_hasTarget) ...[
              const SizedBox(height: 8),
              TextField(
                key: ForwardMessageSheet.commentFieldKey,
                controller: _commentController,
                decoration: InputDecoration(
                  hintText: l10n.chatForwardCommentaryHint,
                ),
                maxLines: 2,
                textCapitalization: TextCapitalization.sentences,
              ),
            ],
            const SizedBox(height: 8),
            Row(
              children: [
                TextButton(
                  key: ForwardMessageSheet.cancelButtonKey,
                  onPressed: _forwarding
                      ? null
                      : () => Navigator.of(context).pop(),
                  child: Text(l10n.commonCancel),
                ),
                const Spacer(),
                if (_hasTarget)
                  FilledButton.icon(
                    key: ForwardMessageSheet.submitButtonKey,
                    onPressed: _forwarding || widget.messagesToForward.isEmpty
                        ? null
                        : _submitSelectedForward,
                    icon: _forwarding
                        ? const SizedBox(
                            width: 16,
                            height: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Icon(Icons.send),
                    label: Text(
                      widget.withoutAttribution
                          ? l10n.chatMessageCopyAsNew
                          : l10n.chatMessageForward,
                    ),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildChatRow({
    required ChatListItem item,
    required Map<String, String> peerMap,
    required String? activeProfileId,
    required AppLocalizations l10n,
  }) {
    final peerId = resolveDmPeerProfileId(
      item: item,
      knownPeerId: peerMap[item.chatId],
      activeProfileId: activeProfileId,
    );
    final profileName = peerId == null
        ? null
        : ref.watch(profileProvider(peerId)).valueOrNull?.displayName;
    final title = profileName != null && profileName.isNotEmpty
        ? profileName
        : _chatTitleFallback(l10n, item);
    return Semantics(
      selected: _selectedChatId == item.chatId,
      child: VoiceListRow(
        key: ForwardMessageSheet.chatTileKey(item.chatId),
        title: title,
        subtitle: item.lastMessagePreview,
        selected: _selectedChatId == item.chatId,
        leading: item.chat.isGroup
            ? VoiceAvatar(imageUrl: item.chat.avatarUrl, label: title)
            : null,
        trailing: _selectedChatId == item.chatId
            ? const Icon(Icons.check_circle_outline)
            : null,
        onTap: _forwarding
            ? null
            : () => _onChatSelected(item, activeProfileId),
      ),
    );
  }
}
