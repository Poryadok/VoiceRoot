import 'package:flutter/material.dart';

import '../../theme/voice_colors.dart';

/// Penpot v2 shell rail uses letter placeholders (Ch / Fr / MM / St) when SVG
/// library icons are missing. Flutter maps each destination to a real icon.
enum VoiceShellRailIcon {
  chats,
  friends,
  matchmaking,
  settings,
}

/// Sized rail icon with Penpot `iconMd` (20px) baseline.
class VoiceShellRailIconWidget extends StatelessWidget {
  const VoiceShellRailIconWidget({
    super.key,
    required this.icon,
    required this.selected,
  });

  final VoiceShellRailIcon icon;
  final bool selected;

  static const double size = 20;

  IconData get _iconData => switch (icon) {
        VoiceShellRailIcon.chats => Icons.chat_bubble_outline,
        VoiceShellRailIcon.friends => Icons.people_outline,
        VoiceShellRailIcon.matchmaking => Icons.sports_esports_outlined,
        VoiceShellRailIcon.settings => Icons.settings_outlined,
      };

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final color = selected ? voice.profileAccent : voice.textSecondary;
    return Icon(_iconData, size: size, color: color);
  }
}
