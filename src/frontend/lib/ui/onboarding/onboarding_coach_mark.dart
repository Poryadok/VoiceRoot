import 'package:flutter/material.dart';

import '../../theme/voice_colors.dart';
import '../../theme/voice_metrics.dart';
import '../a11y/voice_focus_return.dart';
import '../call/call_modal_overlay.dart';

/// Non-blocking coach-mark tooltip anchored to [anchorKey] (docs/features/onboarding.md).
class OnboardingCoachMark extends StatefulWidget {
  static const cardKey = Key('onboarding_coach_mark_card');

  const OnboardingCoachMark({
    super.key,
    required this.anchorKey,
    required this.title,
    required this.body,
    required this.onContinue,
    required this.onSkip,
    required this.continueLabel,
    required this.skipLabel,
    this.secondaryLabel,
    this.onSecondary,
  });

  final GlobalKey anchorKey;
  final String title;
  final String body;
  final VoidCallback onContinue;
  final VoidCallback onSkip;
  final String continueLabel;
  final String skipLabel;
  final String? secondaryLabel;
  final VoidCallback? onSecondary;

  static OverlayEntry? show({
    required BuildContext context,
    required GlobalKey anchorKey,
    required String title,
    required String body,
    required VoidCallback onContinue,
    required VoidCallback onSkip,
    required String continueLabel,
    required String skipLabel,
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
        skipLabel: skipLabel,
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
  State<OnboardingCoachMark> createState() => _OnboardingCoachMarkState();
}

class _OnboardingCoachMarkState extends State<OnboardingCoachMark> {
  late final VoiceFocusReturn _focusReturn = VoiceFocusReturn.capture();

  void _dismiss(VoidCallback action) {
    _focusReturn.restore();
    action();
  }

  @override
  Widget build(BuildContext context) {
    final anchorContext = widget.anchorKey.currentContext;
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
    final viewPadding = MediaQuery.paddingOf(context);
    final radius = context.voiceMetrics.corner('md', fallback: 6);

    const bubbleWidth = 320.0;
    const bubbleMinHeight = 248.0;
    final safeTop = viewPadding.top + 8;
    final safeBottom = screen.height - viewPadding.bottom - 8;
    final safeLeft = viewPadding.left + 8;
    final safeRight = screen.width - viewPadding.right - 8;
    final availableWidth = (safeRight - safeLeft).clamp(0.0, screen.width);
    final cardWidth = availableWidth < bubbleWidth
        ? availableWidth
        : bubbleWidth;
    final maxLeft = safeRight - cardWidth;
    var left = offset.dx + (size.width / 2) - (cardWidth / 2);
    left = left.clamp(safeLeft, maxLeft < safeLeft ? safeLeft : maxLeft);
    final availableHeight = (safeBottom - safeTop).clamp(0.0, screen.height);
    final cardMinHeight = availableHeight < bubbleMinHeight
        ? availableHeight
        : bubbleMinHeight;
    final maxTop = safeBottom - cardMinHeight;
    final boundedMaxTop = maxTop < safeTop ? safeTop : maxTop;
    var top = offset.dy + size.height + 12;
    if (top + cardMinHeight > safeBottom) {
      top = offset.dy - cardMinHeight - 12;
    }
    top = top.clamp(safeTop, boundedMaxTop);

    return Stack(
      children: [
        IgnorePointer(
          ignoring: true,
          child: ExcludeSemantics(
            child: ModalBarrier(
              color: voice.canvas.withValues(alpha: kVoiceOverlayDimOpacity),
              dismissible: false,
            ),
          ),
        ),
        Positioned(
          left: left,
          top: top,
          width: cardWidth,
          child: Material(
            key: OnboardingCoachMark.cardKey,
            elevation: 0,
            borderRadius: BorderRadius.circular(radius),
            color: voice.elevated,
            child: ConstrainedBox(
              constraints: BoxConstraints(
                minHeight: cardMinHeight,
                maxHeight: availableHeight,
              ),
              child: SingleChildScrollView(
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        widget.title,
                        style: Theme.of(context).textTheme.titleSmall,
                      ),
                      const SizedBox(height: 8),
                      Text(
                        widget.body,
                        style: Theme.of(context).textTheme.bodyMedium,
                      ),
                      const SizedBox(height: 12),
                      Wrap(
                        alignment: WrapAlignment.end,
                        spacing: 4,
                        runSpacing: 4,
                        children: [
                          TextButton(
                            onPressed: () => _dismiss(widget.onSkip),
                            child: Text(widget.skipLabel),
                          ),
                          if (widget.secondaryLabel != null &&
                              widget.onSecondary != null)
                            TextButton(
                              onPressed: () =>
                                  _dismiss(widget.onSecondary!),
                              child: Text(widget.secondaryLabel!),
                            ),
                          FilledButton(
                            onPressed: () => _dismiss(widget.onContinue),
                            child: Text(widget.continueLabel),
                          ),
                        ],
                      ),
                    ],
                  ),
              ),
            ),
          ),
        ),
      ),
      ],
    );
  }
}
