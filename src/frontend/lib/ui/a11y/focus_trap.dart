import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// Traps focus inside modal sheets and overlays until dismissed.
///
/// Used by [showVoiceBottomSheet], guest convert, and call overlays per
/// docs/features/accessibility.md §3.6e.
class VoiceFocusTrap extends StatefulWidget {
  const VoiceFocusTrap({super.key, required this.child, this.onEscape});

  final Widget child;

  /// Called when Escape is pressed; caller should pop route and return focus.
  final VoidCallback? onEscape;

  @override
  State<VoiceFocusTrap> createState() => _VoiceFocusTrapState();
}

class _DismissFocusTrapIntent extends Intent {
  const _DismissFocusTrapIntent();
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
    return Shortcuts(
      shortcuts: const {
        SingleActivator(LogicalKeyboardKey.escape): _DismissFocusTrapIntent(),
      },
      child: Actions(
        actions: {
          _DismissFocusTrapIntent: CallbackAction<_DismissFocusTrapIntent>(
            onInvoke: (_) {
              if (widget.onEscape != null) {
                widget.onEscape!();
              } else {
                Navigator.maybePop(context);
              }
              return null;
            },
          ),
        },
        child: FocusScope(
          node: _scopeNode,
          autofocus: true,
          child: FocusTraversalGroup(
            policy: OrderedTraversalPolicy(),
            child: widget.child,
          ),
        ),
      ),
    );
  }
}
