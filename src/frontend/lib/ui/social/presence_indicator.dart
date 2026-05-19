import 'package:flutter/material.dart';

import '../../backend/users_client.dart';

/// Online-status dot for a profile presence (Phase 1).
class PresenceIndicator extends StatelessWidget {
  const PresenceIndicator({
    super.key,
    required this.presence,
    this.size = 10,
  });

  final VoicePresence? presence;
  final double size;

  static Color colorFor(VoicePresence? presence) {
    if (presence == null) return Colors.grey;
    return switch (presence.status) {
      'online' => Colors.green,
      'idle' => Colors.amber,
      'dnd' => Colors.red,
      _ => Colors.grey,
    };
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        color: colorFor(presence),
        shape: BoxShape.circle,
        border: Border.all(color: Theme.of(context).colorScheme.surface, width: 1.5),
      ),
    );
  }
}
