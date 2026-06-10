import 'package:flutter/material.dart';

import '../../theme/voice_colors.dart';

/// Avatar with network image and initials fallback.
class VoiceAvatar extends StatelessWidget {
  const VoiceAvatar({
    super.key,
    this.imageUrl,
    required this.label,
    this.radius = 20,
  });

  final String? imageUrl;
  final String label;
  final double radius;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final initial = label.isNotEmpty ? label[0].toUpperCase() : '?';
    final url = imageUrl;
    if (url == null || url.isEmpty) {
      return CircleAvatar(
        radius: radius,
        backgroundColor: voice.muted,
        foregroundColor: voice.textPrimary,
        child: Text(initial),
      );
    }
    return CircleAvatar(
      radius: radius,
      backgroundColor: voice.muted,
      foregroundColor: voice.textPrimary,
      child: ClipOval(
        child: Image.network(
          url,
          width: radius * 2,
          height: radius * 2,
          fit: BoxFit.cover,
          errorBuilder: (_, _, _) => Center(child: Text(initial)),
        ),
      ),
    );
  }
}

/// Avatar with optional presence dot (bottom-right).
class VoiceAvatarWithPresence extends StatelessWidget {
  const VoiceAvatarWithPresence({
    super.key,
    required this.avatar,
    this.presence,
    this.presenceSize = 12,
  });

  final Widget avatar;
  final Widget? presence;
  final double presenceSize;

  @override
  Widget build(BuildContext context) {
    if (presence == null) return avatar;
    return Stack(
      clipBehavior: Clip.none,
      children: [
        avatar,
        Positioned(
          right: -2,
          bottom: -2,
          child: SizedBox(width: presenceSize, height: presenceSize, child: presence),
        ),
      ],
    );
  }
}
