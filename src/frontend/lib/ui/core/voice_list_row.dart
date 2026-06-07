import 'package:flutter/material.dart';

import '../../theme/voice_colors.dart';
import '../../theme/voice_metrics.dart';

/// Compact list row: leading | title + subtitle | trailing.
class VoiceListRow extends StatelessWidget {
  const VoiceListRow({
    super.key,
    required this.title,
    this.subtitle,
    this.leading,
    this.trailing,
    this.selected = false,
    this.onTap,
  });

  final Widget? leading;
  final String title;
  final String? subtitle;
  final Widget? trailing;
  final bool selected;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final padH = context.voiceMetrics.spacing('12', fallback: 12);
    final padV = context.voiceMetrics.spacing('8', fallback: 8);
    return Material(
      color: selected
          ? voice.profileAccent.withValues(alpha: 0.12)
          : Colors.transparent,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: EdgeInsets.symmetric(horizontal: padH, vertical: padV),
          child: Row(
            children: [
              if (leading != null) ...[leading!, SizedBox(width: padH)],
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      title,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: Theme.of(context).textTheme.bodyLarge,
                    ),
                    if (subtitle != null && subtitle!.isNotEmpty)
                      Text(
                        subtitle!,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: Theme.of(context).textTheme.bodySmall?.copyWith(
                          color: voice.textSecondary,
                        ),
                      ),
                  ],
                ),
              ),
              if (trailing != null) ...[SizedBox(width: padH / 2), trailing!],
            ],
          ),
        ),
      ),
    );
  }
}
