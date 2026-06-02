import 'package:flutter/material.dart';

import '../../theme/voice_colors.dart';

/// Small indicator of the active profile accent color.
class ProfileAccentDot extends StatelessWidget {
  const ProfileAccentDot({super.key, this.size = 10});

  final double size;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        color: VoiceColors.of(context).profileAccent,
        shape: BoxShape.circle,
        border: Border.all(
          color: VoiceColors.of(context).borderDefault,
        ),
      ),
    );
  }
}
