import 'package:flutter/material.dart';

import '../../theme/voice_colors.dart';

/// Compact unread / count badge.
class VoiceBadge extends StatelessWidget {
  const VoiceBadge({super.key, required this.count, this.semanticLabel});

  final int count;
  final String? semanticLabel;

  @override
  Widget build(BuildContext context) {
    if (count <= 0) return const SizedBox.shrink();
    final voice = VoiceColors.of(context);
    final text = count > 99 ? '99+' : '$count';
    final badge = CircleAvatar(
      radius: 10,
      backgroundColor: voice.profileAccent,
      foregroundColor: Theme.of(context).colorScheme.onPrimary,
      child: Text(text, style: Theme.of(context).textTheme.labelSmall),
    );
    if (semanticLabel == null) return badge;
    return Semantics(label: semanticLabel, child: badge);
  }
}
