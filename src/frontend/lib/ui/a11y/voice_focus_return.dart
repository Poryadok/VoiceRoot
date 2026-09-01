import 'package:flutter/widgets.dart';

/// Captures primary focus before a modal opens and restores it on dismiss.
///
/// Per docs/features/accessibility.md §3.6e — focus returns to the trigger
/// after coach-marks, call overlays, and composer panels close.
class VoiceFocusReturn {
  VoiceFocusReturn._(this._previous);

  final FocusNode? _previous;

  /// Remembers [FocusManager.instance.primaryFocus] at call time.
  factory VoiceFocusReturn.capture() {
    return VoiceFocusReturn._(FocusManager.instance.primaryFocus);
  }

  /// Requests focus on the captured node after the current frame.
  void restore() {
    final node = _previous;
    if (node == null || !node.canRequestFocus) return;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!node.canRequestFocus) return;
      FocusManager.instance.primaryFocus?.unfocus();
      node.requestFocus();
    });
  }
}
