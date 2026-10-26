import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../settings/reduced_motion.dart';
import '../a11y/focus_trap.dart';
import '../a11y/voice_focus_return.dart';

/// Draggable bottom sheet capped at 85% viewport height.
///
/// Set [scrollable] to false for children that manage their own scrolling
/// (e.g. [TabBarView] with [Expanded]).
Future<T?> showVoiceBottomSheet<T>({
  required BuildContext context,
  required Widget child,
  double initialSize = 0.85,
  double minSize = 0.4,
  double maxSize = 0.95,
  bool scrollable = true,
  VoidCallback? onDismiss,
}) async {
  final focusReturn = VoiceFocusReturn.capture();
  final container = ProviderScope.containerOf(context, listen: false);
  await container.read(reducedMotionEnabledProvider.notifier).loaded;
  if (!context.mounted) return null;
  return showModalBottomSheet<T>(
    context: context,
    isScrollControlled: true,
    sheetAnimationStyle: panelAnimationDurationOf(context) == Duration.zero
        ? AnimationStyle.noAnimation
        : null,
    builder: (ctx) => VoiceFocusTrap(
      onEscape: () {
        Navigator.of(ctx).pop();
        focusReturn.restore();
        onDismiss?.call();
      },
      child: DraggableScrollableSheet(
        expand: false,
        initialChildSize: initialSize,
        minChildSize: minSize,
        maxChildSize: maxSize,
        builder: (_, scrollController) {
          if (!scrollable) {
            return SizedBox.expand(child: child);
          }
          return SingleChildScrollView(
            controller: scrollController,
            child: child,
          );
        },
      ),
    ),
  );
}
