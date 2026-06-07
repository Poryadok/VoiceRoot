import 'package:flutter/material.dart';

/// Draggable bottom sheet capped at 85% viewport height.
Future<T?> showVoiceBottomSheet<T>({
  required BuildContext context,
  required Widget child,
  double initialSize = 0.85,
  double minSize = 0.4,
  double maxSize = 0.95,
}) {
  return showModalBottomSheet<T>(
    context: context,
    isScrollControlled: true,
    builder: (ctx) => DraggableScrollableSheet(
      expand: false,
      initialChildSize: initialSize,
      minChildSize: minSize,
      maxChildSize: maxSize,
      builder: (_, scrollController) => SingleChildScrollView(
        controller: scrollController,
        child: child,
      ),
    ),
  );
}
