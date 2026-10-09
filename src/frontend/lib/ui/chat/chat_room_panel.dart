import 'dart:async';
import 'dart:convert';

import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:uuid/uuid.dart';

import '../../l10n/app_localizations.dart';
import '../../backend/chats_client.dart';
import '../../e2e/e2e_file_crypto.dart';
import '../../backend/files_client.dart';
import '../../backend/mention_parser.dart';
import '../../backend/messages_client.dart';
import '../../backend/proto_mappers.dart' show protoTimestampToDateTime;
import '../../gen/voice/messaging/v1/messaging.pb.dart' as messaging_pb;
import '../../gen/voice/messaging/v1/messaging.pbenum.dart' as messaging_enums;
import '../../backend/space_permissions.dart';
import '../../backend/voice_client.dart';
import '../../state/bot_providers.dart';
import '../../state/auth_providers.dart';
import '../../state/e2e_providers.dart';
import '../../state/call_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/chat_draft_providers.dart';
import '../../backend/chat_draft_storage.dart';
import '../../settings/chat_theme_preference.dart';
import '../../state/connectivity_providers.dart';
import '../../state/gateway_providers.dart';
import '../../state/presence_providers.dart';
import '../../state/social_providers.dart';
import '../../state/subscription_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_emoji_style.dart';
import '../../theme/voice_layout.dart';
import '../api_error_messages.dart';
import '../core/voice_disabled_action.dart';
import '../core/chat_author_label.dart';
import '../core/voice_avatar.dart';
import '../core/voice_compact_banner.dart';
import '../core/voice_state_panel.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_chat_bubble.dart';
import '../core/voice_share_link.dart';
import '../core/voice_send_button.dart';
import '../a11y/voice_shortcuts.dart';
import '../social/presence_indicator.dart';
import '../report/report_sheet.dart';
import 'chat_info_panel.dart';
import 'chat_message_list.dart';
import 'message_reactions_row.dart';
import 'forward_message_sheet.dart';
import 'mention_message_content.dart';
import 'e2e_attachment_actions.dart';
import 'e2e_chat_settings.dart';
import 'e2e_identity_change_banner.dart';
import 'chat_composer_text_field.dart';
import 'composer_panels.dart';
import '../shell/side_panel.dart';
import '../space/space_chat_slow_mode_sheet.dart';
import '../search/in_chat_search.dart';
import '../../state/shell_providers.dart';
import '../../state/deep_link_navigation.dart';
import '../../state/shared_media_providers.dart';
import '../../state/space_providers.dart';
import '../../e2e/e2e_identity_trust.dart';
import '../../e2e/e2e_store_factory.dart';
import '../../e2e/e2e_verification_code.dart';
import '../../backend/e2e_client.dart';
import 'slash_command_menu.dart';
import 'slash_command_options_sheet.dart';
import 'thread_side_panel.dart';
import 'pinned_messages_panel.dart';

/// Main column: message history (REST) + composer; live updates via Realtime WS.
class ChatRoomPanel extends ConsumerStatefulWidget {
  const ChatRoomPanel({
    super.key,
    required this.chatId,
    this.onBack,
    this.attachmentPicker,
    this.showReconnectBanner = true,
  });

  static const Key panelKey = Key('chat_room_panel');
  static const Key messagesKey = Key('chat_room_messages');
  static const Key selectedMessageFocusKey = Key(
    'chat_room_selected_message_focus',
  );
  static const Key inputKey = Key('chat_room_input');
  static const Key sendKey = Key('chat_room_send');
  static const Key attachKey = Key('chat_room_attach');
  static const Key attachmentUploadFailureKey = Key(
    'chat_attachment_upload_failure',
  );
  static const Key attachmentUploadRetryKey = Key(
    'chat_attachment_upload_retry',
  );
  static const Key attachmentUploadCancelKey = Key(
    'chat_attachment_upload_cancel',
  );
  static const Key scheduledMessagesKey = Key('chat_scheduled_messages');
  static const Key scheduledCreateRetryKey = Key('chat_scheduled_create_retry');
  static const Key scheduledCreateCancelKey = Key(
    'chat_scheduled_create_cancel',
  );
  static const Key peerPresenceKey = Key('chat_room_peer_presence');
  static const Key loadOlderKey = Key('chat_room_load_older');
  static const Key audioCallKey = Key('chat_room_audio_call');
  static const Key videoCallKey = Key('chat_room_video_call');
  static const Key newMessagesChipKey = Key('chat_room_new_messages');
  static const Key pinnedBarKey = Key('chat_room_pinned_bar');
  static const Key pinnedMessagesHeaderKey = Key(
    'chat_room_pinned_messages_header',
  );
  static const Key groupMembersKey = Key('chat_room_group_members');
  static const Key slashCommandsKey = Key('chat_room_slash_commands');
  static const Key emojiPickerKey = Key('chat_room_emoji_picker');
  static const Key offlineBannerKey = Key('chat_room_offline_banner');
  static const Key reconnectBannerKey = Key('chat_room_reconnect_banner');
  static const Key spaceSlowModeKey = Key('chat_room_space_slow_mode');
  static const Key inChatSearchKey = Key('chat_room_in_chat_search');
  static const Key chatInfoKey = Key('chat_room_chat_info');
  static const Key groupVoiceStartKey = Key('chat_room_group_voice_start');
  static const Key groupVoiceJoinKey = Key('chat_room_group_voice_join');
  static Key attachmentPreviewKey(String fileId) =>
      ValueKey('chat_attachment_$fileId');

  final String chatId;
  final VoidCallback? onBack;
  final ChatAttachmentPicker? attachmentPicker;
  final bool showReconnectBanner;

  @override
  ConsumerState<ChatRoomPanel> createState() => _ChatRoomPanelState();
}

typedef ChatAttachmentPicker = Future<ChatAttachmentFile?> Function();

class _ScheduledSendAttempt {
  const _ScheduledSendAttempt({
    required this.chatId,
    required this.profileId,
    required this.authorization,
    required this.clientMessageId,
    required this.content,
    required this.scheduledAt,
    required this.sendWhenOnline,
    required this.mentions,
    this.threadParentId,
  });

  final String chatId;
  final String profileId;
  final String authorization;
  final String clientMessageId;
  final String content;
  final DateTime? scheduledAt;
  final bool sendWhenOnline;
  final List<MessageMention> mentions;
  final String? threadParentId;
}

class _ScheduledMessageRow extends StatelessWidget {
  const _ScheduledMessageRow({
    required this.item,
    required this.l10n,
    required this.busy,
    required this.onEdit,
    required this.onCancel,
    required this.onSendNow,
  });

  final messaging_pb.ScheduledMessage item;
  final AppLocalizations l10n;
  final bool busy;
  final VoidCallback onEdit;
  final VoidCallback onCancel;
  final VoidCallback onSendNow;

  @override
  Widget build(BuildContext context) {
    final scheduledAt = protoTimestampToDateTime(
      item.hasScheduledAt() ? item.scheduledAt : null,
    );
    final scheduleLabel = item.sendWhenOnline
        ? l10n.chatSendWhenOnline
        : scheduledAt == null
        ? l10n.chatSchedulePending
        : '${MaterialLocalizations.of(context).formatMediumDate(scheduledAt.toLocal())} ${MaterialLocalizations.of(context).formatTimeOfDay(TimeOfDay.fromDateTime(scheduledAt.toLocal()))}';
    final isEncrypted = item.hasPayload() && item.payload.isE2e;
    final content = item.hasPayload() && !isEncrypted
        ? item.payload.content
        : '';
    return Card(
      margin: const EdgeInsets.symmetric(vertical: 2),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
        child: Row(
          children: [
            const Icon(Icons.schedule_send_outlined, size: 20),
            const SizedBox(width: 8),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    scheduleLabel,
                    style: Theme.of(context).textTheme.labelMedium,
                  ),
                  if (content.isNotEmpty)
                    Text(content, maxLines: 2, overflow: TextOverflow.ellipsis)
                  else if (isEncrypted)
                    const SizedBox.shrink(),
                ],
              ),
            ),
            if (busy)
              const SizedBox(
                width: 20,
                height: 20,
                child: CircularProgressIndicator(strokeWidth: 2),
              )
            else ...[
              if (!isEncrypted && item.hasPayload())
                IconButton(
                  tooltip: l10n.chatScheduleEdit,
                  onPressed: onEdit,
                  icon: const Icon(Icons.edit_outlined),
                ),
              IconButton(
                tooltip: l10n.chatScheduleSendNow,
                onPressed: onSendNow,
                icon: const Icon(Icons.send_outlined),
              ),
              IconButton(
                tooltip: l10n.chatScheduleCancel,
                onPressed: onCancel,
                icon: const Icon(Icons.close),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

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

class _PendingAttachmentUpload {
  const _PendingAttachmentUpload({
    required this.file,
    required this.bytes,
    required this.mimeType,
    required this.authorization,
    required this.chatId,
    required this.chatType,
    required this.isE2e,
    required this.e2eKeyWire,
    required this.imagesOnly,
  });

  final ChatAttachmentFile file;
  final Uint8List bytes;
  final String mimeType;
  final String authorization;
  final String chatId;
  final String? chatType;
  final bool isE2e;
  final String? e2eKeyWire;
  final bool imagesOnly;

  _PendingAttachmentUpload withAuthorization(String value) =>
      _PendingAttachmentUpload(
        file: file,
        bytes: bytes,
        mimeType: mimeType,
        authorization: value,
        chatId: chatId,
        chatType: chatType,
        isE2e: isE2e,
        e2eKeyWire: e2eKeyWire,
        imagesOnly: imagesOnly,
      );
}

class _ChatRoomPanelState extends ConsumerState<ChatRoomPanel> {
  final _composer = TextEditingController();
  final _composerFocus = FocusNode();
  final _selectedMessageFocus = FocusNode(
    debugLabel: 'chat-room-selected-message',
  );
  final _attachFocus = FocusNode();
  final _emojiFocus = FocusNode();
  final _scrollController = ScrollController();
  final _inChatSearchTriggerFocus = FocusNode(
    debugLabel: 'chat-room-search-trigger',
  );
  final _inChatSearchFocus = FocusNode(debugLabel: 'chat-room-search-field');
  var _uploadingAttachment = false;
  var _attachmentOperation = 0;
  _PendingAttachmentUpload? _pendingAttachmentUpload;
  FilesApiFailure? _attachmentUploadFailure;
  var _initialUnreadCount = 0;
  var _unreadCaptured = false;
  var _pendingNewMessages = 0;
  var _wasNearBottom = true;
  var _slashMenuOpen = false;
  var _executingSlash = false;
  var _inChatSearchOpen = false;
  var _chatInfoSearchHandoffGeneration = 0;
  var _pinnedBarHidden = false;
  var _scheduledMessagesRequested = false;
  _ScheduledSendAttempt? _pendingScheduledSend;
  var _scheduledAttemptInFlight = false;
  var _scheduledAttemptSubmissionGeneration = 0;
  Future<void> Function()? _scheduledActionRetry;
  final Set<String> _scheduledActionBusyIds = <String>{};
  var _pinnedJumpGeneration = 0;
  String? _shownPinnedMessageId;
  var _highlightedMessageId = null as String?;
  var _liveMessageAnnouncement = '';
  ChatDraftKey? _draftKey;
  final _inChatSearchController = TextEditingController();

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScroll);
  }

  @override
  void dispose() {
    _pinnedJumpGeneration++;
    _attachmentOperation++;
    _scheduledAttemptSubmissionGeneration++;
    _pendingAttachmentUpload = null;
    _attachmentUploadFailure = null;
    _composer.dispose();
    _composerFocus.dispose();
    _selectedMessageFocus.dispose();
    _attachFocus.dispose();
    _emojiFocus.dispose();
    _inChatSearchTriggerFocus.dispose();
    _inChatSearchFocus.dispose();
    _scrollController.dispose();
    _inChatSearchController.dispose();
    super.dispose();
  }

  @override
  void didUpdateWidget(covariant ChatRoomPanel oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.chatId != widget.chatId) {
      _chatInfoSearchHandoffGeneration++;
      _inChatSearchOpen = false;
      _inChatSearchController.clear();
      _inChatSearchTriggerFocus.unfocus();
      _pinnedJumpGeneration++;
      _pinnedBarHidden = false;
      _shownPinnedMessageId = null;
      _scheduledMessagesRequested = false;
      _scheduledAttemptSubmissionGeneration++;
      _scheduledAttemptInFlight = false;
      _pendingScheduledSend = null;
      _scheduledActionRetry = null;
      _scheduledActionBusyIds.clear();
      _attachmentOperation++;
      _pendingAttachmentUpload = null;
      _attachmentUploadFailure = null;
      _uploadingAttachment = false;
      _draftKey = null;
      _composer.clear();
      _initialUnreadCount = 0;
      _unreadCaptured = false;
    }
  }

  String _roomErrorText(AppLocalizations l10n, String raw) {
    return chatRoomErrorMessage(l10n, raw);
  }

  void _refocusComposer() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      _composerFocus.requestFocus();
    });
  }

  void _closeInChatSearch() {
    _chatInfoSearchHandoffGeneration++;
    if (!_inChatSearchOpen) return;
    setState(() {
      _inChatSearchOpen = false;
      _inChatSearchController.clear();
    });
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted && !_inChatSearchOpen) {
        _inChatSearchTriggerFocus.requestFocus();
      }
    });
  }

  void _openInChatSearch() {
    _chatInfoSearchHandoffGeneration++;
    setState(() => _inChatSearchOpen = true);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted &&
          _inChatSearchOpen &&
          ref.read(selectedChatIdProvider) == widget.chatId) {
        _inChatSearchFocus.requestFocus();
      }
    });
  }

  bool _isCurrentChatInfoSearchHandoff(
    ChatInfoSearchRequest request,
    int generation,
  ) =>
      mounted &&
      _chatInfoSearchHandoffGeneration == generation &&
      widget.chatId == request.chatId &&
      ref.read(selectedChatIdProvider) == request.chatId &&
      ref.read(authControllerProvider).activeProfileId ==
          request.viewerProfileId;

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

  Future<bool> _scrollToMessage(
    String messageId, {
    bool Function()? isCancelled,
  }) async {
    final generation = ++_pinnedJumpGeneration;
    final controller = ref.read(
      chatRoomControllerProvider(widget.chatId).notifier,
    );
    var room = ref.read(chatRoomControllerProvider(widget.chatId));
    if (!room.messages.any((message) => message.id == messageId)) {
      final found = await controller.loadHistoryUntilMessage(
        messageId,
        isCancelled: () =>
            !mounted ||
            generation != _pinnedJumpGeneration ||
            (isCancelled?.call() ?? false),
      );
      if (!found) return false;
      room = ref.read(chatRoomControllerProvider(widget.chatId));
    }
    if (!mounted ||
        generation != _pinnedJumpGeneration ||
        (isCancelled?.call() ?? false)) {
      return false;
    }
    final index = room.messages.indexWhere(
      (message) => message.id == messageId,
    );
    if (index < 0) return false;
    final completed = Completer<bool>();
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      if (!mounted ||
          generation != _pinnedJumpGeneration ||
          (isCancelled?.call() ?? false) ||
          !_scrollController.hasClients) {
        completed.complete(false);
        return;
      }
      final max = _scrollController.position.maxScrollExtent;
      final count = room.messages.length;
      final target = count <= 1 ? max : max * (index / (count - 1));
      try {
        await _scrollController.animateTo(
          target,
          duration: const Duration(milliseconds: 250),
          curve: Curves.easeOut,
        );
        completed.complete(
          mounted &&
              generation == _pinnedJumpGeneration &&
              !(isCancelled?.call() ?? false),
        );
      } catch (_) {
        completed.complete(false);
      }
    });
    return completed.future;
  }

  void _handlePendingPinnedMessageJump(PendingPinnedMessageJump request) {
    unawaited(() async {
      final opened = await _scrollToMessage(
        request.messageId,
        isCancelled: () => request.cancelled,
      );
      request.complete(opened);
      if (!mounted) return;
      final pending = ref.read(pendingPinnedMessageJumpProvider(widget.chatId));
      if (identical(pending, request)) {
        ref
                .read(pendingPinnedMessageJumpProvider(widget.chatId).notifier)
                .state =
            null;
      }
    }());
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final activeId = ref.watch(authControllerProvider).activeProfileId;
    ref.listen<AuthState>(authControllerProvider, (previous, next) {
      final changed =
          previous?.activeProfileId != next.activeProfileId ||
          previous?.session?.accessToken != next.session?.accessToken;
      if (!changed) return;
      if (mounted) {
        setState(() {
          _scheduledAttemptSubmissionGeneration++;
          _scheduledAttemptInFlight = false;
          _pendingScheduledSend = null;
          _scheduledActionRetry = null;
        });
      }
      _scheduledMessagesRequested = false;
      if (next.activeProfileId != null &&
          ref.read(selectedChatIdProvider) == widget.chatId) {
        _scheduledMessagesRequested = true;
        unawaited(
          ref
              .read(chatRoomControllerProvider(widget.chatId).notifier)
              .loadScheduledMessages(),
        );
      }
    });
    ref.listen<String?>(selectedChatIdProvider, (_, next) {
      if (next != widget.chatId || _scheduledMessagesRequested) return;
      _scheduledMessagesRequested = true;
      unawaited(
        ref
            .read(chatRoomControllerProvider(widget.chatId).notifier)
            .loadScheduledMessages(),
      );
    });
    if (!_scheduledMessagesRequested &&
        activeId != null &&
        ref.read(selectedChatIdProvider) == widget.chatId) {
      _scheduledMessagesRequested = true;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        unawaited(
          ref
              .read(chatRoomControllerProvider(widget.chatId).notifier)
              .loadScheduledMessages(),
        );
      });
    }
    final draftKey = activeId == null
        ? null
        : ChatDraftKey(profileId: activeId, chatId: widget.chatId);
    if (_draftKey != draftKey) {
      _draftKey = draftKey;
      _composer.clear();
    }
    if (draftKey != null) {
      ref.listen<AsyncValue<String>>(chatDraftProvider(draftKey), (_, next) {
        final restored = next.valueOrNull;
        if (restored != null && _composer.text != restored) {
          _composer.value = TextEditingValue(
            text: restored,
            selection: TextSelection.collapsed(offset: restored.length),
          );
        }
      });
      ref.watch(chatDraftProvider(draftKey));
    }
    final controllerRoom = ref.watch(chatRoomControllerProvider(widget.chatId));
    final historyBelongsToActiveProfile =
        controllerRoom.historyProfileId == activeId;
    final room = historyBelongsToActiveProfile
        ? controllerRoom
        : ChatRoomState(
            isLoading: controllerRoom.isLoading,
            errorMessage: controllerRoom.errorMessage,
            realtimeStatus: controllerRoom.realtimeStatus,
          );
    final pinnedMessages = room.pinnedMessages;
    final shownPinIndex = pinnedMessages.indexWhere(
      (message) => message.id == _shownPinnedMessageId,
    );
    final currentPinnedIndex = shownPinIndex < 0 ? 0 : shownPinIndex;
    final currentPinnedMessage = pinnedMessages.isEmpty
        ? null
        : pinnedMessages[currentPinnedIndex];
    final deviceOffline = ref.watch(isDeviceOfflineProvider);
    final isOffline = deviceOffline || room.isOfflineCache;
    final reconnectBanner = ref.watch(reconnectBannerVisibleProvider);
    final canRetryBanner =
        !isOffline &&
        !reconnectBanner.retrying &&
        ref.read(realtimeHubProvider).canRetryCurrentSession;
    final canCall = ref.watch(gatewayConfigProvider).canPlaceVoiceCalls;
    final isGuest = ref.watch(authControllerProvider).isGuest;
    String? groupName;
    String? spaceId;
    var slowModeSeconds = 0;
    var isGroup = false;
    VoiceChat? chatMeta;
    ChatListItem? chatListItem;
    for (final item in ref.watch(chatListControllerProvider).items) {
      if (item.chatId == widget.chatId) {
        chatMeta = item.chat;
        chatListItem = item;
        if (item.chat.isGroup) {
          groupName = item.chat.name;
          spaceId = item.chat.spaceId;
          slowModeSeconds = item.chat.slowModeSeconds;
          isGroup = true;
        }
        break;
      }
    }
    final replyTarget = ref.watch(chatReplyTargetProvider(widget.chatId));
    final activeThreadId = ref.watch(chatActiveThreadProvider(widget.chatId));
    final ephemeralMessages = ref.watch(
      ephemeralMessagesProvider(widget.chatId),
    );
    final deferredInteraction = ref.watch(
      deferredBotInteractionProvider(widget.chatId),
    );
    final blockChannelMainFeed =
        chatMeta?.isChannel == true && chatMeta!.allowUserMainFeed == false;
    final composerBlocked =
        isOffline ||
        room.isDmPeerDeleted ||
        (blockChannelMainFeed && replyTarget == null);
    final chatListState = ref.watch(chatListControllerProvider);
    final roomBelongsToViewer =
        activeId != null && room.historyProfileId == activeId;
    final listBelongsToViewer =
        activeId != null && chatListState.profileId == activeId;
    final peerId = isGroup
        ? null
        : resolveDmPeerForChatId(
            chatId: widget.chatId,
            knownPeers: const {},
            listItems: listBelongsToViewer ? chatListState.items : const [],
            activeProfileId: activeId,
            messages: roomBelongsToViewer ? room.messages : const [],
          );
    final peerProfile = peerId != null
        ? ref.watch(profileProvider(peerId)).valueOrNull
        : null;
    final peerName =
        peerProfile?.displayName ?? chatListItem?.dmPeerDisplayName;
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
        : (peerName ?? groupName ?? l10n.socialProfileUnavailable);
    final peerIsPremium =
        peerId != null && ref.watch(profilePremiumBadgeProvider(peerId));
    final voice = VoiceColors.of(context);
    final headerTitle = _inChatSearchOpen
        ? KeyedSubtree(
            key: const Key('in_chat_search_inline_header'),
            child: TextField(
              key: InChatSearch.searchFieldKey,
              controller: _inChatSearchController,
              focusNode: _inChatSearchFocus,
              autofocus: true,
              decoration: InputDecoration(
                hintText: l10n.inChatSearchHint,
                isDense: true,
                border: InputBorder.none,
              ),
              onChanged: (_) => setState(() {}),
            ),
          )
        : !isGroup && peerProfile != null
        ? ChatAuthorLabel(
            displayName: title,
            isPremium: peerIsPremium,
            verificationType: peerProfile.verificationType,
            style: Theme.of(context).textTheme.titleMedium,
            premiumBadgeSemanticLabel: l10n.premiumBadgeLabel,
            verifiedBadgeSemanticLabel:
                peerProfile.verificationType == 'organization'
                ? l10n.verifiedBadgeOrganization
                : l10n.verifiedBadgePersonal,
          )
        : Text(title, style: Theme.of(context).textTheme.titleMedium);
    final headerLeading = <Widget>[
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
    ];
    final headerActions = <Widget>[
      IconButton(
        key: ChatRoomPanel.inChatSearchKey,
        focusNode: _inChatSearchTriggerFocus,
        tooltip: l10n.inChatSearchOpen,
        onPressed: () {
          if (_inChatSearchOpen) {
            _closeInChatSearch();
          } else {
            _openInChatSearch();
          }
        },
        icon: Icon(_inChatSearchOpen ? Icons.close : Icons.search),
      ),
      IconButton(
        key: ChatRoomPanel.chatInfoKey,
        tooltip: l10n.chatInfoOpen,
        onPressed: () => openChatInfoPanel(
          context,
          ref,
          chatId: widget.chatId,
          groupName: groupName,
          isGroup: isGroup,
        ),
        icon: const Icon(Icons.info_outline),
      ),
      if (pinnedMessages.isNotEmpty && _pinnedBarHidden)
        IconButton(
          key: ChatRoomPanel.pinnedMessagesHeaderKey,
          tooltip: l10n.chatPinnedMessagesRestore,
          onPressed: () => setState(() => _pinnedBarHidden = false),
          icon: const Icon(Icons.push_pin_outlined),
        ),
      if (shareUrlForChat(chatId: widget.chatId, spaceId: spaceId) != null)
        VoiceShareLinkButton(
          link: shareUrlForChat(chatId: widget.chatId, spaceId: spaceId)!,
          tooltip: l10n.shareLinkAction,
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
          onPressed: () => openChatInfoPanel(
            context,
            ref,
            chatId: widget.chatId,
            groupName: groupName,
            isGroup: true,
          ),
          icon: const Icon(Icons.group_outlined),
        ),
      ],
      if (!isGroup && peerId != null && canCall) ...[
        IconButton(
          key: ChatRoomPanel.audioCallKey,
          tooltip: l10n.callStartAudio,
          onPressed: isGuest
              ? null
              : () => ref
                    .read(callControllerProvider.notifier)
                    .startCall(chatId: widget.chatId, calleeProfileId: peerId),
          icon: const Icon(Icons.call_outlined),
        ),
        IconButton(
          key: ChatRoomPanel.videoCallKey,
          tooltip: l10n.callStartVideo,
          onPressed: isGuest
              ? null
              : () => ref
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
    ];

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

    ref.listen(pendingChatMessageScrollProvider(widget.chatId), (prev, next) {
      if (next != null && next.isNotEmpty) {
        unawaited(
          _scrollToMessage(next).then((_) {
            if (!mounted) return;
            final pending = ref.read(
              pendingChatMessageScrollProvider(widget.chatId),
            );
            if (pending == next) {
              ref
                      .read(
                        pendingChatMessageScrollProvider(
                          widget.chatId,
                        ).notifier,
                      )
                      .state =
                  null;
            }
          }),
        );
      }
    });

    ref.listen(pendingPinnedMessageJumpProvider(widget.chatId), (prev, next) {
      if (next == null) return;
      _handlePendingPinnedMessageJump(next);
    });

    ref.listen(pendingChatMessageHighlightProvider(widget.chatId), (
      prev,
      next,
    ) {
      if (next != null && next.isNotEmpty) {
        setState(() => _highlightedMessageId = next);
        Future<void>.delayed(const Duration(seconds: 2), () {
          if (mounted && _highlightedMessageId == next) {
            setState(() => _highlightedMessageId = null);
          }
        });
        ref
                .read(
                  pendingChatMessageHighlightProvider(widget.chatId).notifier,
                )
                .state =
            null;
      }
    });

    ref.listen(composerFocusRequestProvider, (prev, next) {
      if (next != (prev ?? 0)) {
        _refocusComposer();
      }
    });

    ref.listen<ChatInfoSearchRequest?>(chatInfoSearchRequestProvider, (
      previous,
      request,
    ) {
      if (request == null) return;
      final isCurrentContext =
          ref.read(selectedChatIdProvider) == request.chatId &&
          ref.read(authControllerProvider).activeProfileId ==
              request.viewerProfileId;
      if (!isCurrentContext) {
        if (identical(ref.read(chatInfoSearchRequestProvider), request)) {
          ref.read(chatInfoSearchRequestProvider.notifier).state = null;
        }
        return;
      }
      if (widget.chatId != request.chatId || !mounted) return;
      if (identical(ref.read(chatInfoSearchRequestProvider), request)) {
        ref.read(chatInfoSearchRequestProvider.notifier).state = null;
      }
      final handoffGeneration = ++_chatInfoSearchHandoffGeneration;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!_isCurrentChatInfoSearchHandoff(request, handoffGeneration)) {
          return;
        }
        setState(() => _inChatSearchOpen = true);
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (_isCurrentChatInfoSearchHandoff(request, handoffGeneration) &&
              _inChatSearchOpen) {
            _inChatSearchFocus.requestFocus();
          }
        });
      });
    });

    ref.listen<String?>(chatMessageKeyboardProvider, (previous, next) {
      if (next == null || !room.messages.any((message) => message.id == next)) {
        return;
      }
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted ||
            ref.read(selectedChatIdProvider) != widget.chatId ||
            ref.read(chatMessageKeyboardProvider) != next) {
          return;
        }
        _selectedMessageFocus.requestFocus();
      });
    });

    ref.listen(chatMessageReactionRequestProvider(widget.chatId), (prev, next) {
      if (next == null || next.isEmpty) return;
      final messages = ref
          .read(chatRoomControllerProvider(widget.chatId))
          .messages;
      VoiceMessage? message;
      for (final item in messages) {
        if (item.id == next) {
          message = item;
          break;
        }
      }
      ref
              .read(chatMessageReactionRequestProvider(widget.chatId).notifier)
              .state =
          null;
      if (message != null) {
        unawaited(
          _showMessageActions(message, message.senderProfileId == activeId),
        );
      }
    });

    ref.listen(chatMessageContextMenuRequestProvider(widget.chatId), (
      prev,
      next,
    ) {
      if (next == null || next.isEmpty) return;
      final messages = ref
          .read(chatRoomControllerProvider(widget.chatId))
          .messages;
      VoiceMessage? message;
      for (final item in messages) {
        if (item.id == next) {
          message = item;
          break;
        }
      }
      ref
              .read(
                chatMessageContextMenuRequestProvider(widget.chatId).notifier,
              )
              .state =
          null;
      if (message != null) {
        unawaited(
          _showMessageActions(message, message.senderProfileId == activeId),
        );
      }
    });

    ref.listen(chatRoomControllerProvider(widget.chatId), (prev, next) {
      final prevLen = prev?.messages.length ?? 0;
      if (next.messages.length > prevLen) {
        final added = next.messages.length - prevLen;
        final latest = next.messages.last;
        setState(() {
          _liveMessageAnnouncement =
              '${latest.senderProfileId}: ${latest.content}';
        });
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
      _composer.selection = TextSelection.collapsed(
        offset: _composer.text.length,
      );
      ref.read(pendingComposerEmojiProvider.notifier).state = null;
      _composerFocus.requestFocus();
    });

    return Row(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Expanded(
          child: Column(
            key: ChatRoomPanel.panelKey,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Semantics(
                liveRegion: true,
                label: _liveMessageAnnouncement,
                child: const SizedBox.shrink(),
              ),
              Material(
                color: voice.surface,
                elevation: 0,
                child: Padding(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 12,
                    vertical: 8,
                  ),
                  child: LayoutBuilder(
                    builder: (context, constraints) {
                      if (constraints.maxWidth < 600) {
                        return Wrap(
                          crossAxisAlignment: WrapCrossAlignment.center,
                          children: [
                            ...headerLeading,
                            SizedBox(
                              width: constraints.maxWidth * 0.45,
                              child: headerTitle,
                            ),
                            ...headerActions,
                          ],
                        );
                      }
                      return Row(
                        children: [
                          ...headerLeading,
                          Expanded(child: headerTitle),
                          ...headerActions,
                        ],
                      );
                    },
                  ),
                ),
              ),
              if (!reconnectBanner.dismissed &&
                  ((reconnectBanner.visible && widget.showReconnectBanner) ||
                      room.isOfflineCache))
                VoiceCompactBanner(
                  key: isOffline
                      ? ChatRoomPanel.offlineBannerKey
                      : ChatRoomPanel.reconnectBannerKey,
                  message: isOffline
                      ? (room.isOfflineCache
                            ? l10n.chatOfflineReadOnly
                            : l10n.chatRealtimeOffline)
                      : l10n.chatRealtimeReconnecting,
                  detail: isOffline
                      ? l10n.chatOfflineSendBlocked
                      : l10n.networkReconnectDetails,
                  icon: isOffline
                      ? Icons.cloud_off_outlined
                      : Icons.sync_problem,
                  actionLabel: canRetryBanner ? l10n.commonRetry : null,
                  onAction: !canRetryBanner
                      ? null
                      : () => ref
                            .read(reconnectBannerVisibleProvider.notifier)
                            .retry(),
                  onDismiss: () => ref
                      .read(reconnectBannerVisibleProvider.notifier)
                      .dismiss(),
                  networkLayout:
                      reconnectBanner.visible && widget.showReconnectBanner
                      ? VoiceLayout.isNarrow(MediaQuery.sizeOf(context).width)
                            ? VoiceNetworkBannerLayout.phone
                            : VoiceNetworkBannerLayout.desktop
                      : null,
                  isReconnecting:
                      !deviceOffline &&
                      !room.isOfflineCache &&
                      reconnectBanner.visible &&
                      widget.showReconnectBanner,
                  tone: VoiceBannerTone.warning,
                ),
              if (room.pinnedMessages.isEmpty &&
                  room.pinnedMessagesStatus == PinnedMessagesLoadStatus.loading)
                const LinearProgressIndicator(
                  key: Key('chat_room_pins_loading'),
                  minHeight: 2,
                ),
              if (room.pinnedMessagesStatus == PinnedMessagesLoadStatus.failed)
                Builder(
                  builder: (context) {
                    final message = commonActionErrorMessage(
                      l10n,
                      statusCode: room.pinnedMessagesErrorStatusCode,
                    );
                    final voice = VoiceColors.of(context);
                    return Material(
                      key: const Key('chat_room_pins_error'),
                      color: voice.error.withValues(alpha: 0.15),
                      child: Padding(
                        padding: const EdgeInsets.symmetric(horizontal: 12),
                        child: Row(
                          children: [
                            Icon(
                              Icons.cloud_off_outlined,
                              size: 16,
                              color: voice.error,
                            ),
                            const SizedBox(width: 6),
                            Expanded(
                              child: Tooltip(
                                message: message,
                                child: Semantics(
                                  label: message,
                                  child: ExcludeSemantics(
                                    child: Text(
                                      message,
                                      maxLines: 2,
                                      overflow: TextOverflow.ellipsis,
                                      style: Theme.of(context)
                                          .textTheme
                                          .bodySmall
                                          ?.copyWith(color: voice.error),
                                    ),
                                  ),
                                ),
                              ),
                            ),
                            TextButton(
                              onPressed: () => ref
                                  .read(
                                    chatRoomControllerProvider(
                                      widget.chatId,
                                    ).notifier,
                                  )
                                  .retryPinnedMessages(),
                              style: TextButton.styleFrom(
                                minimumSize: const Size(48, 48),
                                padding: const EdgeInsets.symmetric(
                                  horizontal: 8,
                                ),
                              ),
                              child: Text(l10n.commonRetry),
                            ),
                          ],
                        ),
                      ),
                    );
                  },
                ),
              if (peerId != null &&
                  ref
                      .watch(e2eIdentityTrustProvider)
                      .pendingKeyChangePeers
                      .contains(peerId))
                E2eIdentityChangeBanner(
                  peerDisplayName: peerName ?? '@$shortId',
                  onContinue: () {
                    final bundleFuture = ref
                        .read(voiceE2eClientProvider)
                        .getPreKeyBundle(
                          authorization: ref.read(authorizationHeaderProvider)!,
                          profileId: peerId,
                        );
                    unawaited(
                      bundleFuture.then((result) {
                        if (result is! E2eApiOk<String>) return;
                        final parsed = parseSerializedPreKeyBundle(result.data);
                        if (parsed == null) return;
                        ref
                            .read(e2eIdentityTrustProvider.notifier)
                            .acceptKeyChange(
                              peerId,
                              identityKeyBytesFromSerialized(
                                parsed.getIdentityKey().serialize(),
                              ),
                            );
                      }),
                    );
                  },
                  onDistrust: () {
                    ref
                        .read(e2eIdentityTrustProvider.notifier)
                        .distrustPeer(peerId);
                  },
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
              if (currentPinnedMessage != null && !_pinnedBarHidden)
                _PinnedMessagesBar(
                  key: ChatRoomPanel.pinnedBarKey,
                  message: currentPinnedMessage,
                  label: l10n.chatPinnedBar(pinnedMessages.length),
                  contentTypeLabel: pinnedMessageContentTypeLabel(
                    l10n,
                    currentPinnedMessage.contentType,
                  ),
                  onTap: () {
                    unawaited(_scrollToMessage(currentPinnedMessage.id));
                    if (pinnedMessages.length > 1) {
                      setState(() {
                        _shownPinnedMessageId =
                            pinnedMessages[(currentPinnedIndex + 1) %
                                    pinnedMessages.length]
                                .id;
                      });
                    }
                  },
                  onOpenAll: () => PinnedMessagesPanel.show(
                    context,
                    chatId: widget.chatId,
                    spaceId: spaceId,
                    isGroup: isGroup,
                    messages: pinnedMessages,
                    onOpenMessage: _scrollToMessage,
                    onUnpin: (messageId) => ref
                        .read(
                          chatRoomControllerProvider(widget.chatId).notifier,
                        )
                        .togglePinWithResult(messageId, currentlyPinned: true),
                    onCancel: () => _pinnedJumpGeneration++,
                  ),
                  onHide: () => setState(() => _pinnedBarHidden = true),
                ),
              if (room.isDmPeerDeleted && room.messages.isNotEmpty)
                Padding(
                  key: ValueKey<String>('chat_room_dm_peer_deleted'),
                  padding: const EdgeInsets.symmetric(
                    horizontal: 12,
                    vertical: 8,
                  ),
                  child: Text(l10n.chatDmPeerDeleted),
                ),
              Expanded(
                child: Stack(
                  children: [
                    room.isLoading &&
                            room.messages.isEmpty &&
                            ephemeralMessages.isEmpty
                        ? const VoiceListSkeleton(rowCount: 4)
                        : room.messages.isEmpty &&
                              ephemeralMessages.isEmpty &&
                              !room.isLoading
                        ? room.errorMessage != null
                              ? VoiceStatePanel(
                                  title: _roomErrorText(
                                    l10n,
                                    room.errorMessage!,
                                  ),
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
                        : Focus(
                            key: ChatRoomPanel.selectedMessageFocusKey,
                            focusNode: _selectedMessageFocus,
                            onKeyEvent: (node, event) {
                              if (!node.hasPrimaryFocus ||
                                  event is! KeyDownEvent ||
                                  event.logicalKey !=
                                      LogicalKeyboardKey.enter) {
                                return KeyEventResult.ignored;
                              }
                              final selectedId = ref.read(
                                chatMessageKeyboardProvider,
                              );
                              if (selectedId == null ||
                                  ref.read(selectedChatIdProvider) !=
                                      widget.chatId ||
                                  !room.messages.any(
                                    (message) => message.id == selectedId,
                                  )) {
                                return KeyEventResult.ignored;
                              }
                              ref
                                  .read(chatMessageKeyboardProvider.notifier)
                                  .openContextMenuOnSelected();
                              return KeyEventResult.handled;
                            },
                            child: _MessageListView(
                              key: ChatRoomPanel.messagesKey,
                              chatId: widget.chatId,
                              scrollController: _scrollController,
                              room: room,
                              ephemeralMessages: ephemeralMessages,
                              deferredInteraction: deferredInteraction,
                              activeId: activeId,
                              isGroup: isGroup,
                              l10n: l10n,
                              initialUnreadCount: _initialUnreadCount,
                              highlightedMessageId: _highlightedMessageId,
                              keyboardSelectedMessageId: ref.watch(
                                chatMessageKeyboardProvider,
                              ),
                              chatTheme: ref.watch(
                                effectiveChatThemeProvider(widget.chatId),
                              ),
                              onLongPress: (msg, isMine) =>
                                  _showMessageActions(msg, isMine),
                            ),
                          ),
                    if (_inChatSearchOpen)
                      Positioned(
                        top: 0,
                        left: 0,
                        right: 0,
                        child: Material(
                          elevation: 4,
                          color: voice.surface,
                          child: InChatSearch(
                            chatId: widget.chatId,
                            controller: _inChatSearchController,
                            showSearchField: false,
                            onActiveMessageChanged: _scrollToMessage,
                            onDismiss: _closeInChatSearch,
                          ),
                        ),
                      ),
                  ],
                ),
              ),
              if (room.errorMessage != null && room.messages.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 12),
                  child: Text(
                    _roomErrorText(l10n, room.errorMessage!),
                    style: TextStyle(
                      color: Theme.of(context).colorScheme.error,
                    ),
                  ),
                ),
              if (replyTarget != null)
                Material(
                  color: Theme.of(context).colorScheme.surfaceContainerHighest,
                  child: ListTile(
                    dense: true,
                    title: Text(
                      l10n.chatReplyingTo(
                        replyTarget.content.trim().isEmpty
                            ? '…'
                            : replyTarget.content.trim(),
                      ),
                    ),
                    trailing: IconButton(
                      icon: const Icon(Icons.close, size: 18),
                      onPressed: () {
                        ref
                                .read(
                                  chatReplyTargetProvider(
                                    widget.chatId,
                                  ).notifier,
                                )
                                .state =
                            null;
                      },
                    ),
                  ),
                ),
              _buildScheduledMessagesSection(room, l10n),
              if (_attachmentUploadFailure case final failure?)
                Padding(
                  key: ChatRoomPanel.attachmentUploadFailureKey,
                  padding: const EdgeInsets.fromLTRB(12, 0, 12, 4),
                  child: Row(
                    children: [
                      Expanded(
                        child: VoiceCompactBanner(
                          key: failure.errorCode == 'file_infected'
                              ? null
                              : ChatRoomPanel.attachmentUploadRetryKey,
                          message: _attachmentFailureMessage(
                            AppLocalizations.of(context)!,
                            failure,
                          ),
                          icon: Icons.cloud_off_outlined,
                          actionLabel: failure.errorCode == 'file_infected'
                              ? AppLocalizations.of(
                                  context,
                                )!.chatAttachmentPickAnother
                              : AppLocalizations.of(context)!.commonRetry,
                          onAction: failure.errorCode == 'file_infected'
                              ? _pickAnotherAttachment
                              : () => unawaited(_retryAttachmentUpload()),
                          tone: VoiceBannerTone.error,
                        ),
                      ),
                      IconButton(
                        key: ChatRoomPanel.attachmentUploadCancelKey,
                        tooltip: AppLocalizations.of(context)!.commonCancel,
                        onPressed: _cancelPendingAttachmentUpload,
                        icon: const Icon(Icons.close),
                      ),
                    ],
                  ),
                ),
              SafeArea(
                top: false,
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(12, 4, 12, 8),
                  child: Row(
                    children: [
                      Focus(
                        focusNode: _emojiFocus,
                        child: IconButton(
                          key: ChatRoomPanel.emojiPickerKey,
                          tooltip: l10n.composerEmojiPanelTitle,
                          onPressed: room.isSending || composerBlocked
                              ? null
                              : () => openEmojiPanel(
                                  context,
                                  ref,
                                  onSelected: (emoji) {
                                    ref
                                            .read(
                                              pendingComposerEmojiProvider
                                                  .notifier,
                                            )
                                            .state =
                                        emoji;
                                  },
                                  onDismiss: () => _emojiFocus.requestFocus(),
                                ),
                          icon: const Icon(Icons.emoji_emotions_outlined),
                        ),
                      ),
                      Expanded(
                        child: ChatComposerTextField(
                          key: ChatRoomPanel.inputKey,
                          controller: _composer,
                          focusNode: _composerFocus,
                          decoration: InputDecoration(
                            hintText:
                                blockChannelMainFeed && replyTarget == null
                                ? l10n.chatChannelMainFeedBlocked
                                : l10n.chatRoomInputHint,
                            isDense: true,
                          ),
                          onChanged: (value) {
                            final key = _draftKey;
                            if (key != null) {
                              ref
                                  .read(chatDraftProvider(key).notifier)
                                  .update(value);
                            }
                            final hub = ref.read(realtimeHubProvider);
                            if (value.trim().isEmpty) {
                              hub.typingStop(widget.chatId);
                            } else {
                              hub.typingStart(widget.chatId);
                            }
                            if (!_slashMenuOpen &&
                                !_executingSlash &&
                                !composerBlocked &&
                                (value == '/' || value.endsWith(' /'))) {
                              _slashMenuOpen = true;
                              unawaited(
                                _showSlashCommandMenu(context).whenComplete(() {
                                  if (mounted) {
                                    setState(() => _slashMenuOpen = false);
                                  }
                                }),
                              );
                            }
                          },
                          onSend: room.isSending || composerBlocked
                              ? null
                              : _send,
                          readOnly: composerBlocked,
                        ),
                      ),
                      const SizedBox(width: 8),
                      Focus(
                        focusNode: _attachFocus,
                        child: IconButton(
                          key: ChatRoomPanel.attachKey,
                          tooltip: l10n.chatAttachFile,
                          onPressed:
                              room.isSending ||
                                  _uploadingAttachment ||
                                  composerBlocked
                              ? null
                              : _openAttachMenu,
                          icon: _uploadingAttachment
                              ? const SizedBox(
                                  width: 18,
                                  height: 18,
                                  child: CircularProgressIndicator(
                                    strokeWidth: 2,
                                  ),
                                )
                              : const Icon(Icons.attach_file),
                        ),
                      ),
                      const SizedBox(width: 4),
                      VoiceSendButton(
                        key: ChatRoomPanel.sendKey,
                        onPressed: room.isSending || composerBlocked
                            ? null
                            : _send,
                        onLongPress:
                            room.isSending ||
                                _uploadingAttachment ||
                                composerBlocked
                            ? null
                            : _openScheduledSendMenu,
                        isLoading: room.isSending,
                        tooltip: l10n.chatSendMessage,
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
        if (activeThreadId != null)
          SizedBox(
            width: 320,
            child: ThreadSidePanel(
              chatId: widget.chatId,
              parentMessageId: activeThreadId,
              parentPreview:
                  replyTarget?.content ??
                  room.messages
                      .where((m) => m.id == activeThreadId)
                      .map((m) => m.content)
                      .firstOrNull ??
                  '',
              onClose: () {
                ref
                        .read(chatActiveThreadProvider(widget.chatId).notifier)
                        .state =
                    null;
                ref
                        .read(chatReplyTargetProvider(widget.chatId).notifier)
                        .state =
                    null;
              },
            ),
          ),
      ],
    );
  }

  Future<void> _showSlashCommandMenu(BuildContext context) async {
    final l10n = AppLocalizations.of(context)!;
    final text = _composer.text;
    final slashIndex = text.lastIndexOf('/');
    final filter = slashIndex >= 0 ? text.substring(slashIndex + 1) : '';

    await showSlashCommandMenu(
      context: context,
      ref: ref,
      chatId: widget.chatId,
      filter: filter,
      onSelected: (command) async {
        if (slashIndex >= 0) {
          final prefix = text.substring(0, slashIndex).trimRight();
          _composer.text = prefix;
          _composer.selection = TextSelection.collapsed(
            offset: _composer.text.length,
          );
        } else {
          _composer.clear();
        }

        setState(() => _executingSlash = true);
        final messenger = ScaffoldMessenger.of(context);
        try {
          Map<String, dynamic> options = const {};
          if (command.options.isNotEmpty) {
            final collected = await showSlashCommandOptionsSheet(
              context: context,
              ref: ref,
              chatId: widget.chatId,
              command: command,
            );
            if (!mounted) return;
            if (collected == null) {
              return;
            }
            options = collected;
          }
          final failure = await ref
              .read(slashInteractionExecutorProvider)
              .execute(
                chatId: widget.chatId,
                command: command,
                optionsJson: jsonEncode(options),
              );
          if (!mounted) return;
          if (failure == SlashInteractionFailure.botTimeout) {
            messenger.showSnackBar(
              SnackBar(content: Text(l10n.botTimeoutError)),
            );
          } else if (failure == SlashInteractionFailure.botUnavailable) {
            messenger.showSnackBar(
              SnackBar(content: Text(l10n.botUnavailableTooltip)),
            );
          } else if (failure == SlashInteractionFailure.requestFailed) {
            messenger.showSnackBar(
              SnackBar(content: Text(l10n.chatRoomError(command.name))),
            );
          } else {
            _scrollToBottom();
          }
        } finally {
          if (mounted) {
            setState(() => _executingSlash = false);
          }
        }
        _refocusComposer();
      },
    );
  }

  Future<void> _openScheduledSendMenu() async {
    final chatId = widget.chatId;
    if (_composer.text.trim().isEmpty ||
        _isDmPeerDeleted() ||
        !_isCurrentChat(chatId)) {
      return;
    }
    final l10n = AppLocalizations.of(context)!;
    final authorization = ref.read(authorizationHeaderProvider);
    final profileId = ref.read(authControllerProvider).activeProfileId;
    if (authorization == null || profileId == null) return;
    final chatType = ref
        .read(chatListProvider)
        .valueOrNull
        ?.items
        .where((item) => item.chatId == chatId)
        .map((item) => item.chat.type)
        .firstOrNull;
    final isDm = chatType == 'CHAT_TYPE_DM';
    final mode = await showModalBottomSheet<String>(
      context: context,
      builder: (sheetContext) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.schedule),
              title: Text(l10n.chatScheduleMessage),
              onTap: () => Navigator.pop(sheetContext, 'scheduled'),
            ),
            if (isDm)
              ListTile(
                leading: const Icon(Icons.notifications_active_outlined),
                title: Text(l10n.chatSendWhenOnline),
                onTap: () => Navigator.pop(sheetContext, 'online'),
              ),
          ],
        ),
      ),
    );
    if (!mounted ||
        mode == null ||
        widget.chatId != chatId ||
        !_isCurrentChat(chatId) ||
        profileId != ref.read(authControllerProvider).activeProfileId ||
        authorization != ref.read(authorizationHeaderProvider)) {
      return;
    }
    DateTime? scheduledAt;
    if (mode == 'scheduled') {
      scheduledAt = await _pickScheduledDateTime();
      if (!mounted ||
          scheduledAt == null ||
          widget.chatId != chatId ||
          !_isCurrentChat(chatId) ||
          profileId != ref.read(authControllerProvider).activeProfileId ||
          authorization != ref.read(authorizationHeaderProvider)) {
        return;
      }
    }
    final text = _composer.text;
    final mentions = _mentionsForComposer(text);
    final replyTarget = ref.read(chatReplyTargetProvider(chatId));
    final attempt = _ScheduledSendAttempt(
      chatId: chatId,
      profileId: profileId,
      authorization: authorization,
      clientMessageId: const Uuid().v4(),
      content: text,
      scheduledAt: scheduledAt,
      sendWhenOnline: mode == 'online',
      mentions: mentions,
      threadParentId: replyTarget?.id,
    );
    setState(() {
      _pendingScheduledSend = attempt;
      _scheduledActionRetry = null;
    });
    await _submitScheduledAttempt(attempt);
  }

  Future<DateTime?> _pickScheduledDateTime() async {
    final now = DateTime.now();
    final latest = now.add(const Duration(days: 365));
    var date = DateUtils.dateOnly(now);
    var time = TimeOfDay.fromDateTime(now.add(const Duration(minutes: 1)));
    String? validationError;
    return showDialog<DateTime>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setDialogState) {
          final localizations = AppLocalizations.of(context)!;
          return AlertDialog(
            title: Text(localizations.chatSchedulePickerTitle),
            content: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                OutlinedButton.icon(
                  icon: const Icon(Icons.calendar_month_outlined),
                  label: Text(
                    MaterialLocalizations.of(context).formatMediumDate(date),
                  ),
                  onPressed: () async {
                    final selected = await showDatePicker(
                      context: context,
                      initialDate: date,
                      firstDate: DateUtils.dateOnly(now),
                      lastDate: DateUtils.dateOnly(latest),
                    );
                    if (selected != null) {
                      setDialogState(() {
                        date = selected;
                        validationError = null;
                      });
                    }
                  },
                ),
                OutlinedButton.icon(
                  icon: const Icon(Icons.access_time),
                  label: Text(
                    MaterialLocalizations.of(context).formatTimeOfDay(time),
                  ),
                  onPressed: () async {
                    final selected = await showTimePicker(
                      context: context,
                      initialTime: time,
                    );
                    if (selected != null) {
                      setDialogState(() {
                        time = selected;
                        validationError = null;
                      });
                    }
                  },
                ),
                if (validationError != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 8),
                    child: Text(
                      validationError!,
                      style: TextStyle(
                        color: Theme.of(context).colorScheme.error,
                      ),
                    ),
                  ),
              ],
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(dialogContext),
                child: Text(localizations.commonCancel),
              ),
              FilledButton(
                onPressed: () {
                  final selected = DateTime(
                    date.year,
                    date.month,
                    date.day,
                    time.hour,
                    time.minute,
                  );
                  if (!selected.isAfter(now) || selected.isAfter(latest)) {
                    setDialogState(() {
                      validationError = localizations.chatScheduleDateInvalid;
                    });
                    return;
                  }
                  Navigator.pop(dialogContext, selected.toUtc());
                },
                child: Text(localizations.chatScheduleMessage),
              ),
            ],
          );
        },
      ),
    );
  }

  List<MessageMention> _mentionsForComposer(String text) {
    final memberIds = ref
        .read(groupMembersProvider(widget.chatId))
        .maybeWhen(
          data: (data) => data.members.map((member) => member.profileId),
          orElse: () => const <String>[],
        );
    final handles = <String, String>{};
    for (final id in memberIds) {
      final handle = ref.read(profileProvider(id)).valueOrNull?.handle;
      if (handle != null && handle.isNotEmpty) handles[handle] = id;
    }
    final mentions = parseMentionsFromContent(
      text,
      memberProfileIds: memberIds,
      handleToProfileId: handles,
    );
    final chatType = ref
        .read(chatListProvider)
        .valueOrNull
        ?.items
        .where((item) => item.chatId == widget.chatId)
        .map((item) => item.chat.type)
        .firstOrNull;
    return chatType == 'CHAT_TYPE_DM'
        ? mentions.where((mention) => mention.type == 'user').toList()
        : mentions;
  }

  Future<void> _submitScheduledAttempt(_ScheduledSendAttempt attempt) async {
    if (!mounted ||
        _scheduledAttemptInFlight ||
        attempt.chatId != widget.chatId ||
        attempt.profileId != ref.read(authControllerProvider).activeProfileId ||
        attempt.authorization != ref.read(authorizationHeaderProvider) ||
        !_isCurrentChat(attempt.chatId)) {
      if (mounted) setState(() => _pendingScheduledSend = null);
      return;
    }
    final submissionGeneration = ++_scheduledAttemptSubmissionGeneration;
    setState(() => _scheduledAttemptInFlight = true);
    ref.read(realtimeHubProvider).typingStop(widget.chatId);
    final replyTarget = ref.read(chatReplyTargetProvider(widget.chatId));
    try {
      final error = await ref
          .read(chatRoomControllerProvider(widget.chatId).notifier)
          .createScheduledMessage(
            content: attempt.content,
            clientMessageId: attempt.clientMessageId,
            scheduledAt: attempt.scheduledAt,
            sendWhenOnline: attempt.sendWhenOnline,
            mentions: attempt.mentions,
            threadParentId: attempt.threadParentId,
          );
      if (!mounted ||
          submissionGeneration != _scheduledAttemptSubmissionGeneration ||
          !_isCurrentChat(attempt.chatId)) {
        return;
      }
      if (attempt.profileId !=
              ref.read(authControllerProvider).activeProfileId ||
          attempt.authorization != ref.read(authorizationHeaderProvider)) {
        setState(() => _pendingScheduledSend = null);
        return;
      }
      if (error != null) {
        if (identical(_pendingScheduledSend, attempt)) {
          setState(() => _pendingScheduledSend = attempt);
        }
        return;
      }
      if (!identical(_pendingScheduledSend, attempt)) return;
      setState(() => _pendingScheduledSend = null);
      if (_composer.text == attempt.content) {
        _composer.clear();
        final key = _draftKey;
        if (key != null) {
          await ref.read(chatDraftProvider(key).notifier).clear();
        }
      }
      if (replyTarget != null && replyTarget.id == attempt.threadParentId) {
        ref.read(chatReplyTargetProvider(widget.chatId).notifier).state = null;
      }
      _refocusComposer();
    } finally {
      if (mounted &&
          submissionGeneration == _scheduledAttemptSubmissionGeneration) {
        setState(() => _scheduledAttemptInFlight = false);
      }
    }
  }

  void _retryScheduledAttempt() {
    final attempt = _pendingScheduledSend;
    if (attempt == null || _scheduledAttemptInFlight) return;
    unawaited(_submitScheduledAttempt(attempt));
  }

  void _cancelScheduledRetry() {
    if (_pendingScheduledSend == null || _scheduledAttemptInFlight) return;
    setState(() => _pendingScheduledSend = null);
    unawaited(
      ref
          .read(chatRoomControllerProvider(widget.chatId).notifier)
          .loadScheduledMessages(),
    );
  }

  Widget _buildScheduledMessagesSection(
    ChatRoomState room,
    AppLocalizations l10n,
  ) {
    final pending = room.scheduledMessages
        .where(
          (item) =>
              item.status ==
              messaging_enums
                  .ScheduledMessageStatus
                  .SCHEDULED_MESSAGE_STATUS_PENDING,
        )
        .toList(growable: false);
    if (pending.isEmpty &&
        !room.isLoadingScheduledMessages &&
        room.scheduledMessagesError == null &&
        room.scheduledActionError == null &&
        _pendingScheduledSend == null) {
      return const SizedBox.shrink();
    }
    return Padding(
      key: ChatRoomPanel.scheduledMessagesKey,
      padding: const EdgeInsets.fromLTRB(12, 0, 12, 4),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          if (_pendingScheduledSend != null)
            Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                if (room.isSending || _scheduledAttemptInFlight)
                  const LinearProgressIndicator(minHeight: 2),
                VoiceCompactBanner(
                  key: ChatRoomPanel.scheduledCreateRetryKey,
                  message: l10n.chatScheduleFailure,
                  icon: Icons.schedule_send_outlined,
                  actionLabel: room.isSending || _scheduledAttemptInFlight
                      ? null
                      : l10n.chatScheduleRetry,
                  onAction: room.isSending || _scheduledAttemptInFlight
                      ? null
                      : _retryScheduledAttempt,
                  tone: VoiceBannerTone.error,
                ),
                Align(
                  alignment: AlignmentDirectional.centerEnd,
                  child: TextButton.icon(
                    key: ChatRoomPanel.scheduledCreateCancelKey,
                    onPressed: room.isSending || _scheduledAttemptInFlight
                        ? null
                        : _cancelScheduledRetry,
                    icon: const Icon(Icons.close),
                    label: Text(l10n.commonCancel),
                  ),
                ),
              ],
            ),
          if (room.scheduledMessagesError != null)
            VoiceCompactBanner(
              message: l10n.chatScheduleFailure,
              icon: Icons.cloud_off_outlined,
              actionLabel: l10n.commonRetry,
              onAction: () => unawaited(
                ref
                    .read(chatRoomControllerProvider(widget.chatId).notifier)
                    .loadScheduledMessages(
                      loadMore: room.scheduledMessagesFailedLoadMore,
                    ),
              ),
              tone: VoiceBannerTone.error,
            ),
          if (_scheduledActionRetry != null)
            VoiceCompactBanner(
              message: l10n.chatScheduleFailure,
              icon: Icons.cloud_off_outlined,
              actionLabel: l10n.commonRetry,
              onAction: () => unawaited(_scheduledActionRetry?.call()),
              tone: VoiceBannerTone.error,
            ),
          for (final item in pending)
            _ScheduledMessageRow(
              item: item,
              l10n: l10n,
              busy: _scheduledActionBusyIds.contains(item.id),
              onEdit: () => _editScheduledMessage(item),
              onCancel: () => _runScheduledRowAction(
                item.id,
                () => ref
                    .read(chatRoomControllerProvider(widget.chatId).notifier)
                    .cancelScheduledMessage(item.id),
              ),
              onSendNow: () => _runScheduledRowAction(
                item.id,
                () => ref
                    .read(chatRoomControllerProvider(widget.chatId).notifier)
                    .sendScheduledMessageNow(item.id),
              ),
            ),
          if (room.isLoadingScheduledMessages && pending.isEmpty)
            const LinearProgressIndicator(minHeight: 2),
          if (room.hasMoreScheduledMessages)
            TextButton(
              onPressed: room.isLoadingScheduledMessages
                  ? null
                  : () => unawaited(
                      ref
                          .read(
                            chatRoomControllerProvider(widget.chatId).notifier,
                          )
                          .loadScheduledMessages(loadMore: true),
                    ),
              child: Text(l10n.chatScheduleLoadMore),
            ),
        ],
      ),
    );
  }

  Future<void> _runScheduledRowAction(
    String scheduledMessageId,
    Future<String?> Function() action,
  ) async {
    if (_scheduledActionBusyIds.contains(scheduledMessageId)) return;
    setState(() {
      _scheduledActionBusyIds.add(scheduledMessageId);
      _scheduledActionRetry = () async {
        await _runScheduledRowAction(scheduledMessageId, action);
      };
    });
    final error = await action();
    if (!mounted) return;
    setState(() {
      _scheduledActionBusyIds.remove(scheduledMessageId);
      if (error == null) {
        _scheduledActionRetry = null;
      }
    });
  }

  Future<void> _editScheduledMessage(messaging_pb.ScheduledMessage item) async {
    final chatId = widget.chatId;
    final profileId = ref.read(authControllerProvider).activeProfileId;
    final authorization = ref.read(authorizationHeaderProvider);
    if (profileId == null || authorization == null || !_isCurrentChat(chatId)) {
      return;
    }
    final textController = TextEditingController(
      text: item.hasPayload() ? item.payload.content : '',
    );
    var editedAt = item.hasScheduledAt()
        ? protoTimestampToDateTime(item.scheduledAt)?.toLocal()
        : null;
    final edit = await showDialog<(String, DateTime?)>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setDialogState) => AlertDialog(
          title: Text(AppLocalizations.of(context)!.chatScheduleEdit),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                controller: textController,
                autofocus: true,
                maxLines: 4,
                minLines: 1,
              ),
              if (item.hasScheduledAt())
                TextButton.icon(
                  icon: const Icon(Icons.schedule),
                  label: Text(
                    editedAt == null
                        ? AppLocalizations.of(context)!.chatSchedulePickerTitle
                        : '${MaterialLocalizations.of(context).formatMediumDate(editedAt!)} ${MaterialLocalizations.of(context).formatTimeOfDay(TimeOfDay.fromDateTime(editedAt!))}',
                  ),
                  onPressed: () async {
                    final selected = await _pickScheduledDateTime();
                    if (selected != null) {
                      setDialogState(() => editedAt = selected.toLocal());
                    }
                  },
                ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text(AppLocalizations.of(context)!.commonCancel),
            ),
            FilledButton(
              onPressed: () =>
                  Navigator.pop(dialogContext, (textController.text, editedAt)),
              child: Text(AppLocalizations.of(context)!.chatScheduleEdit),
            ),
          ],
        ),
      ),
    );
    textController.dispose();
    if (!mounted ||
        edit == null ||
        widget.chatId != chatId ||
        profileId != ref.read(authControllerProvider).activeProfileId ||
        authorization != ref.read(authorizationHeaderProvider) ||
        !_isCurrentChat(chatId)) {
      return;
    }
    final payload = item.payload.deepCopy();
    payload.content = edit.$1;
    await _runScheduledRowAction(
      item.id,
      () => ref
          .read(chatRoomControllerProvider(chatId).notifier)
          .updateScheduledMessage(
            scheduledMessageId: item.id,
            payload: payload,
            scheduledAt: item.hasScheduledAt() ? edit.$2?.toUtc() : null,
            sendWhenOnline: item.sendWhenOnline ? true : null,
          ),
    );
  }

  Future<void> _send() async {
    if (ref.read(chatRoomControllerProvider(widget.chatId)).isDmPeerDeleted) {
      return;
    }
    if (ref.read(isDeviceOfflineProvider) ||
        ref.read(chatRoomControllerProvider(widget.chatId)).isOfflineCache) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(AppLocalizations.of(context)!.chatOfflineSendBlocked),
        ),
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
    final handleToProfileId = <String, String>{};
    for (final id in memberIds) {
      final profile = ref.read(profileProvider(id)).valueOrNull;
      final handle = profile?.handle;
      if (handle != null && handle.isNotEmpty) {
        handleToProfileId[handle] = id;
      }
    }
    final mentions = parseMentionsFromContent(
      text,
      memberProfileIds: memberIds,
      handleToProfileId: handleToProfileId,
    );
    final chatType = ref
        .read(chatListProvider)
        .valueOrNull
        ?.items
        .where((item) => item.chatId == widget.chatId)
        .map((item) => item.chat.type)
        .firstOrNull;
    final isDm = chatType == 'CHAT_TYPE_DM';
    final filteredMentions = isDm
        ? mentions.where((m) => m.type == 'user').toList()
        : mentions;
    final replyTarget = ref.read(chatReplyTargetProvider(widget.chatId));
    final err = await ref
        .read(chatRoomControllerProvider(widget.chatId).notifier)
        .sendMessage(
          text,
          mentions: filteredMentions,
          threadParentId: replyTarget?.id,
        );
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
      final key = _draftKey;
      if (key != null) {
        await ref.read(chatDraftProvider(key).notifier).clear();
      }
      if (replyTarget != null) {
        ref.read(chatReplyTargetProvider(widget.chatId).notifier).state = null;
      }
    }
    _refocusComposer();
  }

  Future<void> _openAttachMenu() async {
    final action = await showComposerAttachMenu(
      context,
      onDismiss: () => _attachFocus.requestFocus(),
    );
    if (!mounted || action == null) return;
    if (ref.read(chatRoomControllerProvider(widget.chatId)).isDmPeerDeleted) {
      return;
    }
    await _attachAndSend(
      imagesOnly: action == ComposerAttachAction.photoOrVideo,
    );
  }

  Future<void> _attachAndSend({bool imagesOnly = false}) async {
    final chatId = widget.chatId;
    if (_isDmPeerDeleted()) {
      return;
    }
    final picker =
        widget.attachmentPicker ??
        () => _defaultPickChatAttachment(imagesOnly: imagesOnly);
    final picked = await picker();
    if (picked == null || !_isCurrentChat(chatId)) return;
    final mimeType = _attachmentMimeType(picked.contentType, picked.name);
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    final operation = ++_attachmentOperation;
    setState(() {
      _uploadingAttachment = true;
      _pendingAttachmentUpload = null;
      _attachmentUploadFailure = null;
    });
    try {
      final isE2eChat = ref.read(chatE2eEnabledProvider(widget.chatId));
      final chatType = ref
          .read(chatListProvider)
          .valueOrNull
          ?.items
          .where((item) => item.chatId == widget.chatId)
          .map((item) => item.chat.type)
          .firstOrNull;
      var uploadBytes = picked.bytes;
      String? e2eKeyWire;
      if (isE2eChat) {
        final activeId = ref.read(authControllerProvider).activeProfileId;
        final peerId = ref.read(dmPeerProfileByChatIdProvider)[widget.chatId];
        if (activeId == null || peerId == null || peerId.isEmpty) return;
        final encrypted = await const E2eFileCrypto().encryptBytes(
          plaintext: uploadBytes,
          messageService: ref.read(e2eMessageServiceProvider),
          localProfileId: activeId,
          peerProfileId: peerId,
          authorization: auth,
          chatId: widget.chatId,
        );
        uploadBytes = encrypted.ciphertext;
        e2eKeyWire = encrypted.keyWire;
        if (!_isCurrentAttachmentOperation(operation, chatId)) return;
      }
      if (!_isCurrentAttachmentOperation(operation, chatId)) return;
      await _runAttachmentUpload(
        _PendingAttachmentUpload(
          file: picked,
          bytes: uploadBytes,
          mimeType: mimeType,
          authorization: auth,
          chatId: chatId,
          chatType: chatType,
          isE2e: isE2eChat,
          e2eKeyWire: e2eKeyWire,
          imagesOnly: imagesOnly,
        ),
        operation,
      );
    } finally {
      if (_isCurrentAttachmentOperation(operation, chatId)) {
        setState(() => _uploadingAttachment = false);
      }
    }
  }

  Future<void> _runAttachmentUpload(
    _PendingAttachmentUpload attempt,
    int operation,
  ) async {
    final files = ref.read(voiceFilesClientProvider);
    final ticket = await files.requestUpload(
      authorization: attempt.authorization,
      originalName: attempt.file.name,
      mimeType: attempt.mimeType,
      sizeBytes: attempt.bytes.length,
      chatId: attempt.chatId,
      chatType: attempt.chatType,
      isE2e: attempt.isE2e,
    );
    if (!_isCurrentAttachmentOperation(operation, attempt.chatId)) return;
    if (ticket case FilesApiFailure()) {
      _retainFailedAttachment(attempt, ticket, operation);
      return;
    }
    if (ticket is! FilesApiOk<FileUploadTicket>) return;

    final put = await files.putBytes(
      uploadUrl: ticket.data.presignedPutUrl,
      bytes: attempt.bytes,
      mimeType: attempt.mimeType,
    );
    if (!_isCurrentAttachmentOperation(operation, attempt.chatId)) return;
    if (put case FilesApiFailure()) {
      _retainFailedAttachment(attempt, put, operation);
      return;
    }
    if (put is! FilesApiOk<void>) return;

    final confirmed = await files.confirmUpload(
      authorization: attempt.authorization,
      fileId: ticket.data.fileId,
      bytes: attempt.bytes,
    );
    if (!_isCurrentAttachmentOperation(operation, attempt.chatId)) return;
    if (confirmed case FilesApiFailure(
      :final errorCode,
    ) when errorCode == 'file_infected') {
      setState(() {
        _pendingAttachmentUpload = attempt;
        _attachmentUploadFailure = confirmed;
      });
      return;
    }
    if (confirmed case FilesApiFailure()) {
      _retainFailedAttachment(attempt, confirmed, operation);
      return;
    }
    if (confirmed is! FilesApiOk<FileMetadataData>) return;

    final metadata = confirmed.data;
    final err = await ref
        .read(chatRoomControllerProvider(attempt.chatId).notifier)
        .sendMessage(
          _composer.text,
          attachments: [
            MessageAttachment(
              fileId: metadata.fileId,
              type: metadata.fileType,
              name: metadata.originalName,
              sizeBytes: metadata.sizeBytes,
              e2eKeyWire: attempt.e2eKeyWire,
            ),
          ],
        );
    if (!_isCurrentAttachmentOperation(operation, attempt.chatId)) return;
    setState(() {
      _pendingAttachmentUpload = null;
      _attachmentUploadFailure = null;
    });
    if (err == null) {
      _composer.clear();
      final key = _draftKey;
      if (key != null) {
        await ref.read(chatDraftProvider(key).notifier).clear();
      }
    }
    _refocusComposer();
  }

  void _retainFailedAttachment(
    _PendingAttachmentUpload attempt,
    FilesApiFailure failure,
    int operation,
  ) {
    if (!_isCurrentAttachmentOperation(operation, attempt.chatId)) return;
    setState(() {
      _pendingAttachmentUpload = attempt;
      _attachmentUploadFailure = failure;
    });
  }

  Future<void> _retryAttachmentUpload() async {
    final pending = _pendingAttachmentUpload;
    if (pending == null ||
        _uploadingAttachment ||
        !_isCurrentChat(pending.chatId)) {
      return;
    }
    final authorization = ref.read(authorizationHeaderProvider);
    if (authorization == null) return;
    final attempt = pending.withAuthorization(authorization);
    final operation = ++_attachmentOperation;
    setState(() {
      _uploadingAttachment = true;
      _attachmentUploadFailure = null;
    });
    try {
      await _runAttachmentUpload(attempt, operation);
    } finally {
      if (_isCurrentAttachmentOperation(operation, attempt.chatId)) {
        setState(() => _uploadingAttachment = false);
      }
    }
  }

  void _cancelPendingAttachmentUpload() {
    if (_uploadingAttachment) return;
    _attachmentOperation++;
    setState(() {
      _pendingAttachmentUpload = null;
      _attachmentUploadFailure = null;
    });
    _refocusComposer();
  }

  void _pickAnotherAttachment() {
    final pending = _pendingAttachmentUpload;
    if (pending == null || _uploadingAttachment) return;
    final imagesOnly = pending.imagesOnly;
    _attachmentOperation++;
    setState(() {
      _pendingAttachmentUpload = null;
      _attachmentUploadFailure = null;
    });
    unawaited(_attachAndSend(imagesOnly: imagesOnly));
  }

  bool _isCurrentChat(String chatId) =>
      mounted && widget.chatId == chatId && !_isDmPeerDeleted();

  bool _isCurrentAttachmentOperation(int operation, String chatId) =>
      operation == _attachmentOperation && _isCurrentChat(chatId);

  String _attachmentFailureMessage(
    AppLocalizations l10n,
    FilesApiFailure failure,
  ) => switch (failure.errorCode) {
    'file_infected' => l10n.chatAttachmentBlocked,
    'file_scan_failed' => l10n.chatAttachmentScanFailed,
    _ => l10n.chatAttachmentUploadFailed,
  };

  bool _isDmPeerDeleted() =>
      ref.read(chatRoomControllerProvider(widget.chatId)).isDmPeerDeleted;

  Future<void> _showMessageActions(VoiceMessage message, bool isMine) async {
    String? spaceId;
    var canUseSpaceMessageModeration = false;
    for (final item in ref.read(chatListControllerProvider).items) {
      if (item.chatId == widget.chatId) {
        spaceId = item.chat.spaceId;
        canUseSpaceMessageModeration =
            spaceId != null && (item.chat.isGroup || item.chat.isChannel);
        break;
      }
    }
    final action = await showModalBottomSheet<String>(
      context: context,
      builder: (context) {
        final sheetL10n = AppLocalizations.of(context)!;
        return SafeArea(
          child: SingleChildScrollView(
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
                    leading: const Icon(Icons.reply_outlined),
                    title: Text(sheetL10n.chatMessageReply),
                    onTap: () => Navigator.of(context).pop('reply'),
                  ),
                  if (!ref
                      .read(chatRoomControllerProvider(widget.chatId))
                      .isDmPeerDeleted)
                    ListTile(
                      leading: const Icon(Icons.forward_outlined),
                      title: Text(sheetL10n.chatMessageForward),
                      onTap: () => Navigator.of(context).pop('forward'),
                    ),
                  ListTile(
                    key: const Key('message_action_copy_as_new'),
                    leading: const Icon(Icons.content_copy_outlined),
                    title: Text(sheetL10n.chatMessageCopyAsNew),
                    onTap: () => Navigator.of(context).pop('copy_as_new'),
                  ),
                  ListTile(
                    leading: const Icon(Icons.link),
                    title: Text(sheetL10n.shareLinkAction),
                    onTap: () => Navigator.of(context).pop('share'),
                  ),
                  ListTile(
                    leading: Icon(
                      message.isPinned
                          ? Icons.push_pin
                          : Icons.push_pin_outlined,
                    ),
                    title: Text(
                      message.isPinned
                          ? sheetL10n.chatMessageUnpin
                          : sheetL10n.chatMessagePin,
                    ),
                    onTap: () => Navigator.of(
                      context,
                    ).pop(message.isPinned ? 'unpin' : 'pin'),
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
                if (!isMine &&
                    message.deletedAt == null &&
                    message.messageKind != VoiceMessageKind.system)
                  ListTile(
                    leading: const Icon(Icons.flag_outlined),
                    title: Text(sheetL10n.reportAction),
                    onTap: () => Navigator.of(context).pop('report'),
                  ),
                if (isMine)
                  ListTile(
                    leading: const Icon(Icons.delete_forever_outlined),
                    title: Text(sheetL10n.chatMessageDeleteForEveryone),
                    onTap: () => Navigator.of(context).pop('delete_everyone'),
                  ),
                if (!isMine &&
                    canUseSpaceMessageModeration &&
                    message.deletedAt == null &&
                    message.messageKind != VoiceMessageKind.system)
                  Consumer(
                    builder: (context, ref, _) {
                      final permission = ref.watch(
                        spacePermissionProvider((
                          spaceId: spaceId!,
                          permission: SpacePermissions.textChatManageMessages,
                          chatId: widget.chatId,
                          voiceRoomId: null,
                        )),
                      );
                      final allowed = permission.valueOrNull == true;
                      if (allowed) {
                        return ListTile(
                          leading: const Icon(Icons.delete_forever_outlined),
                          title: Text(sheetL10n.chatMessageDeleteForEveryone),
                          onTap: () =>
                              Navigator.of(context).pop('delete_everyone'),
                        );
                      }
                      final unavailableReason = permission.isLoading
                          ? sheetL10n.spacePermissionChecking
                          : sheetL10n.spaceModerationUnavailable;
                      return VoiceDisabledAction(
                        disabledReason: unavailableReason,
                        child: ListTile(
                          leading: const Icon(Icons.info_outline),
                          title: Text(unavailableReason),
                          enabled: false,
                        ),
                      );
                    },
                  ),
              ],
            ),
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
      final result = await controller.togglePinWithResult(
        message.id,
        currentlyPinned: message.isPinned,
      );
      if (mounted && !result.succeeded && !result.stale) {
        final l10n = AppLocalizations.of(context)!;
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(pinMutationErrorText(l10n, result))),
        );
      }
    } else if (action == 'reply') {
      ref.read(chatReplyTargetProvider(widget.chatId).notifier).state = message;
      ref.read(chatActiveThreadProvider(widget.chatId).notifier).state =
          message.id;
      _refocusComposer();
    } else if (action == 'forward') {
      if (ref.read(chatRoomControllerProvider(widget.chatId)).isDmPeerDeleted) {
        return;
      }
      await ForwardMessageSheet.show(
        context,
        sourceMessage: message,
        sourceChatId: widget.chatId,
      );
    } else if (action == 'copy_as_new') {
      await ForwardMessageSheet.show(
        context,
        sourceMessage: message,
        sourceChatId: widget.chatId,
        withoutAttribution: true,
      );
    } else if (action == 'share') {
      final link = shareUrlForChat(
        chatId: widget.chatId,
        spaceId: spaceId,
        messageId: message.id,
      );
      if (link != null) {
        await copyVoiceShareLink(context, link);
      }
    } else if (action == 'edit') {
      final edited = await _promptEdit(message.content);
      if (edited != null) {
        await controller.editMessage(message.id, edited);
      }
    } else if (action == 'delete_me') {
      await controller.deleteMessage(message.id, forMe: true);
    } else if (action == 'delete_everyone') {
      await controller.deleteMessage(message.id, forMe: false);
    } else if (action == 'report') {
      await ReportSheet.show(
        context,
        target: ReportMessageTarget(
          messageId: message.id,
          chatId: widget.chatId,
        ),
      );
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
                  icon: Text(
                    emoji,
                    style: VoiceEmojiStyle.textStyle(fontSize: 28),
                  ),
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
    required this.ephemeralMessages,
    this.deferredInteraction,
    required this.activeId,
    required this.isGroup,
    required this.l10n,
    required this.initialUnreadCount,
    required this.chatTheme,
    this.highlightedMessageId,
    this.keyboardSelectedMessageId,
    required this.onLongPress,
  });

  final String chatId;
  final ScrollController scrollController;
  final ChatRoomState room;
  final List<EphemeralBotMessage> ephemeralMessages;
  final DeferredBotInteraction? deferredInteraction;
  final String? activeId;
  final bool isGroup;
  final AppLocalizations l10n;
  final int initialUnreadCount;
  final ChatTheme? chatTheme;
  final String? highlightedMessageId;
  final String? keyboardSelectedMessageId;
  final void Function(VoiceMessage message, bool isMine) onLongPress;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final rows = buildChatMessageRows(
      messages: room.messages,
      unreadCount: initialUnreadCount,
    );
    final hasOlderControl = room.hasMore || room.isLoadingOlder;
    final ephemeralCount = ephemeralMessages.length;
    final deferredCount = deferredInteraction != null ? 1 : 0;
    return ListView.builder(
      controller: scrollController,
      padding: const EdgeInsets.all(12),
      itemCount:
          rows.length +
          ephemeralCount +
          deferredCount +
          (hasOlderControl ? 1 : 0),
      itemBuilder: (context, index) {
        if (hasOlderControl && index == 0) {
          return Center(
            child: room.isLoadingOlder
                ? const Padding(
                    padding: EdgeInsets.all(8),
                    child: SizedBox(
                      height: 48,
                      child: VoiceListSkeleton(rowCount: 1),
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
        if (rowIndex >= rows.length + ephemeralCount) {
          return _DeferredBotBubble(
            l10n: l10n,
            botName: deferredInteraction?.botName,
          );
        }
        if (rowIndex >= rows.length) {
          final ephemeral = ephemeralMessages[rowIndex - rows.length];
          return _EphemeralBotBubble(message: ephemeral, l10n: l10n);
        }
        final row = rows[rowIndex];
        final msg = row.message;
        final isMine = msg.senderProfileId == activeId;
        final isHighlighted = highlightedMessageId == msg.id;
        final isKeyboardSelected = keyboardSelectedMessageId == msg.id;
        return Column(
          children: [
            if (row.unreadSeparator)
              ChatUnreadSeparator(label: l10n.chatUnreadSeparator),
            if (isGroup && !isMine && row.showTimestamp)
              _MessageAuthorHeader(senderProfileId: msg.senderProfileId),
            AnimatedContainer(
              duration: const Duration(milliseconds: 200),
              decoration: isHighlighted || isKeyboardSelected
                  ? BoxDecoration(
                      color: VoiceColors.of(context).profileAccent.withValues(
                        alpha: isHighlighted ? 0.25 : 0.12,
                      ),
                      borderRadius: BorderRadius.circular(8),
                    )
                  : null,
              child: GestureDetector(
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
                  theme: chatTheme,
                ),
              ),
            ),
            MessageReactionsRow(
              message: msg,
              isMine: isMine,
              onToggle: (emoji, reactedByMe) => ref
                  .read(chatRoomControllerProvider(chatId).notifier)
                  .toggleReaction(msg.id, emoji, currentlyReacted: reactedByMe),
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

Future<ChatAttachmentFile?> _defaultPickChatAttachment({
  bool imagesOnly = false,
}) async {
  final XFile? file;
  if (imagesOnly) {
    file = await openFile(
      acceptedTypeGroups: const [
        XTypeGroup(
          label: 'images',
          extensions: ['jpg', 'jpeg', 'png', 'webp', 'gif', 'mp4', 'mov'],
        ),
      ],
    );
  } else {
    file = await openFile();
  }
  if (file == null) return null;
  final bytes = await file.readAsBytes();
  return ChatAttachmentFile(
    bytes: bytes,
    contentType: _attachmentMimeType(file.mimeType, file.name),
    name: file.name,
  );
}

String _attachmentMimeType(String? mimeType, String name) {
  final selectedType = mimeType?.trim().toLowerCase();
  // Until animated GIF processing is available, preserve the original bytes.
  if (selectedType != null &&
      selectedType.split(';').first.trim() == 'image/gif') {
    return 'application/octet-stream';
  }
  return selectedType == null || selectedType.isEmpty
      ? _contentTypeFromName(name)
      : selectedType;
}

String _contentTypeFromName(String name) {
  final lower = name.toLowerCase();
  if (lower.endsWith('.jpg') || lower.endsWith('.jpeg')) return 'image/jpeg';
  if (lower.endsWith('.png')) return 'image/png';
  if (lower.endsWith('.webp')) return 'image/webp';
  if (lower.endsWith('.mp4')) return 'video/mp4';
  if (lower.endsWith('.mov')) return 'video/quicktime';
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
        if (message.decryptionFailed)
          E2eUndecryptableMessagePlaceholder(beforeDate: message.createdAt)
        else if (message.content.isNotEmpty)
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
          _AttachmentPreview(
            attachment: attachment,
            chatId: message.chatId,
            senderProfileId: message.senderProfileId,
          ),
        ],
      ],
    );
  }
}

class _AttachmentPreview extends ConsumerWidget {
  const _AttachmentPreview({
    required this.attachment,
    required this.chatId,
    required this.senderProfileId,
  });

  final MessageAttachment attachment;
  final String chatId;
  final String senderProfileId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final voice = VoiceColors.of(context);
    final metadata = ref.watch(
      fileAttachmentMetadataProvider(attachment.fileId),
    );
    if (!metadata.isLoading &&
        !metadata.hasError &&
        metadata.valueOrNull?.status == 'expired') {
      final l10n = AppLocalizations.of(context)!;
      return Tooltip(
        message: l10n.chatExpiredFileTooltip,
        triggerMode: TooltipTriggerMode.tap,
        child: Container(
          key: ValueKey('expired_attachment_placeholder_${attachment.fileId}'),
          constraints: const BoxConstraints(maxWidth: 260),
          padding: const EdgeInsets.all(8),
          decoration: BoxDecoration(
            color: voice.surface,
            borderRadius: BorderRadius.circular(4),
            border: Border.all(color: voice.borderDefault),
          ),
          child: const SizedBox(
            width: 160,
            height: 48,
            child: Center(
              child: Text('🦴🦴🦴', style: TextStyle(fontSize: 20)),
            ),
          ),
        ),
      );
    }
    if (attachment.isImage) {
      if (attachment.isE2eEncrypted) {
        final decryptRequest = E2eAttachmentDecryptRequest(
          fileId: attachment.fileId,
          e2eKeyWire: attachment.e2eKeyWire!,
          senderProfileId: senderProfileId,
          chatId: chatId,
        );
        final bytesAsync = ref.watch(
          e2eDecryptedAttachmentThumbProvider(decryptRequest),
        );
        return Semantics(
          key: ChatRoomPanel.attachmentPreviewKey(attachment.fileId),
          label:
              attachment.name ??
              AppLocalizations.of(context)!.chatImageAttachment,
          child: ClipRRect(
            borderRadius: BorderRadius.circular(4),
            child: Container(
              constraints: const BoxConstraints(maxWidth: 220, maxHeight: 160),
              color: voice.surface,
              child: bytesAsync.isLoading || bytesAsync.valueOrNull == null
                  ? const _AttachmentIcon(icon: Icons.image_outlined)
                  : Image.memory(
                      bytesAsync.valueOrNull!,
                      fit: BoxFit.cover,
                      errorBuilder: (context, error, stackTrace) =>
                          const _AttachmentIcon(icon: Icons.image_outlined),
                    ),
            ),
          ),
        );
      }
      // Image previews are always resolved through File's thumbnail variant.
      // Never use attachment metadata as a direct storage URL or key.
      final resolved = ref.watch(
        fileAttachmentThumbnailUrlProvider(attachment.fileId),
      );
      final src = resolved.valueOrNull;
      return Semantics(
        key: ChatRoomPanel.attachmentPreviewKey(attachment.fileId),
        label:
            attachment.name ??
            AppLocalizations.of(context)!.chatImageAttachment,
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
                    errorBuilder: (context, error, stackTrace) {
                      if (error is NetworkImageLoadException &&
                          (error.statusCode == 403 ||
                              error.statusCode == 410)) {
                        ref.invalidate(
                          fileAttachmentThumbnailUrlProvider(attachment.fileId),
                        );
                      }
                      return const _AttachmentIcon(icon: Icons.image_outlined);
                    },
                  ),
          ),
        ),
      );
    }
    if (attachment.isE2eEncrypted) {
      final l10n = AppLocalizations.of(context)!;
      final decryptRequest = E2eAttachmentDecryptRequest(
        fileId: attachment.fileId,
        e2eKeyWire: attachment.e2eKeyWire!,
        senderProfileId: senderProfileId,
        chatId: chatId,
      );
      final bytesAsync = ref.watch(
        e2eDecryptedAttachmentBytesProvider(decryptRequest),
      );
      return Material(
        key: ChatRoomPanel.attachmentPreviewKey(attachment.fileId),
        color: voice.surface,
        borderRadius: BorderRadius.circular(4),
        child: InkWell(
          borderRadius: BorderRadius.circular(4),
          onTap: bytesAsync.isLoading
              ? null
              : () => _downloadE2eAttachment(
                  context,
                  ref,
                  decryptRequest: decryptRequest,
                  fileName: attachment.name ?? attachment.fileId,
                  cachedBytes: bytesAsync.valueOrNull,
                ),
          child: Container(
            constraints: const BoxConstraints(maxWidth: 260),
            padding: const EdgeInsets.all(8),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(4),
              border: Border.all(color: voice.borderDefault),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                bytesAsync.isLoading
                    ? SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: voice.textSecondary,
                        ),
                      )
                    : Icon(
                        Icons.lock_outline,
                        size: 20,
                        color: voice.textSecondary,
                      ),
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
                      Text(
                        l10n.e2eAttachmentTapToDownload,
                        style: Theme.of(context).textTheme.labelSmall?.copyWith(
                          color: voice.textSecondary,
                        ),
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
          ),
        ),
      );
    }
    final l10n = AppLocalizations.of(context)!;
    return Container(
      key: ChatRoomPanel.attachmentPreviewKey(attachment.fileId),
      constraints: const BoxConstraints(maxWidth: 260),
      decoration: BoxDecoration(
        color: voice.surface,
        borderRadius: BorderRadius.circular(4),
        border: Border.all(color: voice.borderDefault),
      ),
      child: Material(
        color: Colors.transparent,
        borderRadius: BorderRadius.circular(4),
        child: InkWell(
          borderRadius: BorderRadius.circular(4),
          onTap: () =>
              _openFileAttachment(context, ref, fileId: attachment.fileId),
          child: Padding(
            padding: const EdgeInsets.all(8),
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
                      Text(
                        l10n.chatAttachmentTapToDownload,
                        style: Theme.of(context).textTheme.labelSmall?.copyWith(
                          color: voice.textSecondary,
                        ),
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
          ),
        ),
      ),
    );
  }

  Future<void> _openFileAttachment(
    BuildContext context,
    WidgetRef ref, {
    required String fileId,
  }) async {
    final l10n = AppLocalizations.of(context)!;
    try {
      // Fetch a fresh short-lived URL only after the user chooses to download.
      final url = await ref.refresh(fileAttachmentUrlProvider(fileId).future);
      if (!context.mounted) return;
      final uri = Uri.tryParse(url ?? '');
      if (uri == null || uri.host.isEmpty) {
        throw const FormatException('File service returned an invalid URL');
      }
      final isLocalDevelopmentHost = const {
        'localhost',
        '127.0.0.1',
        '::1',
        'host.docker.internal',
      }.contains(uri.host.toLowerCase());
      if (uri.scheme != 'https' &&
          !(uri.scheme == 'http' && isLocalDevelopmentHost)) {
        throw const FormatException('File service returned an invalid URL');
      }
      final opened = await launchUrl(uri, mode: LaunchMode.externalApplication);
      if (!opened) throw StateError('Could not open attachment URL');
    } catch (_) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.chatAttachmentDownloadFailed)),
      );
    }
  }

  Future<void> _downloadE2eAttachment(
    BuildContext context,
    WidgetRef ref, {
    required E2eAttachmentDecryptRequest decryptRequest,
    required String fileName,
    Uint8List? cachedBytes,
  }) async {
    final l10n = AppLocalizations.of(context)!;
    final bytes =
        cachedBytes ??
        await ref.read(
          e2eDecryptedAttachmentBytesProvider(decryptRequest).future,
        );
    if (!context.mounted) return;
    if (bytes == null) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(l10n.e2eAttachmentDecryptFailed)));
      return;
    }
    final saved = await saveDecryptedE2eAttachment(
      bytes: bytes,
      fileName: fileName,
    );
    if (!context.mounted) return;
    if (!saved) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(l10n.e2eAttachmentDownloadFailed)));
    }
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
    required this.message,
    required this.label,
    required this.contentTypeLabel,
    required this.onTap,
    required this.onOpenAll,
    required this.onHide,
  });

  final VoiceMessage message;
  final String label;
  final String? contentTypeLabel;
  final VoidCallback onTap;
  final VoidCallback onOpenAll;
  final VoidCallback onHide;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final l10n = AppLocalizations.of(context)!;
    final labelStyle = Theme.of(context).textTheme.labelMedium?.copyWith(
      color: voice.profileAccent,
      fontWeight: FontWeight.w600,
    );
    final bar = Material(
      color: voice.surface,
      child: Row(
        children: [
          Expanded(
            child: InkWell(
              onTap: onTap,
              child: Padding(
                padding: const EdgeInsetsDirectional.fromSTEB(12, 8, 4, 8),
                child: Row(
                  children: [
                    Icon(Icons.push_pin, size: 18, color: voice.profileAccent),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(label, style: labelStyle),
                          Text(
                            message.content.trim().isNotEmpty
                                ? message.content.trim()
                                : (contentTypeLabel ??
                                      l10n.chatPinnedMessagesTitle),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(color: voice.textSecondary),
                          ),
                          if (contentTypeLabel != null)
                            Text(
                              contentTypeLabel!,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: Theme.of(context).textTheme.labelSmall
                                  ?.copyWith(color: voice.textSecondary),
                            ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
          IconButton(
            tooltip: l10n.chatPinnedMessagesOpen,
            onPressed: onOpenAll,
            icon: const Icon(Icons.keyboard_arrow_down),
          ),
          IconButton(
            tooltip: l10n.chatPinnedMessagesHide,
            onPressed: onHide,
            icon: const Icon(Icons.close),
          ),
        ],
      ),
    );
    if (!VoiceLayout.isNarrow(MediaQuery.sizeOf(context).width)) return bar;
    return GestureDetector(
      onHorizontalDragEnd: (details) {
        if ((details.primaryVelocity ?? 0) < -100) onHide();
      },
      child: bar,
    );
  }
}

class _EphemeralBotBubble extends StatelessWidget {
  const _EphemeralBotBubble({required this.message, required this.l10n});

  final EphemeralBotMessage message;
  final AppLocalizations l10n;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    return Align(
      alignment: Alignment.centerLeft,
      child: VoiceChatBubble(
        isMine: false,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            if (message.botName != null && message.botName!.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(bottom: 4),
                child: Text(
                  message.botName!,
                  style: Theme.of(context).textTheme.labelMedium?.copyWith(
                    color: voice.profileAccent,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            Text(message.content),
            Padding(
              padding: const EdgeInsets.only(top: 4),
              child: Text(
                l10n.ephemeralMessageLabel,
                style: Theme.of(context).textTheme.labelSmall?.copyWith(
                  color: voice.textSecondary,
                  fontStyle: FontStyle.italic,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _DeferredBotBubble extends StatelessWidget {
  const _DeferredBotBubble({required this.l10n, this.botName});

  final AppLocalizations l10n;
  final String? botName;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    return Align(
      alignment: Alignment.centerLeft,
      child: VoiceChatBubble(
        isMine: false,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            if (botName != null && botName!.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(bottom: 4),
                child: Text(
                  botName!,
                  style: Theme.of(context).textTheme.labelMedium?.copyWith(
                    color: voice.profileAccent,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: voice.profileAccent,
                  ),
                ),
                const SizedBox(width: 8),
                Text(l10n.botDeferredProcessing),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _MessageAuthorHeader extends ConsumerWidget {
  const _MessageAuthorHeader({required this.senderProfileId});

  final String senderProfileId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final profileAsync = ref.watch(profileProvider(senderProfileId));
    final displayName =
        profileAsync.valueOrNull?.displayName ??
        senderProfileId.substring(
          0,
          senderProfileId.length < 8 ? senderProfileId.length : 8,
        );
    return Padding(
      padding: const EdgeInsets.only(left: 4, bottom: 2, top: 8),
      child: Align(
        alignment: Alignment.centerLeft,
        child: ChatAuthorLabel(
          displayName: displayName,
          isPremium: false,
          verificationType:
              profileAsync.valueOrNull?.verificationType ?? 'none',
          style: Theme.of(context).textTheme.labelMedium,
          premiumBadgeSemanticLabel: AppLocalizations.of(
            context,
          )!.premiumBadgeLabel,
          verifiedBadgeSemanticLabel:
              profileAsync.valueOrNull?.verificationType == 'organization'
              ? AppLocalizations.of(context)!.verifiedBadgeOrganization
              : AppLocalizations.of(context)!.verifiedBadgePersonal,
        ),
      ),
    );
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
