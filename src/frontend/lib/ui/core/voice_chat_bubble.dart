import 'package:flutter/material.dart';

import '../../settings/chat_theme_preference.dart';
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
    this.palette,
  });

  final bool isMine;
  final Widget child;
  final bool showTailSpacing;
  final Widget? footer;
  final ChatThemePalette? palette;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final radius = context.voiceMetrics.corner('sm', fallback: 4);
    final padH = context.voiceMetrics.spacing('12', fallback: 12);
    final padV = context.voiceMetrics.spacing('8', fallback: 8);
    final baseTheme = Theme.of(context);
    final theme = palette == null
        ? baseTheme
        : baseTheme.copyWith(
            colorScheme: baseTheme.colorScheme.copyWith(
              onSurface: palette!.text,
            ),
            textTheme: baseTheme.textTheme.apply(
              bodyColor: palette!.text,
              displayColor: palette!.text,
            ),
            extensions: [
              for (final extension in baseTheme.extensions.values)
                if (extension is! VoiceColors) extension,
              voice.copyWith(
                textPrimary: palette!.text,
                textSecondary: palette!.textSecondary,
              ),
            ],
          );
    return Theme(
      data: theme,
      child: Align(
        alignment: isMine ? Alignment.centerRight : Alignment.centerLeft,
        child: Container(
          margin: EdgeInsets.only(bottom: showTailSpacing ? padV : padV / 2),
          padding: EdgeInsets.symmetric(horizontal: padH, vertical: padV),
          constraints: const BoxConstraints(maxWidth: 480),
          decoration: BoxDecoration(
            color:
                palette?.backgroundFor(isMine) ??
                (isMine
                    ? voice.profileAccent.withValues(alpha: 0.22)
                    : voice.elevated),
            borderRadius: BorderRadius.circular(radius),
            border: Border.all(
              color:
                  palette?.borderFor(isMine) ??
                  (isMine
                      ? voice.profileAccent.withValues(alpha: 0.35)
                      : voice.borderDefault),
            ),
          ),
          child: Column(
            crossAxisAlignment: isMine
                ? CrossAxisAlignment.end
                : CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              child,
              if (footer != null) ...[SizedBox(height: padV / 4), footer!],
            ],
          ),
        ),
      ),
    );
  }
}
