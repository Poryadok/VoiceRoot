import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../backend/chats_client.dart';
import '../../e2e/e2e_file_crypto.dart';
import '../../backend/files_client.dart';
import '../../backend/mention_parser.dart';
import '../../backend/messages_client.dart';
import '../../backend/voice_client.dart';
import '../../state/bot_providers.dart';
import '../../state/auth_providers.dart';
import '../../state/e2e_providers.dart';
import '../../state/call_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/connectivity_providers.dart';
import '../../state/gateway_providers.dart';
import '../../state/presence_providers.dart';
import '../../state/social_providers.dart';
import '../privacy/privacy_action_errors.dart';
import '../../state/subscription_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_emoji_style.dart';
import '../core/chat_author_label.dart';
import '../core/voice_avatar.dart';
import '../core/voice_compact_banner.dart';
import '../core/voice_state_panel.dart';
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
  var _slashMenuOpen = false;
  var _executingSlash = false;
  var _inChatSearchOpen = false;
  var _highlightedMessageId = null as String?;
  var _liveMessageAnnouncement = '';
  final _inChatSearchController = TextEditingController();

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
    _inChatSearchController.dispose();
    super.dispose();
  }

  String _roomErrorText(AppLocalizations l10n, String raw) {
    return l10n.chatRoomError(privacyActionErrorMessage(l10n, raw));
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
    final isGuest = ref.watch(authControllerProvider).isGuest;
    String? groupName;
    String? spaceId;
    var slowModeSeconds = 0;
    var isGroup = false;
    VoiceChat? chatMeta;
    for (final item in ref.watch(chatListControllerProvider).items) {
      if (item.chatId == widget.chatId) {
        chatMeta = item.chat;
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
    final ephemeralMessages = ref.watch(ephemeralMessagesProvider(widget.chatId));
    final deferredInteraction = ref.watch(deferredBotInteractionProvider(widget.chatId));
    final blockChannelMainFeed =
        chatMeta?.isChannel == true && chatMeta!.allowUserMainFeed == false;
    final composerBlocked =
        isOffline || (blockChannelMainFeed && replyTarget == null);
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
    final peerIsPremium = peerId != null &&
        ref.watch(profilePremiumBadgeProvider(peerId));
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

    ref.listen(pendingChatMessageScrollProvider(widget.chatId), (prev, next) {
      if (next != null && next.isNotEmpty) {
        _scrollToMessage(next);
        ref.read(pendingChatMessageScrollProvider(widget.chatId).notifier).state =
            null;
      }
    });

    ref.listen(pendingChatMessageHighlightProvider(widget.chatId), (prev, next) {
      if (next != null && next.isNotEmpty) {
        setState(() => _highlightedMessageId = next);
        Future<void>.delayed(const Duration(seconds: 2), () {
          if (mounted && _highlightedMessageId == next) {
            setState(() => _highlightedMessageId = null);
          }
        });
        ref
            .read(pendingChatMessageHighlightProvider(widget.chatId).notifier)
            .state = null;
      }
    });

    ref.listen(composerFocusRequestProvider, (prev, next) {
      if (next != (prev ?? 0)) {
        _refocusComposer();
      }
    });

    ref.listen(chatMessageReactionRequestProvider(widget.chatId), (prev, next) {
      if (next == null || next.isEmpty) return;
      final messages = ref.read(chatRoomControllerProvider(widget.chatId)).messages;
      VoiceMessage? message;
      for (final item in messages) {
        if (item.id == next) {
          message = item;
          break;
        }
      }
      ref.read(chatMessageReactionRequestProvider(widget.chatId).notifier).state =
          null;
      if (message != null) {
        unawaited(_showMessageActions(message, message.senderProfileId == activeId));
      }
    });

    ref.listen(chatMessageContextMenuRequestProvider(widget.chatId), (prev, next) {
      if (next == null || next.isEmpty) return;
      final messages = ref.read(chatRoomControllerProvider(widget.chatId)).messages;
      VoiceMessage? message;
      for (final item in messages) {
        if (item.id == next) {
          message = item;
          break;
        }
      }
      ref.read(chatMessageContextMenuRequestProvider(widget.chatId).notifier).state =
          null;
      if (message != null) {
        unawaited(_showMessageActions(message, message.senderProfileId == activeId));
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
      _composer.selection = TextSelection.collapsed(offset: _composer.text.length);
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
                  child: _inChatSearchOpen
                      ? KeyedSubtree(
                          key: const Key('in_chat_search_inline_header'),
                          child: TextField(
                            key: InChatSearch.searchFieldKey,
                            controller: _inChatSearchController,
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
                          verificationType:
                              peerProfile.verificationType,
                          style: Theme.of(context).textTheme.titleMedium,
                          premiumBadgeSemanticLabel: l10n.premiumBadgeLabel,
                          verifiedBadgeSemanticLabel:
                              peerProfile.verificationType == 'organization'
                              ? l10n.verifiedBadgeOrganization
                              : l10n.verifiedBadgePersonal,
                        )
                      : Text(
                          title,
                          style: Theme.of(context).textTheme.titleMedium,
                        ),
                ),
                IconButton(
                  key: ChatRoomPanel.inChatSearchKey,
                  tooltip: l10n.inChatSearchOpen,
                  onPressed: () {
                    setState(() {
                      _inChatSearchOpen = !_inChatSearchOpen;
                      if (!_inChatSearchOpen) {
                        _inChatSearchController.clear();
                      }
                    });
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
                if (shareUrlForChat(
                      chatId: widget.chatId,
                      spaceId: spaceId,
                    ) !=
                    null)
                  VoiceShareLinkButton(
                    link: shareUrlForChat(
                      chatId: widget.chatId,
                      spaceId: spaceId,
                    )!,
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
                            .startCall(
                              chatId: widget.chatId,
                              calleeProfileId: peerId,
                            ),
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
              ],
            ),
          ),
        ),
        if (room.isLoading) const LinearProgressIndicator(minHeight: 2),
        if (ref.watch(reconnectBannerVisibleProvider))
          VoiceCompactBanner(
            key: ChatRoomPanel.reconnectBannerKey,
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
        if (peerId != null &&
            ref.watch(e2eIdentityTrustProvider).pendingKeyChangePeers.contains(peerId))
          E2eIdentityChangeBanner(
            peerDisplayName: peerName ?? '@$shortId',
            onContinue: () {
              final bundleFuture = ref.read(voiceE2eClientProvider).getPreKeyBundle(
                authorization: ref.read(authorizationHeaderProvider)!,
                profileId: peerId,
              );
              unawaited(bundleFuture.then((result) {
                if (result is! E2eApiOk<String>) return;
                final parsed = parseSerializedPreKeyBundle(result.data);
                if (parsed == null) return;
                ref.read(e2eIdentityTrustProvider.notifier).acceptKeyChange(
                  peerId,
                  identityKeyBytesFromSerialized(
                    parsed.getIdentityKey().serialize(),
                  ),
                );
              }));
            },
            onDistrust: () {
              ref.read(e2eIdentityTrustProvider.notifier).distrustPeer(peerId);
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
        if (room.pinnedMessages.isNotEmpty)
          _PinnedMessagesBar(
            key: ChatRoomPanel.pinnedBarKey,
            pinned: room.pinnedMessages,
            label: l10n.chatPinnedBar(room.pinnedMessages.length),
            onTap: (messageId) => _scrollToMessage(messageId),
          ),
        Expanded(
          child: Stack(
            children: [
              room.messages.isEmpty &&
                      ephemeralMessages.isEmpty &&
                      !room.isLoading
                  ? room.errorMessage != null
                        ? VoiceStatePanel(
                            title: _roomErrorText(l10n, room.errorMessage!),
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
                      ephemeralMessages: ephemeralMessages,
                      deferredInteraction: deferredInteraction,
                      activeId: activeId,
                      isGroup: isGroup,
                      l10n: l10n,
                      initialUnreadCount: _initialUnreadCount,
                      highlightedMessageId: _highlightedMessageId,
                      keyboardSelectedMessageId:
                          ref.watch(chatMessageKeyboardProvider),
                      onLongPress: (msg, isMine) =>
                          _showMessageActions(msg, isMine),
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
                      onDismiss: () => setState(() {
                        _inChatSearchOpen = false;
                        _inChatSearchController.clear();
                      }),
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
              style: TextStyle(color: Theme.of(context).colorScheme.error),
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
                  ref.read(chatReplyTargetProvider(widget.chatId).notifier).state =
                      null;
                },
              ),
            ),
          ),
        SafeArea(
          top: false,
          child: Padding(
            padding: const EdgeInsets.fromLTRB(12, 4, 12, 8),
            child: Row(
              children: [
                IconButton(
                  key: ChatRoomPanel.emojiPickerKey,
                  tooltip: l10n.chatMessageAddReaction,
                  onPressed: room.isSending || composerBlocked
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
                      hintText: blockChannelMainFeed && replyTarget == null
                          ? l10n.chatChannelMainFeedBlocked
                          : l10n.chatRoomInputHint,
                      isDense: true,
                    ),
                    onChanged: (value) {
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
                    onSubmitted: room.isSending || composerBlocked ? null : (_) => _send(),
                    readOnly: composerBlocked,
                  ),
                ),
                const SizedBox(width: 8),
                IconButton(
                  key: ChatRoomPanel.attachKey,
                  tooltip: l10n.chatAttachFile,
                  onPressed: room.isSending || _uploadingAttachment || composerBlocked
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
                  onPressed: room.isSending || composerBlocked ? null : _send,
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
              parentPreview: replyTarget?.content ??
                  room.messages
                      .where((m) => m.id == activeThreadId)
                      .map((m) => m.content)
                      .firstOrNull ??
                  '',
              onClose: () {
                ref.read(chatActiveThreadProvider(widget.chatId).notifier).state =
                    null;
                ref.read(chatReplyTargetProvider(widget.chatId).notifier).state =
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
      if (replyTarget != null) {
        ref.read(chatReplyTargetProvider(widget.chatId).notifier).state = null;
      }
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
      }
      final ticket = await files.requestUpload(
        authorization: auth,
        originalName: picked.name,
        mimeType: picked.contentType,
        sizeBytes: uploadBytes.length,
        chatId: widget.chatId,
        chatType: chatType,
        isE2e: isE2eChat,
      );
      if (ticket is! FilesApiOk<FileUploadTicket>) return;
      final put = await files.putBytes(
        uploadUrl: ticket.data.presignedPutUrl,
        bytes: uploadBytes,
        mimeType: picked.contentType,
      );
      if (put is! FilesApiOk<void>) return;
      final confirmed = await files.confirmUpload(
        authorization: auth,
        fileId: ticket.data.fileId,
        bytes: uploadBytes,
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
                e2eKeyWire: e2eKeyWire,
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
    String? spaceId;
    for (final item in ref.read(chatListControllerProvider).items) {
      if (item.chatId == widget.chatId) {
        spaceId = item.chat.spaceId;
        break;
      }
    }
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
                  leading: const Icon(Icons.reply_outlined),
                  title: Text(sheetL10n.chatMessageReply),
                  onTap: () => Navigator.of(context).pop('reply'),
                ),
                ListTile(
                  leading: const Icon(Icons.forward_outlined),
                  title: Text(sheetL10n.chatMessageForward),
                  onTap: () => Navigator.of(context).pop('forward'),
                ),
                ListTile(
                  leading: const Icon(Icons.link),
                  title: Text(sheetL10n.shareLinkAction),
                  onTap: () => Navigator.of(context).pop('share'),
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
    } else if (action == 'reply') {
      ref.read(chatReplyTargetProvider(widget.chatId).notifier).state = message;
      ref.read(chatActiveThreadProvider(widget.chatId).notifier).state =
          message.id;
      _refocusComposer();
    } else if (action == 'forward') {
      await ForwardMessageSheet.show(
        context,
        sourceMessage: message,
        sourceChatId: widget.chatId,
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
                  icon: Text(emoji, style: VoiceEmojiStyle.textStyle(fontSize: 28)),
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
      itemCount: rows.length + ephemeralCount + deferredCount + (hasOlderControl ? 1 : 0),
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
              ),
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

  static bool _isHttpUrl(String? value) {
    if (value == null || value.isEmpty) return false;
    final uri = Uri.tryParse(value);
    return uri != null && (uri.scheme == 'http' || uri.scheme == 'https');
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final voice = VoiceColors.of(context);
    if (attachment.isImage) {
      if (attachment.isE2eEncrypted) {
        final decryptRequest = E2eAttachmentDecryptRequest(
          fileId: attachment.fileId,
          e2eKeyWire: attachment.e2eKeyWire!,
          senderProfileId: senderProfileId,
          chatId: chatId,
        );
        final bytesAsync =
            ref.watch(e2eDecryptedAttachmentThumbProvider(decryptRequest));
        return Semantics(
          key: ChatRoomPanel.attachmentPreviewKey(attachment.fileId),
          label: attachment.name ??
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
    if (attachment.isE2eEncrypted) {
      final l10n = AppLocalizations.of(context)!;
      final decryptRequest = E2eAttachmentDecryptRequest(
        fileId: attachment.fileId,
        e2eKeyWire: attachment.e2eKeyWire!,
        senderProfileId: senderProfileId,
        chatId: chatId,
      );
      final bytesAsync =
          ref.watch(e2eDecryptedAttachmentBytesProvider(decryptRequest));
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

  Future<void> _downloadE2eAttachment(
    BuildContext context,
    WidgetRef ref, {
    required E2eAttachmentDecryptRequest decryptRequest,
    required String fileName,
    Uint8List? cachedBytes,
  }) async {
    final l10n = AppLocalizations.of(context)!;
    final bytes = cachedBytes ??
        await ref.read(e2eDecryptedAttachmentBytesProvider(decryptRequest).future);
    if (!context.mounted) return;
    if (bytes == null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.e2eAttachmentDecryptFailed)),
      );
      return;
    }
    final saved = await saveDecryptedE2eAttachment(
      bytes: bytes,
      fileName: fileName,
    );
    if (!context.mounted) return;
    if (!saved) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.e2eAttachmentDownloadFailed)),
      );
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

class _EphemeralBotBubble extends StatelessWidget {
  const _EphemeralBotBubble({
    required this.message,
    required this.l10n,
  });

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
    final displayName = profileAsync.valueOrNull?.displayName ??
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
          verificationType: profileAsync.valueOrNull?.verificationType ?? 'none',
          style: Theme.of(context).textTheme.labelMedium,
          premiumBadgeSemanticLabel:
              AppLocalizations.of(context)!.premiumBadgeLabel,
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
