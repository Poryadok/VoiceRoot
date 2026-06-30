import 'package:flutter/material.dart';

import '../../theme/voice_colors.dart';

/// Non-blocking coach-mark tooltip anchored to [anchorKey] (docs/features/onboarding.md).
class OnboardingCoachMark extends StatelessWidget {
  const OnboardingCoachMark({
    super.key,
    required this.anchorKey,
    required this.title,
    required this.body,
    required this.onContinue,
    required this.onSkip,
    this.continueLabel = 'Got it',
    this.secondaryLabel,
    this.onSecondary,
  });

  final GlobalKey anchorKey;
  final String title;
  final String body;
  final VoidCallback onContinue;
  final VoidCallback onSkip;
  final String continueLabel;
  final String? secondaryLabel;
  final VoidCallback? onSecondary;

  static OverlayEntry? show({
    required BuildContext context,
    required GlobalKey anchorKey,
    required String title,
    required String body,
    required VoidCallback onContinue,
    required VoidCallback onSkip,
    String continueLabel = 'Got it',
    String? secondaryLabel,
    VoidCallback? onSecondary,
  }) {
    final overlay = Overlay.of(context);
    late OverlayEntry entry;
    entry = OverlayEntry(
      builder: (ctx) => OnboardingCoachMark(
        anchorKey: anchorKey,
        title: title,
        body: body,
        continueLabel: continueLabel,
        secondaryLabel: secondaryLabel,
        onContinue: () {
          onContinue();
        },
        onSkip: () {
          onSkip();
        },
        onSecondary: secondaryLabel == null
            ? null
            : () {
                onSecondary?.call();
              },
      ),
    );
    overlay.insert(entry);
    return entry;
  }

  @override
  Widget build(BuildContext context) {
    final anchorContext = anchorKey.currentContext;
    if (anchorContext == null) {
      return const SizedBox.shrink();
    }
    final box = anchorContext.findRenderObject() as RenderBox?;
    if (box == null || !box.hasSize) {
      return const SizedBox.shrink();
    }
    final offset = box.localToGlobal(Offset.zero);
    final size = box.size;
    final voice = VoiceColors.of(context);
    final screen = MediaQuery.sizeOf(context);

    const bubbleWidth = 280.0;
    var left = offset.dx + (size.width / 2) - (bubbleWidth / 2);
    left = left.clamp(8.0, screen.width - bubbleWidth - 8);
    var top = offset.dy + size.height + 12;
    if (top + 180 > screen.height) {
      top = offset.dy - 180;
    }

    return Stack(
      children: [
        ModalBarrier(
          color: Colors.black.withValues(alpha: 0.35),
          dismissible: false,
        ),
        Positioned(
          left: left,
          top: top,
          width: bubbleWidth,
          child: Material(
            elevation: 8,
            borderRadius: BorderRadius.circular(12),
            color: voice.surface,
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(title, style: Theme.of(context).textTheme.titleSmall),
                  const SizedBox(height: 8),
                  Text(body, style: Theme.of(context).textTheme.bodyMedium),
                  const SizedBox(height: 12),
                  Wrap(
                    alignment: WrapAlignment.end,
                    spacing: 4,
                    runSpacing: 4,
                    children: [
                      TextButton(onPressed: onSkip, child: const Text('Skip')),
                      if (secondaryLabel != null && onSecondary != null)
                        TextButton(
                          onPressed: onSecondary,
                          child: Text(secondaryLabel!),
                        ),
                      FilledButton(
                        onPressed: onContinue,
                        child: Text(continueLabel),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
        ),
      ],
    );
  }
}
