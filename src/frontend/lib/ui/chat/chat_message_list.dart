import 'package:flutter/material.dart';

import '../../backend/messages_client.dart';
import '../../l10n/app_localizations.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_chat_bubble.dart';

const _groupGapMinutes = 5;

class ChatMessageRow {
  const ChatMessageRow({
    required this.message,
    this.showTimestamp = false,
    this.unreadSeparator = false,
  });

  final VoiceMessage message;
  final bool showTimestamp;
  final bool unreadSeparator;
}

List<ChatMessageRow> buildChatMessageRows({
  required List<VoiceMessage> messages,
  required int unreadCount,
}) {
  if (messages.isEmpty) return const [];
  final separatorIndex = unreadCount > 0 && unreadCount <= messages.length
      ? messages.length - unreadCount
      : -1;
  final rows = <ChatMessageRow>[];
  for (var i = 0; i < messages.length; i++) {
    final msg = messages[i];
    final next = i + 1 < messages.length ? messages[i + 1] : null;
    final showTimestamp = next == null || !_sameGroup(msg, next);
    rows.add(
      ChatMessageRow(
        message: msg,
        showTimestamp: showTimestamp,
        unreadSeparator: i == separatorIndex,
      ),
    );
  }
  return rows;
}

bool _sameGroup(VoiceMessage a, VoiceMessage b) {
  if (a.senderProfileId != b.senderProfileId) return false;
  final at = a.createdAt;
  final bt = b.createdAt;
  if (at == null || bt == null) return true;
  return bt.difference(at).inMinutes < _groupGapMinutes;
}

String formatMessageTime(DateTime? time) {
  if (time == null) return '';
  final h = time.hour.toString().padLeft(2, '0');
  final m = time.minute.toString().padLeft(2, '0');
  return '$h:$m';
}

class ChatUnreadSeparator extends StatelessWidget {
  const ChatUnreadSeparator({super.key, required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 12),
      child: Row(
        children: [
          Expanded(child: Divider(color: voice.borderDefault)),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 8),
            child: Text(
              label,
              style: Theme.of(context).textTheme.labelSmall?.copyWith(
                color: voice.textSecondary,
              ),
            ),
          ),
          Expanded(child: Divider(color: voice.borderDefault)),
        ],
      ),
    );
  }
}

class ChatNewMessagesChip extends StatelessWidget {
  const ChatNewMessagesChip({
    super.key,
    required this.label,
    required this.onTap,
  });

  final String label;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.only(bottom: 8),
        child: Material(
          color: voice.elevated,
          borderRadius: BorderRadius.circular(16),
          child: InkWell(
            onTap: onTap,
            borderRadius: BorderRadius.circular(16),
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
              child: Text(label, style: Theme.of(context).textTheme.labelLarge),
            ),
          ),
        ),
      ),
    );
  }
}

class ChatMessageBubbleTile extends StatelessWidget {
  const ChatMessageBubbleTile({
    super.key,
    required this.message,
    required this.isMine,
    required this.showTimestamp,
    required this.l10n,
    required this.content,
    this.deliveryFooter,
  });

  final VoiceMessage message;
  final bool isMine;
  final bool showTimestamp;
  final AppLocalizations l10n;
  final Widget content;
  final Widget? deliveryFooter;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    return VoiceChatBubble(
      isMine: isMine,
      showTailSpacing: showTimestamp,
      footer: deliveryFooter,
      child: Column(
        crossAxisAlignment:
            isMine ? CrossAxisAlignment.end : CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          content,
          if (showTimestamp && message.createdAt != null)
            Padding(
              padding: const EdgeInsets.only(top: 4),
              child: Text(
                formatMessageTime(message.createdAt),
                style: Theme.of(context).textTheme.labelSmall?.copyWith(
                  color: voice.textDisabled,
                ),
              ),
            ),
        ],
      ),
    );
  }
}
