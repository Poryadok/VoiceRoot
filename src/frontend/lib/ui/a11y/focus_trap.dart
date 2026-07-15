import 'package:flutter/material.dart';

/// Traps focus inside modal sheets and overlays until dismissed.
///
/// Used by [showVoiceBottomSheet], guest convert, and call overlays per
/// docs/features/accessibility.md.
class VoiceFocusTrap extends StatefulWidget {
  const VoiceFocusTrap({super.key, required this.child});

  final Widget child;

  @override
  State<VoiceFocusTrap> createState() => _VoiceFocusTrapState();
}

class _VoiceFocusTrapState extends State<VoiceFocusTrap> {
  final FocusScopeNode _scopeNode = FocusScopeNode(debugLabel: 'VoiceFocusTrap');

  @override
  void dispose() {
    _scopeNode.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return FocusScope(
      node: _scopeNode,
      autofocus: true,
      child: FocusTraversalGroup(
        policy: OrderedTraversalPolicy(),
        child: widget.child,
      ),
    );
  }
}
