import 'dart:typed_data';

import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../backend/chats_client.dart';
import '../../backend/files_client.dart';
import '../../backend/messages_client.dart';
import '../../backend/voice_client.dart';
import '../../state/auth_providers.dart';
import '../../state/call_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/gateway_providers.dart';
import '../../state/presence_providers.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_avatar.dart';
import '../core/voice_compact_banner.dart';
import '../core/voice_state_panel.dart';
import '../core/voice_send_button.dart';
import '../social/presence_indicator.dart';
import 'chat_message_list.dart';

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

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final room = ref.watch(chatRoomControllerProvider(widget.chatId));
    final activeId = ref.watch(authControllerProvider).activeProfileId;
    final peerId = resolveDmPeerForChatId(
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
    final canCall = ref.watch(gatewayConfigProvider).canPlaceVoiceCalls;
    final title = peerName ?? l10n.chatRoomTitle(widget.chatId.substring(0, 8));
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
                if (peerId != null && canCall) ...[
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
                    onSubmitted: room.isSending ? null : (_) => _send(),
                  ),
                ),
                const SizedBox(width: 8),
                IconButton(
                  key: ChatRoomPanel.attachKey,
                  tooltip: l10n.chatAttachFile,
                  onPressed: room.isSending || _uploadingAttachment
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
                  onPressed: _send,
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

  Future<void> _send() async {
    final text = _composer.text;
    ref.read(realtimeHubProvider).typingStop(widget.chatId);
    final err = await ref
        .read(chatRoomControllerProvider(widget.chatId).notifier)
        .sendMessage(text);
    if (!mounted) return;
    if (err == null) {
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
    if (action == 'edit') {
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

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (message.content.isNotEmpty) Text(message.content),
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
