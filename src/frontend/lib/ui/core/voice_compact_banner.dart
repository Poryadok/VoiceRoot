import 'package:flutter/material.dart';

import '../../theme/voice_colors.dart';
import '../../theme/voice_layout.dart';
import '../../theme/voice_metrics.dart';

/// Inline banner for reconnect, errors, or gateway issues.
class VoiceCompactBanner extends StatelessWidget {
  const VoiceCompactBanner({
    super.key,
    required this.message,
    this.detail,
    this.icon = Icons.info_outline,
    this.actionLabel,
    this.onAction,
    this.onDismiss,
    this.tone = VoiceBannerTone.neutral,
  });

  final String message;
  final String? detail;
  final IconData icon;
  final String? actionLabel;
  final VoidCallback? onAction;
  final VoidCallback? onDismiss;
  final VoiceBannerTone tone;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final pad = context.voiceMetrics.spacing('12', fallback: 12);
    final (bg, fg) = switch (tone) {
      VoiceBannerTone.error => (
        voice.error.withValues(alpha: 0.15),
        voice.error,
      ),
      VoiceBannerTone.warning => (
        voice.focusRing.withValues(alpha: 0.12),
        voice.textPrimary,
      ),
      VoiceBannerTone.neutral => (voice.elevated, voice.textSecondary),
    };
    final text = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          message,
          style: Theme.of(context).textTheme.bodySmall?.copyWith(color: fg),
        ),
        if (detail != null)
          Text(
            detail!,
            style: Theme.of(context).textTheme.bodySmall?.copyWith(color: fg),
          ),
      ],
    );
    final actions = <Widget>[
      if (actionLabel != null && onAction != null)
        TextButton(onPressed: onAction, child: Text(actionLabel!)),
      if (onDismiss != null)
        IconButton(
          tooltip: MaterialLocalizations.of(context).closeButtonTooltip,
          onPressed: onDismiss,
          icon: const Icon(Icons.close),
          visualDensity: VisualDensity.compact,
        ),
    ];

    return Material(
      color: bg,
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: pad, vertical: pad / 2),
        child: LayoutBuilder(
          builder: (context, constraints) {
            final narrow = VoiceLayout.isNarrow(constraints.maxWidth);
            final content = Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Icon(icon, size: 16, color: fg),
                SizedBox(width: pad / 2),
                Expanded(child: text),
              ],
            );

            if (!narrow) {
              return Row(
                children: [
                  Icon(icon, size: 16, color: fg),
                  SizedBox(width: pad / 2),
                  Expanded(child: text),
                  ...actions,
                ],
              );
            }

            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              mainAxisSize: MainAxisSize.min,
              children: [
                content,
                if (actions.isNotEmpty)
                  Align(
                    alignment: AlignmentDirectional.centerEnd,
                    child: Wrap(
                      alignment: WrapAlignment.end,
                      crossAxisAlignment: WrapCrossAlignment.center,
                      children: actions,
                    ),
                  ),
              ],
            );
          },
        ),
      ),
    );
  }
}

enum VoiceBannerTone { neutral, warning, error }
