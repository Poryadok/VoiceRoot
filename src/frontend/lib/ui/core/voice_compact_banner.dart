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
    this.networkLayout,
    this.isReconnecting = false,
    this.tone = VoiceBannerTone.neutral,
  });

  final String message;
  final String? detail;
  final IconData icon;
  final String? actionLabel;
  final VoidCallback? onAction;
  final VoidCallback? onDismiss;
  final VoiceNetworkBannerLayout? networkLayout;
  final bool isReconnecting;
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
    final metrics = context.voiceMetrics;
    final networkTitleStyle = metrics.typeStyle('label');
    final text = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          message,
          style:
              (networkLayout == null
                      ? Theme.of(context).textTheme.bodySmall
                      : networkTitleStyle?.toTextStyle(color: fg) ??
                            const TextStyle(
                              fontSize: 14,
                              fontWeight: FontWeight.w500,
                              height: 20 / 14,
                            ))
                  ?.copyWith(color: fg),
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
        if (networkLayout == null)
          TextButton(onPressed: onAction, child: Text(actionLabel!))
        else
          OutlinedButton.icon(
            onPressed: onAction,
            icon: const Icon(Icons.refresh, size: 16),
            label: Text(actionLabel!),
            style: OutlinedButton.styleFrom(
              minimumSize: const Size(0, 36),
              padding: const EdgeInsets.symmetric(horizontal: 12),
              visualDensity: VisualDensity.compact,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(
                  metrics.corner('md', fallback: 6),
                ),
              ),
              side: BorderSide(color: voice.borderDefault),
            ),
          ),
      if (networkLayout != null &&
          actionLabel != null &&
          onAction != null &&
          onDismiss != null)
        const SizedBox(width: 8),
      if (onDismiss != null)
        IconButton(
          tooltip: MaterialLocalizations.of(context).closeButtonTooltip,
          onPressed: onDismiss,
          icon: const Icon(Icons.close),
          visualDensity: VisualDensity.compact,
        ),
    ];

    final content = LayoutBuilder(
      builder: (context, constraints) {
        final narrow = VoiceLayout.isNarrow(constraints.maxWidth);
        final gap = switch (networkLayout) {
          VoiceNetworkBannerLayout.desktop => 12.0,
          VoiceNetworkBannerLayout.phone => 10.0,
          null => pad / 2,
        };
        final Widget leading = switch (networkLayout) {
          VoiceNetworkBannerLayout.desktop => Container(
            key: const Key('network_offline_mark'),
            width: 36,
            height: 36,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: voice.elevated,
              borderRadius: BorderRadius.circular(
                metrics.corner('lg', fallback: 8),
              ),
            ),
            child: Icon(icon, size: 20, color: voice.warning),
          ),
          VoiceNetworkBannerLayout.phone when isReconnecting => SizedBox(
            key: const Key('network_reconnecting_progress'),
            width: 22,
            height: 22,
            child: CircularProgressIndicator(
              strokeWidth: 2,
              color: voice.warning,
            ),
          ),
          VoiceNetworkBannerLayout.phone => SizedBox(
            key: const Key('network_offline_mark'),
            width: 22,
            height: 22,
            child: Icon(icon, size: 16, color: voice.warning),
          ),
          null => Icon(icon, size: 16, color: fg),
        };
        final iconAndText = Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            leading,
            SizedBox(width: gap),
            Expanded(child: text),
          ],
        );

        if (!narrow) {
          return Row(
            children: [
              leading,
              SizedBox(width: gap),
              Expanded(child: text),
              ...actions,
            ],
          );
        }

        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            iconAndText,
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
    );

    final layout = networkLayout;
    if (layout == null) {
      return Material(
        color: bg,
        child: Padding(
          padding: EdgeInsets.symmetric(horizontal: pad, vertical: pad / 2),
          child: content,
        ),
      );
    }

    final isPhone = layout == VoiceNetworkBannerLayout.phone;
    return Container(
      margin: isPhone
          ? const EdgeInsets.fromLTRB(16, 12, 16, 4)
          : const EdgeInsets.fromLTRB(12, 10, 12, 12),
      padding: isPhone
          ? const EdgeInsets.fromLTRB(12, 10, 8, 10)
          : const EdgeInsets.fromLTRB(12, 10, 10, 10),
      decoration: BoxDecoration(
        color: voice.canvas,
        border: Border.all(color: voice.dividerRail, width: 1),
        borderRadius: BorderRadius.circular(metrics.corner('md', fallback: 6)),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: isPhone ? 0.09 : 0.12),
            offset: Offset(0, isPhone ? 5 : 8),
            blurRadius: isPhone ? 16 : 24,
          ),
        ],
      ),
      child: Material(color: Colors.transparent, child: content),
    );
  }
}

enum VoiceBannerTone { neutral, warning, error }

enum VoiceNetworkBannerLayout { desktop, phone }
