import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../backend/voice_client.dart';
import '../../state/auth_providers.dart';
import '../../state/call_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/gateway_providers.dart';
import '../../state/presence_providers.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_state_panel.dart';
import '../core/voice_send_button.dart';
import '../social/presence_indicator.dart';

/// Main column: message history (REST) + composer; live updates via Realtime WS.
class ChatRoomPanel extends ConsumerStatefulWidget {
  const ChatRoomPanel({super.key, required this.chatId, this.onBack});

  static const Key panelKey = Key('chat_room_panel');
  static const Key messagesKey = Key('chat_room_messages');
  static const Key inputKey = Key('chat_room_input');
  static const Key sendKey = Key('chat_room_send');
  static const Key peerPresenceKey = Key('chat_room_peer_presence');
  static const Key loadOlderKey = Key('chat_room_load_older');
  static const Key audioCallKey = Key('chat_room_audio_call');
  static const Key videoCallKey = Key('chat_room_video_call');

  final String chatId;
  final VoidCallback? onBack;

  @override
  ConsumerState<ChatRoomPanel> createState() => _ChatRoomPanelState();
}

class _ChatRoomPanelState extends ConsumerState<ChatRoomPanel> {
  final _composer = TextEditingController();
  final _scrollController = ScrollController();

  @override
  void dispose() {
    _composer.dispose();
    _scrollController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final room = ref.watch(chatRoomControllerProvider(widget.chatId));
    final activeId = ref.watch(authControllerProvider).activeProfileId;
    final peerId = ref.watch(dmPeerProfileByChatIdProvider)[widget.chatId];
    final peerName = peerId != null
        ? ref.watch(profileProvider(peerId)).valueOrNull?.displayName
        : null;
    final peerPresence = peerId != null
        ? ref.watch(presenceProvider(peerId))
        : null;
    final canCall = ref.watch(gatewayConfigProvider).hasLivekitUrl;
    final title = peerName ?? l10n.chatRoomTitle(widget.chatId.substring(0, 8));
    final voice = VoiceColors.of(context);

    ref.listen(chatRoomControllerProvider(widget.chatId), (prev, next) {
      if ((prev?.messages.length ?? 0) < next.messages.length) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (_scrollController.hasClients) {
            _scrollController.animateTo(
              _scrollController.position.maxScrollExtent,
              duration: const Duration(milliseconds: 200),
              curve: Curves.easeOut,
            );
          }
        });
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
              : ListView.builder(
                  key: ChatRoomPanel.messagesKey,
                  controller: _scrollController,
                  padding: const EdgeInsets.all(12),
                  itemCount:
                      room.messages.length +
                      (room.hasMore || room.isLoadingOlder ? 1 : 0),
                  itemBuilder: (context, index) {
                    final hasOlderControl = room.hasMore || room.isLoadingOlder;
                    if (hasOlderControl && index == 0) {
                      return Center(
                        child: room.isLoadingOlder
                            ? const Padding(
                                padding: EdgeInsets.all(8),
                                child: SizedBox(
                                  width: 24,
                                  height: 24,
                                  child: CircularProgressIndicator(
                                    strokeWidth: 2,
                                  ),
                                ),
                              )
                            : TextButton.icon(
                                key: ChatRoomPanel.loadOlderKey,
                                icon: const Icon(Icons.expand_less),
                                label: Text(l10n.chatRoomLoadOlder),
                                onPressed: () => ref
                                    .read(
                                      chatRoomControllerProvider(
                                        widget.chatId,
                                      ).notifier,
                                    )
                                    .loadOlderMessages(),
                              ),
                      );
                    }
                    final messageIndex = hasOlderControl ? index - 1 : index;
                    final msg = room.messages[messageIndex];
                    final isMine = msg.senderProfileId == activeId;
                    return Align(
                      alignment: isMine
                          ? Alignment.centerRight
                          : Alignment.centerLeft,
                      child: Container(
                        margin: const EdgeInsets.only(bottom: 8),
                        padding: const EdgeInsets.symmetric(
                          horizontal: 12,
                          vertical: 8,
                        ),
                        decoration: BoxDecoration(
                          color: isMine
                              ? voice.profileAccent.withValues(alpha: 0.22)
                              : voice.elevated,
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: Text(msg.content),
                      ),
                    );
                  },
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
                    decoration: InputDecoration(
                      hintText: l10n.chatRoomInputHint,
                      isDense: true,
                    ),
                    onSubmitted: room.isSending ? null : (_) => _send(),
                  ),
                ),
                const SizedBox(width: 8),
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
    final err = await ref
        .read(chatRoomControllerProvider(widget.chatId).notifier)
        .sendMessage(text);
    if (!mounted) return;
    if (err == null) {
      _composer.clear();
    }
  }
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
