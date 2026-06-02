import 'package:flutter/material.dart';

import '../../theme/voice_colors.dart';

/// Send message control — profile accent icon.
class VoiceSendButton extends StatelessWidget {
  const VoiceSendButton({
    super.key,
    required this.onPressed,
    this.isLoading = false,
    this.tooltip,
  });

  final VoidCallback? onPressed;
  final bool isLoading;
  final String? tooltip;

  @override
  Widget build(BuildContext context) {
    final accent = VoiceColors.of(context).profileAccent;
    return IconButton(
      tooltip: tooltip,
      onPressed: isLoading ? null : onPressed,
      icon: isLoading
          ? SizedBox(
              width: 20,
              height: 20,
              child: CircularProgressIndicator(strokeWidth: 2, color: accent),
            )
          : Icon(Icons.send, color: accent),
    );
  }
}
