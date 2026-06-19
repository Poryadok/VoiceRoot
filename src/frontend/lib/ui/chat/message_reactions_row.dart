import 'package:flutter/material.dart';

import '../../backend/messages_client.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_emoji_style.dart';

class MessageReactionsRow extends StatelessWidget {
  const MessageReactionsRow({
    super.key,
    required this.message,
    required this.isMine,
    required this.onToggle,
  });

  final VoiceMessage message;
  final bool isMine;
  final void Function(String emoji, bool reactedByMe) onToggle;

  @override
  Widget build(BuildContext context) {
    if (message.reactions.isEmpty) {
      return const SizedBox.shrink();
    }
    final voice = VoiceColors.of(context);
    return Padding(
      padding: EdgeInsets.only(
        top: 4,
        left: isMine ? 0 : 8,
        right: isMine ? 8 : 0,
      ),
      child: Align(
        alignment: isMine ? Alignment.centerRight : Alignment.centerLeft,
        child: Wrap(
          spacing: 4,
          runSpacing: 4,
          children: [
            for (final reaction in message.reactions)
              _ReactionChip(
                key: ValueKey('message_reaction_${message.id}_${reaction.emoji}'),
                emoji: reaction.emoji,
                count: reaction.count,
                reactedByMe: reaction.reactedByMe,
                voice: voice,
                onTap: () => onToggle(reaction.emoji, reaction.reactedByMe),
              ),
          ],
        ),
      ),
    );
  }
}

class _ReactionChip extends StatelessWidget {
  const _ReactionChip({
    super.key,
    required this.emoji,
    required this.count,
    required this.reactedByMe,
    required this.voice,
    required this.onTap,
  });

  final String emoji;
  final int count;
  final bool reactedByMe;
  final VoiceColors voice;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: reactedByMe ? voice.profileAccent.withValues(alpha: 0.18) : voice.elevated,
      borderRadius: BorderRadius.circular(12),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(emoji, style: VoiceEmojiStyle.textStyle(fontSize: 14)),
              if (count > 1) ...[
                const SizedBox(width: 4),
                Text(
                  '$count',
                  style: Theme.of(context).textTheme.labelSmall?.copyWith(
                    color: voice.textSecondary,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
