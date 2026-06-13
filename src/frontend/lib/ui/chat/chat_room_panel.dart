import 'dart:typed_data';

import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../backend/chats_client.dart';
import '../../backend/files_client.dart';
import '../../backend/mention_parser.dart';
import '../../backend/messages_client.dart';
import '../../backend/voice_client.dart';
import '../../state/auth_providers.dart';
import '../../state/call_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/connectivity_providers.dart';
import '../../state/gateway_providers.dart';
import '../../state/presence_providers.dart';
import '../../state/social_providers.dart';
import '../../state/space_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_avatar.dart';
import '../core/voice_compact_banner.dart';
import '../core/voice_state_panel.dart';
import '../core/voice_send_button.dart';
import '../social/presence_indicator.dart';
import 'chat_message_list.dart';
import 'message_reactions_row.dart';
import 'forward_message_sheet.dart';
import 'mention_message_content.dart';
import '../shell/side_panel.dart';
import '../space/space_chat_slow_mode_sheet.dart';
import '../search/in_chat_search.dart';
import '../../state/shell_providers.dart';

/// Main column: message history (REST) + composer; live updates via Realtime WS.
class ChatRoomPanel extends ConsumerStatefulWidget {
  const ChatRoomPanel({
    super.key,
    required this.chatId,
    this.onBack,
    this.attachmentPicker,
  });

  static const Key panelKey = Key('chat_room_panel');
  static const Key messagesKey = Key('chat_room_messages');
  static const Key inputKey = Key('chat_room_input');
  static const Key sendKey = Key('chat_room_send');
  static const Key attachKey = Key('chat_room_attach');
  static const Key peerPresenceKey = Key('chat_room_peer_presence');
  static const Key loadOlderKey = Key('chat_room_load_older');
  static const Key audioCallKey = Key('chat_room_audio_call');
  static const Key videoCallKey = Key('chat_room_video_call');
  static const Key newMessagesChipKey = Key('chat_room_new_messages');
  static const Key pinnedBarKey = Key('chat_room_pinned_bar');
  static const Key groupMembersKey = Key('chat_room_group_members');
  static const Key emojiPickerKey = Key('chat_room_emoji_picker');
  static const Key offlineBannerKey = Key('chat_room_offline_banner');
  static const Key spaceSlowModeKey = Key('chat_room_space_slow_mode');
  static const Key inChatSearchKey = Key('chat_room_in_chat_search');
  static const Key groupVoiceStartKey = Key('chat_room_group_voice_start');
  static const Key groupVoiceJoinKey = Key('chat_room_group_voice_join');
  static Key attachmentPreviewKey(String fileId) =>
      ValueKey('chat_attachment_$fileId');

  final String chatId;
  final VoidCallback? onBack;
  final ChatAttachmentPicker? attachmentPicker;

  @override
  ConsumerState<ChatRoomPanel> createState() => _ChatRoomPanelState();
}

typedef ChatAttachmentPicker = Future<ChatAttachmentFile?> Function();

class ChatAttachmentFile {
  const ChatAttachmentFile({
    required this.bytes,
    required this.contentType,
    required this.name,
  });

  final Uint8List bytes;
  final String contentType;
  final String name;
}

class _ChatRoomPanelState extends ConsumerState<ChatRoomPanel> {
  final _composer = TextEditingController();
  final _composerFocus = FocusNode();
  final _scrollController = ScrollController();
  var _uploadingAttachment = false;
  var _initialUnreadCount = 0;
  var _unreadCaptured = false;
  var _pendingNewMessages = 0;
  var _wasNearBottom = true;

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScroll);
  }

  @override
  void dispose() {
    _composer.dispose();
    _composerFocus.dispose();
    _scrollController.dispose();
    super.dispose();
  }

  void _refocusComposer() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      _composerFocus.requestFocus();
    });
  }

  void _onScroll() {
    if (!_scrollController.hasClients) return;
    final near =
        _scrollController.position.pixels >=
        _scrollController.position.maxScrollExtent - 80;
    if (near && _pendingNewMessages > 0) {
      setState(() => _pendingNewMessages = 0);
    }
    _wasNearBottom = near;
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scrollController.hasClients) return;
      _scrollController.animateTo(
        _scrollController.position.maxScrollExtent,
        duration: const Duration(milliseconds: 200),
        curve: Curves.easeOut,
      );
    });
  }

  void _scrollToMessage(String messageId) {
    final room = ref.read(chatRoomControllerProvider(widget.chatId));
    final index = room.messages.indexWhere((m) => m.id == messageId);
    if (index < 0) return;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scrollController.hasClients) return;
      final max = _scrollController.position.maxScrollExtent;
      final count = room.messages.length;
      final target = count <= 1 ? max : max * (index / (count - 1));
      _scrollController.animateTo(
        target,
        duration: const Duration(milliseconds: 250),
        curve: Curves.easeOut,
      );
    });
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final room = ref.watch(chatRoomControllerProvider(widget.chatId));
    final isOffline = ref.watch(isDeviceOfflineProvider) || room.isOfflineCache;
    final activeId = ref.watch(authControllerProvider).activeProfileId;
    final canCall = ref.watch(gatewayConfigProvider).canPlaceVoiceCalls;
    String? groupName;
    String? spaceId;
    var slowModeSeconds = 0;
    var isGroup = false;
    for (final item in ref.watch(chatListControllerProvider).items) {
      if (item.chatId == widget.chatId && item.chat.isGroup) {
        groupName = item.chat.name;
        spaceId = item.chat.spaceId;
        slowModeSeconds = item.chat.slowModeSeconds;
        isGroup = true;
        break;
      }
    }
    final peerId = isGroup
        ? null
        : resolveDmPeerForChatId(
            chatId: widget.chatId,
            knownPeers: ref.watch(dmPeerProfileByChatIdProvider),
            listItems: ref.watch(chatListControllerProvider).items,
            activeProfileId: activeId,
            messages: room.messages,
          );
    final peerProfile = peerId != null
        ? ref.watch(profileProvider(peerId)).valueOrNull
        : null;
    final peerName = peerProfile?.displayName;
    final peerPresence = peerId != null
        ? ref.watch(presenceProvider(peerId))
        : null;
    final canSetSlowMode = spaceId != null
        ? ref
                .watch(
                  spacePermissionProvider((
                    spaceId: spaceId,
                    permission: 'TEXT_CHAT_SET_SLOW_MODE',
                    chatId: widget.chatId,
                    voiceRoomId: null,
                  )),
                )
                .valueOrNull ??
            false
        : false;
    final activeGroupCall = isGroup
        ? ref.watch(groupActiveCallProvider(widget.chatId))
        : null;
    final callState = ref.watch(callControllerProvider);
    final inThisGroupVoice =
        callState.isActive &&
        callState.session?.chatId == widget.chatId &&
        callState.session?.isGroupVoice == true;
    final shortId = widget.chatId.length <= 8
        ? widget.chatId
        : widget.chatId.substring(0, 8);
    final title = isGroup
        ? (groupName ?? l10n.chatRoomTitle(shortId))
        : (peerName ?? groupName ?? l10n.chatRoomTitle(shortId));
    final voice = VoiceColors.of(context);

    if (!_unreadCaptured) {
      ChatListItem? listItem;
      for (final item in ref.read(chatListControllerProvider).items) {
        if (item.chatId == widget.chatId) {
          listItem = item;
          break;
        }
      }
      _initialUnreadCount = listItem?.unreadCount ?? 0;
      _unreadCaptured = true;
    }

    ref.listen(chatRoomControllerProvider(widget.chatId), (prev, next) {
      final prevLen = prev?.messages.length ?? 0;
      if (next.messages.length > prevLen) {
        final added = next.messages.length - prevLen;
        if (_wasNearBottom) {
          _scrollToBottom();
        } else {
          setState(() => _pendingNewMessages += added);
        }
      }
    });

    ref.listen(pendingComposerEmojiProvider, (prev, next) {
      if (next == null || next.isEmpty) return;
      final text = _composer.text;
      _composer.text = '$text$next';
      _composer.selection = TextSelection.collapsed(offset: _composer.text.length);
      ref.read(pendingComposerEmojiProvider.notifier).state = null;
      _composerFocus.requestFocus();
    });

    return Column(
      key: ChatRoomPanel.panelKey,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Material(
          color: voice.surface,
          elevation: 0,
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
            child: Row(
              children: [
                if (widget.onBack != null) ...[
                  IconButton(
                    tooltip: l10n.chatRoomBack,
                    onPressed: widget.onBack,
                    icon: const Icon(Icons.arrow_back),
                  ),
                  const SizedBox(width: 4),
                ],
                if (peerProfile != null) ...[
                  VoiceAvatar(
                    imageUrl: peerProfile.avatarUrl,
                    label: peerProfile.displayName,
                    radius: 16,
                  ),
                  const SizedBox(width: 8),
                ],
                if (peerPresence != null) ...[
                  PresenceIndicator(
                    key: ChatRoomPanel.peerPresenceKey,
                    presence: peerPresence,
                    semanticLabel: _presenceLabel(l10n, peerPresence.status),
                    size: 10,
                  ),
                  const SizedBox(width: 8),
                ],
                Expanded(
                  child: Text(
                    title,
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                ),
                IconButton(
                  key: ChatRoomPanel.inChatSearchKey,
                  tooltip: l10n.inChatSearchOpen,
                  onPressed: () {
                    showModalBottomSheet<void>(
                      context: context,
                      isScrollControlled: true,
                      builder: (context) => SizedBox(
                        height: MediaQuery.sizeOf(context).height * 0.5,
                        child: InChatSearch(chatId: widget.chatId),
                      ),
                    );
                  },
                  icon: const Icon(Icons.search),
                ),
                if (isGroup) ...[
                  if (canCall && !inThisGroupVoice)
                    _GroupVoiceHeaderButton(
                      activeGroupCall: activeGroupCall,
                      chatId: widget.chatId,
                      l10n: l10n,
                    ),
                  if (canSetSlowMode)
                    IconButton(
                      key: ChatRoomPanel.spaceSlowModeKey,
                      tooltip: l10n.spaceSlowMode,
                      onPressed: () => SpaceChatSlowModeSheet.show(
                        context,
                        chatId: widget.chatId,
                        currentSeconds: slowModeSeconds,
                      ),
                      icon: const Icon(Icons.timer_outlined),
                    ),
                  IconButton(
                    key: ChatRoomPanel.groupMembersKey,
                    tooltip: l10n.chatGroupMembersTooltip,
                    onPressed: () => openMembersPanel(
                      context,
                      ref,
                      chatId: widget.chatId,
                      groupName: groupName,
                    ),
                    icon: const Icon(Icons.group_outlined),
                  ),
                ],
                if (!isGroup && peerId != null && canCall) ...[
                  IconButton(
                    key: ChatRoomPanel.audioCallKey,
                    tooltip: l10n.callStartAudio,
                    onPressed: () => ref
                        .read(callControllerProvider.notifier)
                        .startCall(
                          chatId: widget.chatId,
                          calleeProfileId: peerId,
                        ),
                    icon: const Icon(Icons.call_outlined),
                  ),
                  IconButton(
                    key: ChatRoomPanel.videoCallKey,
                    tooltip: l10n.callStartVideo,
                    onPressed: () => ref
                        .read(callControllerProvider.notifier)
                        .startCall(
                          chatId: widget.chatId,
                          calleeProfileId: peerId,
                          mediaKind: VoiceCallMediaKind.video,
                        ),
                    icon: const Icon(Icons.videocam_outlined),
                  ),
                ],
                _RealtimeBadge(status: room.realtimeStatus, l10n: l10n),
              ],
            ),
          ),
        ),
        if (room.isLoading) const LinearProgressIndicator(minHeight: 2),
        if (room.realtimeStatus == RealtimeLinkStatus.reconnecting)
          VoiceCompactBanner(
            message: l10n.chatRealtimeReconnecting,
            icon: Icons.sync_problem,
            tone: VoiceBannerTone.warning,
          ),
        if (room.isOfflineCache || ref.watch(isDeviceOfflineProvider))
          VoiceCompactBanner(
            key: ChatRoomPanel.offlineBannerKey,
            message: l10n.chatOfflineReadOnly,
            icon: Icons.cloud_off_outlined,
            tone: VoiceBannerTone.warning,
          ),
        if (isGroup && canCall && !inThisGroupVoice)
          _GroupVoiceJoinBanner(
            activeGroupCall: activeGroupCall,
            l10n: l10n,
          ),
        if (room.typingProfileIds.isNotEmpty)
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 6, 12, 0),
            child: Text(
              l10n.chatTyping,
              style: TextStyle(color: voice.textSecondary),
            ),
          ),
        if (_pendingNewMessages > 0)
          ChatNewMessagesChip(
            key: ChatRoomPanel.newMessagesChipKey,
            label: l10n.chatNewMessages,
            onTap: () {
              setState(() => _pendingNewMessages = 0);
              _scrollToBottom();
            },
          ),
        if (room.pinnedMessages.isNotEmpty)
          _PinnedMessagesBar(
            key: ChatRoomPanel.pinnedBarKey,
            pinned: room.pinnedMessages,
            label: l10n.chatPinnedBar(room.pinnedMessages.length),
            onTap: (messageId) => _scrollToMessage(messageId),
          ),
        Expanded(
          child: room.messages.isEmpty && !room.isLoading
              ? room.errorMessage != null
                    ? VoiceStatePanel(
                        title: l10n.chatRoomError(room.errorMessage!),
                        icon: Icons.cloud_off_outlined,
                        actionLabel: l10n.commonRetry,
                        onAction: () => ref
                            .read(
                              chatRoomControllerProvider(
                                widget.chatId,
                              ).notifier,
                            )
                            .loadInitial(),
                      )
                    : VoiceStatePanel(
                        title: l10n.chatRoomEmpty,
                        message: l10n.chatRoomEmptyHint,
                        icon: Icons.chat_bubble_outline,
                      )
              : _MessageListView(
                  key: ChatRoomPanel.messagesKey,
                  chatId: widget.chatId,
                  scrollController: _scrollController,
                  room: room,
                  activeId: activeId,
                  l10n: l10n,
                  initialUnreadCount: _initialUnreadCount,
                  onLongPress: (msg, isMine) =>
                      _showMessageActions(msg, isMine),
                ),
        ),
        if (room.errorMessage != null && room.messages.isNotEmpty)
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12),
            child: Text(
              l10n.chatRoomError(room.errorMessage!),
              style: TextStyle(color: Theme.of(context).colorScheme.error),
            ),
          ),
        SafeArea(
          top: false,
          child: Padding(
            padding: const EdgeInsets.fromLTRB(12, 4, 12, 8),
            child: Row(
              children: [
                IconButton(
                  tooltip: l10n.chatMentionInsert,
                  onPressed: room.isSending || isOffline
                      ? null
                      : () => _showMentionMenu(context),
                  icon: const Icon(Icons.alternate_email),
                ),
                IconButton(
                  key: ChatRoomPanel.emojiPickerKey,
                  tooltip: l10n.chatMessageAddReaction,
                  onPressed: room.isSending || isOffline
                      ? null
                      : () => openEmojiPanel(
                          context,
                          ref,
                          onSelected: (emoji) {
                            ref
                                .read(pendingComposerEmojiProvider.notifier)
                                .state = emoji;
                          },
                        ),
                  icon: const Icon(Icons.emoji_emotions_outlined),
                ),
                Expanded(
                  child: TextField(
                    key: ChatRoomPanel.inputKey,
                    controller: _composer,
                    focusNode: _composerFocus,
                    decoration: InputDecoration(
                      hintText: l10n.chatRoomInputHint,
                      isDense: true,
                    ),
                    onChanged: (value) {
                      final hub = ref.read(realtimeHubProvider);
                      if (value.trim().isEmpty) {
                        hub.typingStop(widget.chatId);
                      } else {
                        hub.typingStart(widget.chatId);
                      }
                    },
                    onSubmitted: room.isSending || isOffline ? null : (_) => _send(),
                    readOnly: isOffline,
                  ),
                ),
                const SizedBox(width: 8),
                IconButton(
                  key: ChatRoomPanel.attachKey,
                  tooltip: l10n.chatAttachFile,
                  onPressed: room.isSending || _uploadingAttachment || isOffline
                      ? null
                      : _attachAndSend,
                  icon: _uploadingAttachment
                      ? const SizedBox(
                          width: 18,
                          height: 18,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Icon(Icons.attach_file),
                ),
                const SizedBox(width: 4),
                VoiceSendButton(
                  key: ChatRoomPanel.sendKey,
                  onPressed: room.isSending ? null : _send,
                  isLoading: room.isSending,
                  tooltip: l10n.chatSendMessage,
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }

  Future<void> _showMentionMenu(BuildContext context) async {
    final l10n = AppLocalizations.of(context)!;
    final membersAsync = ref.read(groupMembersProvider(widget.chatId));
    final memberIds = membersAsync.maybeWhen(
      data: (data) => data.members.map((m) => m.profileId).toList(),
      orElse: () => const <String>[],
    );
    final choice = await showModalBottomSheet<String>(
      context: context,
      builder: (ctx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              title: Text(l10n.chatMentionEveryone),
              onTap: () => Navigator.pop(ctx, '@everyone '),
            ),
            ListTile(
              title: Text(l10n.chatMentionHere),
              onTap: () => Navigator.pop(ctx, '@here '),
            ),
            for (final id in memberIds)
              _MentionMemberTile(
                profileId: id,
                onPick: () => Navigator.pop(ctx, '@$id '),
              ),
          ],
        ),
      ),
    );
    if (choice == null || !mounted) return;
    final text = _composer.text;
    _composer.text = '$text$choice';
    _composer.selection = TextSelection.collapsed(offset: _composer.text.length);
    _refocusComposer();
  }

  Future<void> _send() async {
    if (ref.read(isDeviceOfflineProvider) ||
        ref.read(chatRoomControllerProvider(widget.chatId)).isOfflineCache) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(AppLocalizations.of(context)!.chatOfflineSendBlocked)),
      );
      return;
    }
    final text = _composer.text;
    ref.read(realtimeHubProvider).typingStop(widget.chatId);
    final memberIds = ref
        .read(groupMembersProvider(widget.chatId))
        .maybeWhen(
          data: (data) => data.members.map((m) => m.profileId),
          orElse: () => const <String>[],
        );
    final mentions = parseMentionsFromContent(text, memberProfileIds: memberIds);
    final err = await ref
        .read(chatRoomControllerProvider(widget.chatId).notifier)
        .sendMessage(text, mentions: mentions);
    if (!mounted) return;
    if (err == kChatOfflineBlockedError) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(AppLocalizations.of(context)!.chatOfflineSendBlocked),
        ),
      );
    } else if (err == null) {
      _composer.clear();
    }
    _refocusComposer();
  }

  Future<void> _attachAndSend() async {
    final picker = widget.attachmentPicker ?? _defaultPickChatAttachment;
    final picked = await picker();
    if (picked == null || !mounted) return;
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    setState(() => _uploadingAttachment = true);
    try {
      final files = ref.read(voiceFilesClientProvider);
      final chatType = ref
          .read(chatListProvider)
          .valueOrNull
          ?.items
          .where((item) => item.chatId == widget.chatId)
          .map((item) => item.chat.type)
          .firstOrNull;
      final ticket = await files.requestUpload(
        authorization: auth,
        originalName: picked.name,
        mimeType: picked.contentType,
        sizeBytes: picked.bytes.length,
        chatId: widget.chatId,
        chatType: chatType,
      );
      if (ticket is! FilesApiOk<FileUploadTicket>) return;
      final put = await files.putBytes(
        uploadUrl: ticket.data.presignedPutUrl,
        bytes: picked.bytes,
        mimeType: picked.contentType,
      );
      if (put is! FilesApiOk<void>) return;
      final confirmed = await files.confirmUpload(
        authorization: auth,
        fileId: ticket.data.fileId,
        bytes: picked.bytes,
      );
      if (confirmed is! FilesApiOk<FileMetadataData>) return;
      final metadata = confirmed.data;
      final err = await ref
          .read(chatRoomControllerProvider(widget.chatId).notifier)
          .sendMessage(
            _composer.text,
            attachments: [
              MessageAttachment(
                fileId: metadata.fileId,
                type: metadata.fileType,
                name: metadata.originalName,
                sizeBytes: metadata.sizeBytes,
              ),
            ],
          );
      if (!mounted) return;
      if (err == null) {
        _composer.clear();
      }
      _refocusComposer();
    } finally {
      if (mounted) {
        setState(() => _uploadingAttachment = false);
      }
    }
  }

  Future<void> _showMessageActions(VoiceMessage message, bool isMine) async {
    final action = await showModalBottomSheet<String>(
      context: context,
      builder: (context) {
        final sheetL10n = AppLocalizations.of(context)!;
        return SafeArea(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              if (message.deletedAt == null &&
                  message.messageKind != VoiceMessageKind.system) ...[
                ListTile(
                  leading: const Icon(Icons.add_reaction_outlined),
                  title: Text(sheetL10n.chatMessageAddReaction),
                  onTap: () => Navigator.of(context).pop('react'),
                ),
                ListTile(
                  leading: const Icon(Icons.forward_outlined),
                  title: Text(sheetL10n.chatMessageForward),
                  onTap: () => Navigator.of(context).pop('forward'),
                ),
                ListTile(
                  leading: Icon(
                    message.isPinned ? Icons.push_pin : Icons.push_pin_outlined,
                  ),
                  title: Text(
                    message.isPinned
                        ? sheetL10n.chatMessageUnpin
                        : sheetL10n.chatMessagePin,
                  ),
                  onTap: () => Navigator.of(context).pop(
                    message.isPinned ? 'unpin' : 'pin',
                  ),
                ),
              ],
              if (isMine)
                ListTile(
                  leading: const Icon(Icons.edit_outlined),
                  title: Text(sheetL10n.chatMessageEdit),
                  onTap: () => Navigator.of(context).pop('edit'),
                ),
              ListTile(
                leading: const Icon(Icons.delete_outline),
                title: Text(sheetL10n.chatMessageDeleteForMe),
                onTap: () => Navigator.of(context).pop('delete_me'),
              ),
              if (isMine)
                ListTile(
                  leading: const Icon(Icons.delete_forever_outlined),
                  title: Text(sheetL10n.chatMessageDeleteForEveryone),
                  onTap: () => Navigator.of(context).pop('delete_everyone'),
                ),
            ],
          ),
        );
      },
    );
    if (!mounted || action == null) return;
    final controller = ref.read(
      chatRoomControllerProvider(widget.chatId).notifier,
    );
    if (action == 'react') {
      final emoji = await _pickReactionEmoji();
      if (emoji != null) {
        await controller.addReaction(message.id, emoji);
      }
    } else if (action == 'pin' || action == 'unpin') {
      await controller.togglePin(
        message.id,
        currentlyPinned: message.isPinned,
      );
    } else if (action == 'forward') {
      await ForwardMessageSheet.show(
        context,
        sourceMessage: message,
        sourceChatId: widget.chatId,
      );
    } else if (action == 'edit') {
      final edited = await _promptEdit(message.content);
      if (edited != null) {
        await controller.editMessage(message.id, edited);
      }
    } else if (action == 'delete_me') {
      await controller.deleteMessage(message.id, forMe: true);
    } else if (action == 'delete_everyone') {
      await controller.deleteMessage(message.id, forMe: false);
    }
  }

  Future<String?> _pickReactionEmoji() async {
    const choices = ['👍', '❤️', '🔥', '😂', '🎉'];
    return showModalBottomSheet<String>(
      context: context,
      builder: (context) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 16, horizontal: 12),
          child: Wrap(
            alignment: WrapAlignment.center,
            spacing: 12,
            runSpacing: 12,
            children: [
              for (final emoji in choices)
                IconButton(
                  onPressed: () => Navigator.of(context).pop(emoji),
                  icon: Text(emoji, style: const TextStyle(fontSize: 28)),
                ),
            ],
          ),
        ),
      ),
    );
  }

  Future<String?> _promptEdit(String initial) async {
    final controller = TextEditingController(text: initial);
    try {
      return showDialog<String>(
        context: context,
        builder: (context) {
          final dialogL10n = AppLocalizations.of(context)!;
          return AlertDialog(
            title: Text(dialogL10n.chatEditMessageTitle),
            content: TextField(controller: controller, autofocus: true),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(context).pop(),
                child: Text(dialogL10n.commonCancel),
              ),
              FilledButton(
                onPressed: () => Navigator.of(context).pop(controller.text),
                child: Text(dialogL10n.commonSave),
              ),
            ],
          );
        },
      );
    } finally {
      controller.dispose();
    }
  }
}

class _GroupVoiceHeaderButton extends ConsumerWidget {
  const _GroupVoiceHeaderButton({
    required this.activeGroupCall,
    required this.chatId,
    required this.l10n,
  });

  final AsyncValue<VoiceCallSession?>? activeGroupCall;
  final String chatId;
  final AppLocalizations l10n;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final async = activeGroupCall;
    if (async == null) return const SizedBox.shrink();
    return async.when(
      data: (session) {
        if (session != null && session.roomId.isNotEmpty) {
          return IconButton(
            key: ChatRoomPanel.groupVoiceJoinKey,
            tooltip: l10n.callGroupVoiceJoin,
            onPressed: () => ref
                .read(callControllerProvider.notifier)
                .joinGroupVoice(roomId: session.roomId),
            icon: const Icon(Icons.headset_outlined),
          );
        }
        return IconButton(
          key: ChatRoomPanel.groupVoiceStartKey,
          tooltip: l10n.callGroupVoiceStart,
          onPressed: () => ref
              .read(callControllerProvider.notifier)
              .startGroupVoice(groupChatId: chatId),
          icon: const Icon(Icons.record_voice_over_outlined),
        );
      },
      loading: () => const SizedBox(
        width: 24,
        height: 24,
        child: CircularProgressIndicator(strokeWidth: 2),
      ),
      error: (_, _) => IconButton(
        key: ChatRoomPanel.groupVoiceStartKey,
        tooltip: l10n.callGroupVoiceStart,
        onPressed: () => ref
            .read(callControllerProvider.notifier)
            .startGroupVoice(groupChatId: chatId),
        icon: const Icon(Icons.record_voice_over_outlined),
      ),
    );
  }
}

class _GroupVoiceJoinBanner extends ConsumerWidget {
  const _GroupVoiceJoinBanner({
    required this.activeGroupCall,
    required this.l10n,
  });

  final AsyncValue<VoiceCallSession?>? activeGroupCall;
  final AppLocalizations l10n;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final async = activeGroupCall;
    if (async == null) return const SizedBox.shrink();
    return async.when(
      data: (session) {
        if (session == null || session.roomId.isEmpty) {
          return const SizedBox.shrink();
        }
        return VoiceCompactBanner(
          message: l10n.callGroupVoiceInProgress,
          icon: Icons.headset_mic_outlined,
          actionLabel: l10n.callGroupVoiceJoin,
          onAction: () => ref
              .read(callControllerProvider.notifier)
              .joinGroupVoice(roomId: session.roomId),
        );
      },
      loading: () => const SizedBox.shrink(),
      error: (_, _) => const SizedBox.shrink(),
    );
  }
}

class _MessageListView extends ConsumerWidget {
  const _MessageListView({
    super.key,
    required this.chatId,
    required this.scrollController,
    required this.room,
    required this.activeId,
    required this.l10n,
    required this.initialUnreadCount,
    required this.onLongPress,
  });

  final String chatId;
  final ScrollController scrollController;
  final ChatRoomState room;
  final String? activeId;
  final AppLocalizations l10n;
  final int initialUnreadCount;
  final void Function(VoiceMessage message, bool isMine) onLongPress;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final rows = buildChatMessageRows(
      messages: room.messages,
      unreadCount: initialUnreadCount,
    );
    final hasOlderControl = room.hasMore || room.isLoadingOlder;
    return ListView.builder(
      controller: scrollController,
      padding: const EdgeInsets.all(12),
      itemCount: rows.length + (hasOlderControl ? 1 : 0),
      itemBuilder: (context, index) {
        if (hasOlderControl && index == 0) {
          return Center(
            child: room.isLoadingOlder
                ? const Padding(
                    padding: EdgeInsets.all(8),
                    child: SizedBox(
                      width: 24,
                      height: 24,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    ),
                  )
                : TextButton.icon(
                    key: ChatRoomPanel.loadOlderKey,
                    icon: const Icon(Icons.expand_less),
                    label: Text(l10n.chatRoomLoadOlder),
                    onPressed: () => ref
                        .read(chatRoomControllerProvider(chatId).notifier)
                        .loadOlderMessages(),
                  ),
          );
        }
        final rowIndex = hasOlderControl ? index - 1 : index;
        final row = rows[rowIndex];
        final msg = row.message;
        final isMine = msg.senderProfileId == activeId;
        return Column(
          children: [
            if (row.unreadSeparator)
              ChatUnreadSeparator(label: l10n.chatUnreadSeparator),
            GestureDetector(
              onLongPress: () => onLongPress(msg, isMine),
              child: ChatMessageBubbleTile(
                message: msg,
                isMine: isMine,
                showTimestamp: row.showTimestamp,
                l10n: l10n,
                deliveryFooter: isMine
                    ? _DeliveryTick(
                        l10n: l10n,
                        delivered: room.deliveredMessageIds.contains(msg.id),
                        read: room.readMessageIds.contains(msg.id),
                      )
                    : null,
                content: _MessageBubbleContent(message: msg, l10n: l10n),
              ),
            ),
            MessageReactionsRow(
              message: msg,
              isMine: isMine,
              onToggle: (emoji, reactedByMe) => ref
                  .read(chatRoomControllerProvider(chatId).notifier)
                  .toggleReaction(
                    msg.id,
                    emoji,
                    currentlyReacted: reactedByMe,
                  ),
            ),
          ],
        );
      },
    );
  }
}

class _DeliveryTick extends StatelessWidget {
  const _DeliveryTick({
    required this.l10n,
    required this.delivered,
    required this.read,
  });

  final AppLocalizations l10n;
  final bool delivered;
  final bool read;

  @override
  Widget build(BuildContext context) {
    final label = read
        ? l10n.chatDeliveryRead
        : delivered
        ? l10n.chatDeliveryDelivered
        : l10n.chatDeliverySent;
    final icon = read || delivered ? Icons.done_all : Icons.done;
    final voice = VoiceColors.of(context);
    return Semantics(
      label: label,
      child: Icon(
        icon,
        size: 14,
        color: read ? voice.profileAccent : voice.textSecondary,
      ),
    );
  }
}

Future<ChatAttachmentFile?> _defaultPickChatAttachment() async {
  final file = await openFile();
  if (file == null) return null;
  final bytes = await file.readAsBytes();
  return ChatAttachmentFile(
    bytes: bytes,
    contentType: file.mimeType ?? _contentTypeFromName(file.name),
    name: file.name,
  );
}

String _contentTypeFromName(String name) {
  final lower = name.toLowerCase();
  if (lower.endsWith('.jpg') || lower.endsWith('.jpeg')) return 'image/jpeg';
  if (lower.endsWith('.png')) return 'image/png';
  if (lower.endsWith('.webp')) return 'image/webp';
  if (lower.endsWith('.pdf')) return 'application/pdf';
  return 'application/octet-stream';
}

class _MessageBubbleContent extends StatelessWidget {
  const _MessageBubbleContent({required this.message, required this.l10n});

  final VoiceMessage message;
  final AppLocalizations l10n;

  bool get _showForwardAttribution {
    final sender = message.forwardFromSender;
    return message.messageKind == VoiceMessageKind.forward ||
        (sender != null && sender.isNotEmpty);
  }

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (_showForwardAttribution)
          Padding(
            padding: const EdgeInsets.only(bottom: 4),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.forward, size: 14, color: voice.profileAccent),
                const SizedBox(width: 4),
                Flexible(
                  child: Text(
                    l10n.chatForwardFrom(message.forwardFromSender ?? ''),
                    style: Theme.of(context).textTheme.labelSmall?.copyWith(
                      color: voice.profileAccent,
                      fontStyle: FontStyle.italic,
                    ),
                  ),
                ),
              ],
            ),
          ),
        if (message.content.isNotEmpty)
          MentionMessageContent(
            content: message.content,
            mentions: message.mentions,
          ),
        if (message.editedAt != null)
          Text(
            l10n.chatMessageEdited,
            style: Theme.of(context).textTheme.labelSmall,
          ),
        for (final attachment in message.attachments) ...[
          if (message.content.isNotEmpty ||
              attachment != message.attachments.first)
            const SizedBox(height: 6),
          _AttachmentPreview(attachment: attachment),
        ],
      ],
    );
  }
}

class _AttachmentPreview extends ConsumerWidget {
  const _AttachmentPreview({required this.attachment});

  final MessageAttachment attachment;

  static bool _isHttpUrl(String? value) {
    if (value == null || value.isEmpty) return false;
    final uri = Uri.tryParse(value);
    return uri != null && (uri.scheme == 'http' || uri.scheme == 'https');
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final voice = VoiceColors.of(context);
    if (attachment.isImage) {
      final direct =
          _isHttpUrl(attachment.previewUrl) ? attachment.previewUrl : null;
      final directUrl =
          direct ?? (_isHttpUrl(attachment.url) ? attachment.url : null);
      final resolved = directUrl != null
          ? AsyncValue.data(directUrl)
          : ref.watch(fileAttachmentUrlProvider(attachment.fileId));
      final src = resolved.valueOrNull;
      return Semantics(
        key: ChatRoomPanel.attachmentPreviewKey(attachment.fileId),
        label: attachment.name ?? AppLocalizations.of(context)!.chatImageAttachment,
        child: ClipRRect(
          borderRadius: BorderRadius.circular(4),
          child: Container(
            constraints: const BoxConstraints(maxWidth: 220, maxHeight: 160),
            color: voice.surface,
            child: resolved.isLoading || src == null || src.isEmpty
                ? const _AttachmentIcon(icon: Icons.image_outlined)
                : Image.network(
                    src,
                    fit: BoxFit.cover,
                    errorBuilder: (context, error, stackTrace) =>
                        const _AttachmentIcon(icon: Icons.image_outlined),
                  ),
          ),
        ),
      );
    }
    return Container(
      key: ChatRoomPanel.attachmentPreviewKey(attachment.fileId),
      constraints: const BoxConstraints(maxWidth: 260),
      padding: const EdgeInsets.all(8),
      decoration: BoxDecoration(
        color: voice.surface,
        borderRadius: BorderRadius.circular(4),
        border: Border.all(color: voice.borderDefault),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.insert_drive_file_outlined, size: 20),
          const SizedBox(width: 8),
          Flexible(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  attachment.name ?? attachment.fileId,
                  overflow: TextOverflow.ellipsis,
                ),
                if (attachment.sizeBytes != null)
                  Text(
                    _formatBytes(attachment.sizeBytes!),
                    style: Theme.of(context).textTheme.labelSmall,
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _AttachmentIcon extends StatelessWidget {
  const _AttachmentIcon({required this.icon});

  final IconData icon;

  @override
  Widget build(BuildContext context) {
    return SizedBox(width: 160, height: 96, child: Center(child: Icon(icon)));
  }
}

String _formatBytes(int bytes) {
  if (bytes >= 1024 * 1024) {
    return '${(bytes / (1024 * 1024)).toStringAsFixed(1)} MB';
  }
  if (bytes >= 1024) {
    return '${(bytes / 1024).toStringAsFixed(1)} KB';
  }
  return '$bytes B';
}

String _presenceLabel(AppLocalizations l10n, String status) {
  return switch (status) {
    'online' => l10n.socialPresenceOnline,
    'idle' => l10n.socialPresenceIdle,
    'dnd' => l10n.socialPresenceDnd,
    _ => l10n.socialPresenceOffline,
  };
}

class _PinnedMessagesBar extends StatelessWidget {
  const _PinnedMessagesBar({
    super.key,
    required this.pinned,
    required this.label,
    required this.onTap,
  });

  final List<VoiceMessage> pinned;
  final String label;
  final void Function(String messageId) onTap;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final preview = pinned.first;
    return Material(
      color: voice.surface,
      child: InkWell(
        onTap: () => onTap(preview.id),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          child: Row(
            children: [
              Icon(Icons.push_pin, size: 18, color: voice.profileAccent),
              const SizedBox(width: 8),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      label,
                      style: Theme.of(context).textTheme.labelMedium?.copyWith(
                        color: voice.profileAccent,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    Text(
                      preview.content,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(color: voice.textSecondary),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _MentionMemberTile extends ConsumerWidget {
  const _MentionMemberTile({required this.profileId, required this.onPick});

  final String profileId;
  final VoidCallback onPick;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final profile = ref.watch(profileProvider(profileId)).valueOrNull;
    final label =
        profile?.displayName ?? profile?.handle ?? l10n.chatMentionMember(profileId);
    return ListTile(title: Text(label), onTap: onPick);
  }
}

class _RealtimeBadge extends StatelessWidget {
  const _RealtimeBadge({required this.status, required this.l10n});

  final RealtimeLinkStatus status;
  final AppLocalizations l10n;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final (label, color) = switch (status) {
      RealtimeLinkStatus.connected => (
        l10n.chatRealtimeConnected,
        voice.profileAccent,
      ),
      RealtimeLinkStatus.connecting => (
        l10n.chatRealtimeConnecting,
        voice.focusRing,
      ),
      RealtimeLinkStatus.reconnecting => (
        l10n.chatRealtimeReconnecting,
        voice.focusRing,
      ),
      RealtimeLinkStatus.disconnected => (
        l10n.chatRealtimeOffline,
        voice.textDisabled,
      ),
    };
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(Icons.circle, size: 8, color: color),
        const SizedBox(width: 4),
        Text(label, style: Theme.of(context).textTheme.labelSmall),
      ],
    );
  }
}
