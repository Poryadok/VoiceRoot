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
import '../../backend/gateway_http.dart';
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
import 'scheduled_messages_strip.dart';
import 'message_reactions_row.dart';
import 'forward_message_sheet.dart';
import 'mention_message_content.dart';
import 'e2e_attachment_actions.dart';
import 'e2e_chat_settings.dart';
import 'e2e_identity_change_banner.dart';
import 'chat_composer_text_field.dart';
import 'composer_panels.dart';
import 'music_format.dart';
import 'music_attachment_player.dart';
import 'music_playback.dart';
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
  static const Key sendRetryKey = Key('chat_room_send_retry');
  static const Key replyTargetCancelKey = Key('chat_room_reply_target_cancel');
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

class _TextSendAttempt {
  _TextSendAttempt({
    required this.chatId,
    required this.identity,
    required this.sessionInstallGeneration,
    required this.generation,
    required this.content,
    required List<MessageMention> mentions,
    required this.replyTarget,
    required this.draftKey,
  }) : mentions = List.unmodifiable(mentions);

  final String chatId;
  final GatewayRequestIdentity identity;
  final int sessionInstallGeneration;
  final int generation;
  final String content;
  final List<MessageMention> mentions;
  final VoiceMessage? replyTarget;
  final ChatDraftKey? draftKey;
}

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
    required this.music,
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
  final bool music;

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
        music: music,
      );
}

class _ChatRoomPanelState extends ConsumerState<ChatRoomPanel> {
  final _composer = TextEditingController();
  final _slashComposerAnchor = GlobalKey();
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
  final Map<String, Future<void> Function()> _scheduledActionRetries = {};
  int _scheduledRowGeneration = 0;
  messaging_pb.ScheduledMessage? _scheduledEdit;
  final _scheduledEditComposer = TextEditingController();
  DateTime? _scheduledEditAt;
  bool _scheduledEditInvalidDate = false;
  bool _scheduledEditInvalidContent = false;
  final Set<String> _scheduledActionBusyIds = <String>{};
  var _pinnedJumpGeneration = 0;
  String? _shownPinnedMessageId;
  var _highlightedMessageId = null as String?;
  var _liveMessageAnnouncement = '';
  ChatDraftKey? _draftKey;
  _TextSendAttempt? _pendingTextSend;
  int? _pendingTextSendControllerGeneration;
  var _textSendGeneration = 0;
  var _textSendInFlight = false;
  var _textSendFailed = false;
  final _inChatSearchController = TextEditingController();

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScroll);
  }

  @override
  void dispose() {
    _cancelTextSend();
    _pinnedJumpGeneration++;
    _attachmentOperation++;
    _scheduledAttemptSubmissionGeneration++;
    _pendingAttachmentUpload = null;
    _attachmentUploadFailure = null;
    _composer.dispose();
    _scheduledEditComposer.dispose();
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
      _cancelTextSend();
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
      _scheduledActionRetries.clear();
      _scheduledActionBusyIds.clear();
      _scheduledRowGeneration++;
      _scheduledEdit = null;
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
    if (raw.toLowerCase().contains('threads are disabled for this chat')) {
      return l10n.chatThreadRepliesUnavailable;
    }
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
    final scheduledMessagesEnabled = ref
        .watch(gatewayConfigProvider)
        .scheduledMessagesEnabled;
    ref.listen<AuthState>(authControllerProvider, (previous, next) {
      final textAttempt = _pendingTextSend;
      if (textAttempt != null && !_isTextSendActorCurrent(textAttempt)) {
        setState(_cancelTextSend);
      }
      final changed =
          previous?.activeProfileId != next.activeProfileId ||
          previous?.session?.accessToken != next.session?.accessToken;
      if (!changed) return;
      if (mounted) {
        setState(() {
          _scheduledAttemptSubmissionGeneration++;
          _scheduledAttemptInFlight = false;
          _pendingScheduledSend = null;
          _scheduledActionRetries.clear();
          _scheduledActionBusyIds.clear();
          _scheduledRowGeneration++;
          _scheduledEdit = null;
        });
      }
      _scheduledMessagesRequested = false;
      if (scheduledMessagesEnabled &&
          next.activeProfileId != null &&
          ref.read(selectedChatIdProvider) == widget.chatId) {
        _scheduledMessagesRequested = true;
        unawaited(
          ref
              .read(chatRoomControllerProvider(widget.chatId).notifier)
              .loadScheduledMessages(),
        );
      }
    });
    ref.listen<String?>(selectedChatIdProvider, (previous, next) {
      if (previous != next && next != widget.chatId) {
        setState(() {
          _cancelTextSend();
          _scheduledRowGeneration++;
          _scheduledActionBusyIds.clear();
          _scheduledActionRetries.clear();
          _scheduledEdit = null;
        });
      }
      if (!scheduledMessagesEnabled ||
          next != widget.chatId ||
          _scheduledMessagesRequested) {
        return;
      }
      _scheduledMessagesRequested = true;
      unawaited(
        ref
            .read(chatRoomControllerProvider(widget.chatId).notifier)
            .loadScheduledMessages(),
      );
    });
    if (scheduledMessagesEnabled &&
        !_scheduledMessagesRequested &&
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
      final restored = draftKey == null
          ? ''
          : ref.read(chatDraftProvider(draftKey)).valueOrNull ?? '';
      _composer.value = TextEditingValue(
        text: restored,
        selection: TextSelection.collapsed(offset: restored.length),
      );
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
    final textSendFailed =
        _pendingTextSend?.replyTarget != null &&
            !_threadsEnabledForCurrentChat() ||
        (_textSendFailed &&
            _pendingTextSendControllerGeneration ==
                ref
                    .read(chatRoomControllerProvider(widget.chatId).notifier)
                    .immediateSendGeneration);
    final textSendOwnsRoomError =
        textSendFailed &&
        ref
            .read(chatRoomControllerProvider(widget.chatId).notifier)
            .failedMessageOwnsRoomError;
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
    final chatListState = ref.watch(chatListControllerProvider);
    final listBelongsToViewer =
        activeId != null && chatListState.profileId == activeId;
    VoiceChat? chatMeta;
    ChatListItem? chatListItem;
    for (final item
        in listBelongsToViewer ? chatListState.items : const <ChatListItem>[]) {
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
    final threadsEnabled = chatMeta?.threadsEnabled == true;
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
    final roomBelongsToViewer =
        activeId != null && room.historyProfileId == activeId;
    final chatMetadataAvailable = chatMeta != null;
    final peerId = !chatMetadataAvailable || isGroup
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
    final title = !chatMetadataAvailable
        ? l10n.chatRoomTitle(shortId)
        : isGroup
        ? (groupName ?? l10n.chatRoomTitle(shortId))
        : (peerName ?? l10n.socialProfileUnavailable);
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
      if (chatMetadataAvailable)
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
                        ? room.errorMessage != null && !textSendOwnsRoomError
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
              if (room.errorMessage != null &&
                  room.messages.isNotEmpty &&
                  !textSendOwnsRoomError)
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
                      key: ChatRoomPanel.replyTargetCancelKey,
                      icon: const Icon(Icons.close, size: 18),
                      onPressed: _cancelReplyTarget,
                    ),
                  ),
                ),
              if (scheduledMessagesEnabled)
                _buildScheduledMessagesSection(room, l10n),
              if (_scheduledEdit != null) _buildScheduledEdit(l10n),
              if (textSendFailed && _pendingTextSend != null)
                _buildTextSendFailure(
                  l10n,
                  room.isSending || composerBlocked,
                  _pendingTextSend!,
                ),
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
                          onPressed:
                              room.isSending ||
                                  composerBlocked ||
                                  _scheduledEdit != null
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
                        key: _slashComposerAnchor,
                        child: ChatComposerTextField(
                          key: ChatRoomPanel.inputKey,
                          controller: _scheduledEdit == null
                              ? _composer
                              : _scheduledEditComposer,
                          focusNode: _composerFocus,
                          decoration: InputDecoration(
                            hintText:
                                blockChannelMainFeed && replyTarget == null
                                ? l10n.chatChannelMainFeedBlocked
                                : l10n.chatRoomInputHint,
                            isDense: true,
                          ),
                          onChanged: (value) {
                            if (_scheduledEdit != null) {
                              if (_scheduledEditInvalidContent) {
                                setState(
                                  () => _scheduledEditInvalidContent = false,
                                );
                              }
                              return;
                            }
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
                          onSend:
                              room.isSending ||
                                  _textSendInFlight ||
                                  composerBlocked
                              ? null
                              : _send,
                          readOnly:
                              composerBlocked ||
                              _scheduledActionBusyIds.contains(
                                _scheduledEdit?.id,
                              ),
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
                                  _scheduledEdit != null ||
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
                        onPressed:
                            room.isSending ||
                                _textSendInFlight ||
                                composerBlocked
                            ? null
                            : _send,
                        onLongPress:
                            room.isSending ||
                                _scheduledEdit != null ||
                                _uploadingAttachment ||
                                composerBlocked ||
                                !scheduledMessagesEnabled
                            ? null
                            : _openScheduledSendMenu,
                        isLoading: room.isSending || _textSendInFlight,
                        tooltip: _scheduledEdit == null
                            ? l10n.chatSendMessage
                            : l10n.chatScheduleEdit,
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
        if (activeThreadId != null && threadsEnabled)
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
              onClose: _cancelReplyTarget,
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
      anchorContext: _slashComposerAnchor.currentContext,
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
    if (!ref.read(gatewayConfigProvider).scheduledMessagesEnabled ||
        _composer.text.trim().isEmpty ||
        _isDmPeerDeleted() ||
        !_isCurrentChat(chatId)) {
      return;
    }
    final l10n = AppLocalizations.of(context)!;
    final authorization = ref.read(authorizationHeaderProvider);
    final profileId = ref.read(authControllerProvider).activeProfileId;
    if (authorization == null || profileId == null) return;
    final chatListState = ref.read(chatListControllerProvider);
    final chatType = chatListState.profileId == profileId
        ? chatListState.items
              .where((item) => item.chatId == chatId)
              .map((item) => item.chat.type)
              .firstOrNull
        : null;
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
    });
    await _submitScheduledAttempt(attempt);
  }

  Future<DateTime?> _pickScheduledDateTime({DateTime? initial}) async {
    final now = DateTime.now();
    final latest = now.add(const Duration(days: 365));
    final candidate = initial?.toLocal();
    final valid =
        candidate != null &&
        candidate.isAfter(now) &&
        !candidate.isAfter(latest);
    var date = DateUtils.dateOnly(valid ? candidate : now);
    var time = TimeOfDay.fromDateTime(
      valid ? candidate : now.add(const Duration(minutes: 1)),
    );
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
          ScheduledMessagesStrip(
            key: ValueKey(
              '${widget.chatId}:${ref.read(authControllerProvider).activeProfileId}',
            ),
            messages: pending,
            editingId: _scheduledEdit?.id,
            busyIds: _scheduledActionBusyIds,
            failedIds: _scheduledActionRetries.keys.toSet(),
            onEdit: _editScheduledMessage,
            onCancel: (item) => _runScheduledRowAction(
              item.id,
              () => ref
                  .read(chatRoomControllerProvider(widget.chatId).notifier)
                  .cancelScheduledMessage(item.id),
            ),
            onSendNow: (item) => _runScheduledRowAction(
              item.id,
              () => ref
                  .read(chatRoomControllerProvider(widget.chatId).notifier)
                  .sendScheduledMessageNow(item.id),
            ),
            onRetry: (id) => unawaited(_scheduledActionRetries[id]?.call()),
            onDismissError: (id) =>
                setState(() => _scheduledActionRetries.remove(id)),
            loading: room.isLoadingScheduledMessages,
            loadFailed: room.scheduledMessagesError != null,
            hasMore: room.hasMoreScheduledMessages,
            onRetryLoad: () => unawaited(
              ref
                  .read(chatRoomControllerProvider(widget.chatId).notifier)
                  .loadScheduledMessages(
                    loadMore: room.scheduledMessagesFailedLoadMore,
                  ),
            ),
            onLoadMore: () => unawaited(
              ref
                  .read(chatRoomControllerProvider(widget.chatId).notifier)
                  .loadScheduledMessages(loadMore: true),
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _runScheduledRowAction(
    String scheduledMessageId,
    Future<String?> Function() action,
  ) async {
    if (_scheduledActionBusyIds.contains(scheduledMessageId) ||
        !_isCurrentChat(widget.chatId))
      return;
    final generation = _scheduledRowGeneration;
    final chatId = widget.chatId;
    final profileId = ref.read(authControllerProvider).activeProfileId;
    final authorization = ref.read(authorizationHeaderProvider);
    setState(() {
      _scheduledActionBusyIds.add(scheduledMessageId);
      _scheduledActionRetries.remove(scheduledMessageId);
    });
    String? error;
    try {
      error = await action();
    } catch (_) {
      error = 'unavailable';
    }
    if (!mounted ||
        generation != _scheduledRowGeneration ||
        widget.chatId != chatId ||
        !_isCurrentChat(chatId) ||
        profileId != ref.read(authControllerProvider).activeProfileId ||
        authorization != ref.read(authorizationHeaderProvider)) {
      return;
    }
    setState(() {
      _scheduledActionBusyIds.remove(scheduledMessageId);
      if (error != null) {
        _scheduledActionRetries[scheduledMessageId] = () =>
            _runScheduledRowAction(scheduledMessageId, action);
      }
    });
  }

  void _editScheduledMessage(messaging_pb.ScheduledMessage item) {
    if (_scheduledEdit != null ||
        item.payload.isE2e ||
        _pendingScheduledSend != null ||
        _textSendInFlight ||
        !_isCurrentChat(widget.chatId)) {
      return;
    }
    setState(() {
      _scheduledEdit = item.deepCopy();
      _scheduledEditComposer.text = item.payload.content;
      _scheduledEditAt = protoTimestampToDateTime(
        item.hasScheduledAt() ? item.scheduledAt : null,
      );
      _scheduledEditInvalidDate = false;
      _scheduledEditInvalidContent = false;
    });
    _refocusComposer();
  }

  Widget _buildScheduledEdit(AppLocalizations l10n) {
    final item = _scheduledEdit!;
    final busy = _scheduledActionBusyIds.contains(item.id);
    final date = _scheduledEditAt?.toLocal();
    final material = MaterialLocalizations.of(context);
    final label = item.sendWhenOnline
        ? l10n.chatSendWhenOnline
        : date == null
        ? l10n.chatSchedulePickerTitle
        : '${material.formatMediumDate(date)} ${material.formatTimeOfDay(TimeOfDay.fromDateTime(date))}';
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(l10n.chatScheduleEdit),
          Row(
            children: [
              Expanded(
                child: TextButton.icon(
                  key: const Key('scheduled_edit_date'),
                  icon: const Icon(Icons.schedule),
                  label: Text(label),
                  onPressed: busy || item.sendWhenOnline
                      ? null
                      : () async {
                          final generation = _scheduledRowGeneration;
                          final selected = await _pickScheduledDateTime(
                            initial: _scheduledEditAt,
                          );
                          if (!mounted ||
                              generation != _scheduledRowGeneration ||
                              _scheduledEdit != item ||
                              selected == null) {
                            return;
                          }
                          setState(() {
                            _scheduledEditAt = selected;
                            _scheduledEditInvalidDate = false;
                          });
                          _refocusComposer();
                        },
                ),
              ),
              IconButton(
                key: const Key('scheduled_edit_cancel'),
                tooltip: l10n.commonCancel,
                onPressed: busy
                    ? null
                    : () {
                        setState(() => _scheduledEdit = null);
                        _refocusComposer();
                      },
                icon: const Icon(Icons.close),
              ),
            ],
          ),
          if (_scheduledEditInvalidDate)
            Text(
              l10n.chatScheduleDateInvalid,
              style: TextStyle(color: VoiceColors.of(context).error),
            ),
          if (_scheduledEditInvalidContent)
            Text(
              l10n.chatScheduledContentInvalid,
              style: TextStyle(color: VoiceColors.of(context).error),
            ),
        ],
      ),
    );
  }

  Future<void> _saveScheduledEdit() async {
    final item = _scheduledEdit;
    if (item == null || _scheduledActionBusyIds.contains(item.id)) return;
    final date = _scheduledEditAt;
    final now = DateTime.now().toUtc();
    if (!item.sendWhenOnline &&
        (date == null ||
            !date.isAfter(now) ||
            date.isAfter(now.add(const Duration(days: 365))))) {
      setState(() => _scheduledEditInvalidDate = true);
      return;
    }
    final content = _scheduledEditComposer.text;
    if ((content.trim().isEmpty &&
            MessageAttachment.listFromWire(
              item.payload.attachmentsJson,
            ).isEmpty) ||
        content.length > 4000) {
      setState(() => _scheduledEditInvalidContent = true);
      return;
    }
    final chatId = widget.chatId;
    final generation = _scheduledRowGeneration;
    final payload = item.payload.deepCopy()
      ..content = content
      ..mentionsJson = MessageMention.encodeJson(_mentionsForComposer(content));
    await _runScheduledRowAction(item.id, () async {
      final error = await ref
          .read(chatRoomControllerProvider(chatId).notifier)
          .updateScheduledMessage(
            scheduledMessageId: item.id,
            payload: payload,
            scheduledAt: item.sendWhenOnline ? null : date,
            sendWhenOnline: item.sendWhenOnline ? true : null,
          );
      if (mounted &&
          generation == _scheduledRowGeneration &&
          error == null &&
          identical(_scheduledEdit, item) &&
          _scheduledEditComposer.text == content &&
          _scheduledEditAt == date) {
        setState(() => _scheduledEdit = null);
        _refocusComposer();
      }
      return error;
    });
  }

  Future<void> _send() async {
    if (_textSendInFlight) return;
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
    if (_scheduledEdit != null) {
      await _saveScheduledEdit();
      return;
    }
    final text = _composer.text;
    if (text.trim().isEmpty) return;
    final authController = ref.read(authControllerProvider.notifier);
    final identity = authController.gatewayRequestIdentity;
    if (identity == null) return;
    ref.read(realtimeHubProvider).typingStop(widget.chatId);
    final attempt = _TextSendAttempt(
      chatId: widget.chatId,
      identity: identity,
      sessionInstallGeneration: authController.sessionInstallGeneration,
      generation: ++_textSendGeneration,
      content: text,
      mentions: _mentionsForComposer(text),
      replyTarget: ref.read(chatReplyTargetProvider(widget.chatId)),
      draftKey: _draftKey,
    );
    _pendingTextSend = attempt;
    _pendingTextSendControllerGeneration = null;
    await _submitTextSend(attempt);
  }

  void _cancelTextSend() {
    _textSendGeneration++;
    _pendingTextSend = null;
    _pendingTextSendControllerGeneration = null;
    _textSendInFlight = false;
    _textSendFailed = false;
  }

  void _cancelReplyTarget() {
    final target = ref.read(chatReplyTargetProvider(widget.chatId));
    ref.read(chatReplyTargetProvider(widget.chatId).notifier).state = null;
    ref.read(chatActiveThreadProvider(widget.chatId).notifier).state = null;
    final attempt = _pendingTextSend;
    if (attempt?.replyTarget?.id == target?.id && _textSendFailed) {
      ref
          .read(chatRoomControllerProvider(widget.chatId).notifier)
          .discardFailedMessage();
      setState(_cancelTextSend);
    }
  }

  bool _isTextSendActorCurrent(_TextSendAttempt attempt) =>
      mounted &&
      ref.read(authControllerProvider.notifier).gatewayRequestIdentity ==
          attempt.identity &&
      ref.read(authControllerProvider.notifier).sessionInstallGeneration ==
          attempt.sessionInstallGeneration;

  bool _isCurrentTextSend(_TextSendAttempt attempt) {
    if (!_isTextSendActorCurrent(attempt)) return false;
    final selected = ref.read(selectedChatIdProvider);
    return widget.chatId == attempt.chatId &&
        (selected == null || selected == attempt.chatId) &&
        _textSendGeneration == attempt.generation;
  }

  Widget _buildTextSendFailure(
    AppLocalizations l10n,
    bool blocked,
    _TextSendAttempt attempt,
  ) => Padding(
    padding: const EdgeInsets.symmetric(horizontal: 12),
    child: Row(
      children: [
        Expanded(child: Text(_textSendFailureMessage(l10n))),
        TextButton(
          key: ChatRoomPanel.sendRetryKey,
          onPressed: blocked || _textSendInFlight
              ? null
              : () => unawaited(_submitTextSend(attempt, retry: true)),
          child: Text(l10n.commonRetry),
        ),
      ],
    ),
  );

  String _textSendFailureMessage(AppLocalizations l10n) {
    if (_pendingTextSend?.replyTarget != null &&
        !_threadsEnabledForCurrentChat()) {
      return l10n.chatThreadRepliesUnavailable;
    }
    final raw = ref
        .read(chatRoomControllerProvider(widget.chatId).notifier)
        .failedMessageError;
    final mapped = raw == null ? null : _roomErrorText(l10n, raw);
    return mapped == null || mapped == l10n.chatRoomLoadError
        ? commonActionErrorMessage(l10n)
        : mapped;
  }

  Future<void> _submitTextSend(
    _TextSendAttempt attempt, {
    bool retry = false,
  }) async {
    if (_textSendInFlight ||
        !identical(_pendingTextSend, attempt) ||
        !_isCurrentTextSend(attempt)) {
      return;
    }
    final controller = ref.read(
      chatRoomControllerProvider(attempt.chatId).notifier,
    );
    if (attempt.replyTarget != null && !_threadsEnabledForCurrentChat()) {
      setState(() {
        _textSendInFlight = false;
        _textSendFailed = true;
        _pendingTextSendControllerGeneration =
            controller.immediateSendGeneration;
      });
      return;
    }
    setState(() {
      _textSendInFlight = true;
      _textSendFailed = false;
    });
    final Future<String?> submission;
    if (retry) {
      submission = controller.retryFailedMessage(
        expectedGeneration: _pendingTextSendControllerGeneration,
      );
    } else {
      submission = controller.sendMessage(
        attempt.content,
        mentions: attempt.mentions,
        threadParentId: attempt.replyTarget?.id,
      );
      _pendingTextSendControllerGeneration = controller.immediateSendGeneration;
    }
    final err = await submission;
    if (!mounted || !_isCurrentTextSend(attempt)) return;
    setState(() {
      _textSendInFlight = false;
      _textSendFailed =
          err != null &&
          controller.canRetryFailedMessage &&
          _pendingTextSendControllerGeneration ==
              controller.immediateSendGeneration;
      if (err == null || (err == kChatActionStaleContext && !_textSendFailed)) {
        _pendingTextSend = null;
      }
    });
    if (err == kChatOfflineBlockedError) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(AppLocalizations.of(context)!.chatOfflineSendBlocked),
        ),
      );
    } else if (err == null) {
      if (_composer.text == attempt.content) {
        _composer.clear();
        final key = attempt.draftKey;
        if (key != null) {
          unawaited(ref.read(chatDraftProvider(key).notifier).clear());
        }
      }
      if (attempt.replyTarget != null &&
          ref.read(chatReplyTargetProvider(attempt.chatId))?.id ==
              attempt.replyTarget!.id) {
        ref.read(chatReplyTargetProvider(attempt.chatId).notifier).state = null;
      }
    }
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_isCurrentTextSend(attempt)) _composerFocus.requestFocus();
    });
  }

  Future<void> _openAttachMenu() async {
    final action = await showComposerAttachMenu(
      context,
      anchorContext: _attachFocus.context,
      onDismiss: () => _attachFocus.requestFocus(),
    );
    if (!mounted || action == null) return;
    if (ref.read(chatRoomControllerProvider(widget.chatId)).isDmPeerDeleted) {
      return;
    }
    await _attachAndSend(
      imagesOnly: action == ComposerAttachAction.photoOrVideo,
      music: action == ComposerAttachAction.music,
    );
  }

  Future<void> _attachAndSend({
    bool imagesOnly = false,
    bool music = false,
  }) async {
    final chatId = widget.chatId;
    if (_isDmPeerDeleted()) {
      return;
    }
    final picker =
        widget.attachmentPicker ??
        () => _defaultPickChatAttachment(imagesOnly: imagesOnly, music: music);
    final ChatAttachmentFile? picked;
    try {
      picked = await picker();
    } on _MusicFileRejected {
      if (_isCurrentChat(chatId)) _showMusicFileRejected();
      return;
    }
    if (picked == null || !_isCurrentChat(chatId)) return;
    final mimeType = _attachmentMimeType(picked.contentType, picked.name);
    if (music &&
        (!mimeType.startsWith('audio/') ||
            !hasSupportedMusicSignature(picked.bytes))) {
      _showMusicFileRejected();
      return;
    }
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
      if (music && uploadBytes.length > 10 * 1024 * 1024) {
        _showMusicFileRejected();
        return;
      }
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
          music: music,
        ),
        operation,
      );
    } finally {
      if (_isCurrentAttachmentOperation(operation, chatId)) {
        setState(() => _uploadingAttachment = false);
      }
    }
  }

  void _showMusicFileRejected() {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(AppLocalizations.of(context)!.chatMusicInvalidFile),
      ),
    );
  }

  Future<void> _runAttachmentUpload(
    _PendingAttachmentUpload attempt,
    int operation,
  ) async {
    final files = ref.read(voiceFilesClientProvider);
    final ticket = await files.requestUpload(
      authorization: attempt.authorization,
      originalName: attempt.music && attempt.isE2e
          ? 'encrypted-audio'
          : attempt.file.name,
      mimeType: attempt.mimeType,
      sizeBytes: attempt.bytes.length,
      chatId: attempt.chatId,
      chatType: attempt.chatType,
      isE2e: attempt.isE2e,
      music: attempt.music,
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
              type: attempt.music ? 'music' : metadata.fileType,
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
    unawaited(_attachAndSend(imagesOnly: imagesOnly, music: pending.music));
  }

  bool _isCurrentChat(String chatId) =>
      mounted && widget.chatId == chatId && !_isDmPeerDeleted();

  bool _threadsEnabledForCurrentChat() {
    final profileId = ref.read(authControllerProvider).activeProfileId;
    final list = ref.read(chatListControllerProvider);
    if (profileId == null || list.profileId != profileId) return false;
    return list.items
            .where((item) => item.chatId == widget.chatId)
            .map((item) => item.chat.threadsEnabled)
            .firstOrNull ==
        true;
  }

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
    final threadsEnabled = _threadsEnabledForCurrentChat();
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
                  if (threadsEnabled)
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
      if (!_threadsEnabledForCurrentChat()) return;
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
  bool music = false,
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
  } else if (music) {
    file = await openFile(
      acceptedTypeGroups: const [
        XTypeGroup(
          label: 'audio',
          extensions: ['mp3', 'ogg', 'flac', 'wav'],
          mimeTypes: ['audio/*'],
        ),
      ],
    );
  } else {
    file = await openFile();
  }
  if (file == null) return null;
  if (music) {
    final length = await file.length();
    if (length <= 0 || length > 10 * 1024 * 1024) {
      throw const _MusicFileRejected();
    }
  }
  final bytes = await file.readAsBytes();
  return ChatAttachmentFile(
    bytes: bytes,
    contentType: _attachmentMimeType(file.mimeType, file.name),
    name: file.name,
  );
}

class _MusicFileRejected implements Exception {
  const _MusicFileRejected();
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
  if (lower.endsWith('.mp3')) return 'audio/mpeg';
  if (lower.endsWith('.m4a')) return 'audio/mp4';
  if (lower.endsWith('.aac')) return 'audio/aac';
  if (lower.endsWith('.ogg')) return 'audio/ogg';
  if (lower.endsWith('.flac')) return 'audio/flac';
  if (lower.endsWith('.wav')) return 'audio/wav';
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
    if (attachment.isMusic) {
      if (!metadata.isLoading &&
          (metadata.hasError || metadata.valueOrNull?.status != 'ready')) {
        return Text(AppLocalizations.of(context)!.chatAttachmentDownloadFailed);
      }
      final viewer = ref.watch(authControllerProvider);
      final switching = ref.watch(profileSwitchInProgressProvider);
      final controller = ref.read(authControllerProvider.notifier);
      final generation = controller.sessionInstallGeneration;
      final session = viewer.session;
      final profile = viewer.activeProfileId;
      if (session == null || profile == null || switching) {
        return Text(AppLocalizations.of(context)!.chatAttachmentDownloadFailed);
      }
      bool viewerIsCurrent() =>
          context.mounted &&
          identical(ref.read(authControllerProvider.notifier), controller) &&
          controller.sessionInstallGeneration == generation &&
          !ref.read(profileSwitchInProgressProvider) &&
          ref.read(authControllerProvider).session?.accountId ==
              session.accountId &&
          ref.read(authControllerProvider).activeProfileId == profile;
      return MusicAttachmentPlayer(
        key: ValueKey((attachment.fileId, profile, generation, controller)),
        attachment: attachment,
        createPlayback: ref.watch(musicPlaybackFactoryProvider),
        loadBytes: () async {
          final auth = ref.read(authorizationHeaderProvider);
          if (auth == null || !viewerIsCurrent()) {
            throw const FormatException('music_access_missing');
          }
          final files = ref.read(voiceFilesClientProvider);
          final metadata = await files.getFileMetadata(
            authorization: auth,
            fileId: attachment.fileId,
          );
          if (!viewerIsCurrent() ||
              metadata is! FilesApiOk<FileMetadataData> ||
              metadata.data.status != 'ready' ||
              metadata.data.fileType != 'audio' ||
              !metadata.data.isMusic ||
              metadata.data.isE2e != attachment.isE2eEncrypted ||
              metadata.data.sizeBytes == null ||
              metadata.data.sizeBytes! > musicMaxStoredBytes) {
            throw const FormatException('music_file_unavailable');
          }
          final downloaded = await files.fetchFileBytes(
            authorization: auth,
            fileId: attachment.fileId,
            maxBytes: musicMaxStoredBytes,
          );
          if (downloaded is! FilesApiOk<Uint8List>) {
            throw const FormatException('music_download_unavailable');
          }
          if (!viewerIsCurrent()) {
            downloaded.data.fillRange(0, downloaded.data.length, 0);
            throw const FormatException('music_load_cancelled');
          }
          if (!attachment.isE2eEncrypted) return downloaded.data;
          try {
            final plaintext = await const E2eFileCrypto().decryptBytes(
              ciphertext: downloaded.data,
              keyWire: attachment.e2eKeyWire!,
              messageService: ref.read(e2eMessageServiceProvider),
              localProfileId: profile,
              peerProfileId: senderProfileId,
              authorization: auth,
            );
            if (!viewerIsCurrent()) {
              plaintext.fillRange(0, plaintext.length, 0);
              throw const FormatException('music_load_cancelled');
            }
            return plaintext;
          } finally {
            downloaded.data.fillRange(0, downloaded.data.length, 0);
          }
        },
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
