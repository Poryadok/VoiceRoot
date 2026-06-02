import 'package:flutter/material.dart';

/// Neutral outlined action (no profile accent).
class VoiceSecondaryButton extends StatelessWidget {
  const VoiceSecondaryButton({
    super.key,
    required this.onPressed,
    required this.child,
  });

  final VoidCallback? onPressed;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return OutlinedButton(onPressed: onPressed, child: child);
  }
}
