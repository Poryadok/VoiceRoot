import 'package:flutter/material.dart';

import '../../backend/users_client.dart';
import '../../theme/voice_colors.dart';

/// Online-status dot for a profile presence (Phase 1).
class PresenceIndicator extends StatelessWidget {
  const PresenceIndicator({
    super.key,
    required this.presence,
    this.size = 10,
    this.semanticLabel,
  });

  final VoicePresence? presence;
  final double size;
  final String? semanticLabel;

  static Color colorFor(BuildContext context, VoicePresence? presence) {
    final voice = VoiceColors.of(context);
    if (presence == null) return voice.textDisabled;
    return switch (presence.status) {
      'online' => voice.profileAccent,
      'idle' => voice.focusRing,
      'dnd' => Theme.of(context).colorScheme.error,
      _ => voice.textDisabled,
    };
  }

  @override
  Widget build(BuildContext context) {
    return Semantics(
      label: semanticLabel ?? presence?.status ?? 'offline',
      child: Container(
        width: size,
        height: size,
        decoration: BoxDecoration(
          color: colorFor(context, presence),
          shape: BoxShape.circle,
          border: Border.all(
            color: Theme.of(context).colorScheme.surface,
            width: 1.5,
          ),
        ),
      ),
    );
  }
}
