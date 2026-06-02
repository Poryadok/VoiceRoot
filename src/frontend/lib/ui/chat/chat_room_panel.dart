import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/presence_providers.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_send_button.dart';
import '../social/presence_indicator.dart';

/// Main column: message history (REST) + composer; live updates via Realtime WS.
class ChatRoomPanel extends ConsumerStatefulWidget {
  const ChatRoomPanel({super.key, required this.chatId});

  static const Key panelKey = Key('chat_room_panel');
  static const Key messagesKey = Key('chat_room_messages');
  static const Key inputKey = Key('chat_room_input');
  static const Key sendKey = Key('chat_room_send');
  static const Key peerPresenceKey = Key('chat_room_peer_presence');

  final String chatId;

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
    final peerPresence =
        peerId != null ? ref.watch(presenceProvider(peerId)) : null;
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
                if (peerPresence != null) ...[
                  PresenceIndicator(
                    key: ChatRoomPanel.peerPresenceKey,
                    presence: peerPresence,
                    size: 10,
                  ),
                  const SizedBox(width: 8),
                ],
                Expanded(
                  child: Text(title, style: Theme.of(context).textTheme.titleMedium),
                ),
                _RealtimeBadge(status: room.realtimeStatus, l10n: l10n),
              ],
            ),
          ),
        ),
        if (room.isLoading) const LinearProgressIndicator(minHeight: 2),
        Expanded(
          child: room.messages.isEmpty && !room.isLoading
              ? Center(child: Text(l10n.chatRoomEmpty))
              : ListView.builder(
                  key: ChatRoomPanel.messagesKey,
                  controller: _scrollController,
                  padding: const EdgeInsets.all(12),
                  itemCount: room.messages.length,
                  itemBuilder: (context, index) {
                    final msg = room.messages[index];
                    final isMine = msg.senderProfileId == activeId;
                    return Align(
                      alignment:
                          isMine ? Alignment.centerRight : Alignment.centerLeft,
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
        if (room.errorMessage != null)
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
                      border: const OutlineInputBorder(),
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

class _RealtimeBadge extends StatelessWidget {
  const _RealtimeBadge({required this.status, required this.l10n});

  final RealtimeLinkStatus status;
  final AppLocalizations l10n;

  @override
  Widget build(BuildContext context) {
    final (label, color) = switch (status) {
      RealtimeLinkStatus.connected => (l10n.chatRealtimeConnected, Colors.green),
      RealtimeLinkStatus.connecting => (l10n.chatRealtimeConnecting, Colors.orange),
      RealtimeLinkStatus.reconnecting => (l10n.chatRealtimeReconnecting, Colors.orange),
      RealtimeLinkStatus.disconnected => (l10n.chatRealtimeOffline, Colors.grey),
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
