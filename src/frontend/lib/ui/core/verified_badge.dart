import 'package:flutter/material.dart';

import '../../theme/voice_colors.dart';

/// System-icon verification badge (personal check or organization building).
class VerifiedBadge extends StatelessWidget {
  const VerifiedBadge({
    super.key,
    required this.verificationType,
    this.semanticLabel,
    this.size = 16,
  });

  static const Key personalKey = Key('verified_badge_personal');
  static const Key organizationKey = Key('verified_badge_organization');

  final String verificationType;
  final String? semanticLabel;
  final double size;

  @override
  Widget build(BuildContext context) {
    final icon = _iconForType(verificationType);
    if (icon == null) return const SizedBox.shrink();

    final voice = VoiceColors.of(context);
    final badge = Icon(
      icon,
      size: size,
      color: voice.profileAccent,
    );
    final key = verificationType == 'organization'
        ? organizationKey
        : personalKey;

    return Semantics(
      label: semanticLabel,
      child: KeyedSubtree(key: key, child: badge),
    );
  }

  IconData? _iconForType(String type) {
    return switch (type) {
      'personal' => Icons.verified,
      'organization' => Icons.apartment_outlined,
      _ => null,
    };
  }
}

bool showsVerifiedBadge(String verificationType) {
  return verificationType == 'personal' || verificationType == 'organization';
}
