import 'package:flutter/material.dart';

import '../../theme/voice_colors.dart';

/// Premium ★ badge shown next to display names.
class PremiumBadge extends StatelessWidget {
  const PremiumBadge({super.key, this.semanticLabel});

  static const Key badgeKey = Key('premium_badge');

  final String? semanticLabel;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final badge = Text(
      '★',
      style: Theme.of(context).textTheme.labelMedium?.copyWith(
        color: voice.profileAccent,
        fontWeight: FontWeight.w700,
      ),
    );
    return Semantics(
      label: semanticLabel,
      child: KeyedSubtree(key: badgeKey, child: badge),
    );
  }
}
