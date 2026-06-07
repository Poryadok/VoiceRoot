import 'package:flutter/material.dart';

import '../../theme/voice_colors.dart';
import '../../theme/voice_metrics.dart';

/// Inline banner for reconnect, errors, or gateway issues.
class VoiceCompactBanner extends StatelessWidget {
  const VoiceCompactBanner({
    super.key,
    required this.message,
    this.icon = Icons.info_outline,
    this.actionLabel,
    this.onAction,
    this.tone = VoiceBannerTone.neutral,
  });

  final String message;
  final IconData icon;
  final String? actionLabel;
  final VoidCallback? onAction;
  final VoiceBannerTone tone;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final pad = context.voiceMetrics.spacing('12', fallback: 12);
    final (bg, fg) = switch (tone) {
      VoiceBannerTone.error => (voice.error.withValues(alpha: 0.15), voice.error),
      VoiceBannerTone.warning => (voice.focusRing.withValues(alpha: 0.12), voice.textPrimary),
      VoiceBannerTone.neutral => (voice.elevated, voice.textSecondary),
    };
    return Material(
      color: bg,
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: pad, vertical: pad / 2),
        child: Row(
          children: [
            Icon(icon, size: 16, color: fg),
            SizedBox(width: pad / 2),
            Expanded(
              child: Text(
                message,
                style: Theme.of(context).textTheme.bodySmall?.copyWith(color: fg),
              ),
            ),
            if (actionLabel != null && onAction != null)
              TextButton(
                onPressed: onAction,
                child: Text(actionLabel!),
              ),
          ],
        ),
      ),
    );
  }
}

enum VoiceBannerTone { neutral, warning, error }
