import 'package:flutter/material.dart';

/// Traps focus inside modal sheets until dismissed (docs/features/accessibility.md).
class VoiceFocusTrap extends StatelessWidget {
  const VoiceFocusTrap({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    return FocusTraversalGroup(
      policy: OrderedTraversalPolicy(),
      child: FocusScope(
        autofocus: true,
        child: child,
      ),
    );
  }
}
