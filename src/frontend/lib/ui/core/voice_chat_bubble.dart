import 'package:flutter/material.dart';

import '../../theme/voice_colors.dart';
import '../../theme/voice_metrics.dart';

/// Message bubble shell (mine / theirs).
class VoiceChatBubble extends StatelessWidget {
  const VoiceChatBubble({
    super.key,
    required this.isMine,
    required this.child,
    this.showTailSpacing = true,
    this.footer,
  });

  final bool isMine;
  final Widget child;
  final bool showTailSpacing;
  final Widget? footer;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final radius = context.voiceMetrics.corner('sm', fallback: 4);
    final padH = context.voiceMetrics.spacing('12', fallback: 12);
    final padV = context.voiceMetrics.spacing('8', fallback: 8);
    return Align(
      alignment: isMine ? Alignment.centerRight : Alignment.centerLeft,
      child: Container(
        margin: EdgeInsets.only(bottom: showTailSpacing ? padV : padV / 2),
        padding: EdgeInsets.symmetric(horizontal: padH, vertical: padV),
        constraints: const BoxConstraints(maxWidth: 480),
        decoration: BoxDecoration(
          color: isMine
              ? voice.profileAccent.withValues(alpha: 0.22)
              : voice.elevated,
          borderRadius: BorderRadius.circular(radius),
          border: Border.all(
            color: isMine ? voice.profileAccent.withValues(alpha: 0.35) : voice.borderDefault,
          ),
        ),
        child: Column(
          crossAxisAlignment:
              isMine ? CrossAxisAlignment.end : CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            child,
            if (footer != null) ...[SizedBox(height: padV / 4), footer!],
          ],
        ),
      ),
    );
  }
}
