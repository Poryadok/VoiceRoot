import 'package:flutter/material.dart';

import '../../theme/voice_colors.dart';
import '../../theme/voice_metrics.dart';
import '../a11y/voice_focus_return.dart';

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
    this.step = 1,
    this.totalSteps = 4,
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
  final int step;
  final int totalSteps;

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
    int step = 1,
    int totalSteps = 4,
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
        step: step,
        totalSteps: totalSteps,
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
  late final VoiceFocusReturn _focusReturn;
  final _focusScope = FocusScopeNode(
    traversalEdgeBehavior: TraversalEdgeBehavior.closedLoop,
  );

  @override
  void initState() {
    super.initState();
    _focusReturn = VoiceFocusReturn.capture();
  }

  void _dismiss(VoidCallback action) {
    _focusReturn.restore();
    action();
  }

  @override
  void dispose() {
    _focusScope.dispose();
    super.dispose();
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
    final radius = context.voiceMetrics.corner('lg', fallback: 8);

    const bubbleWidth = 320.0;
    const bubbleMinHeight = 180.0;
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
    // Desktop hints sit beside their target; narrow layouts retain the safe
    // below/above placement so every action remains reachable.
    final beside = offset.dx + size.width + 18 + cardWidth <= safeRight;
    if (screen.width >= 600 && beside) {
      left = offset.dx + size.width + 18;
      top = (offset.dy - 20).clamp(safeTop, boundedMaxTop);
    }

    return Stack(
      children: [
        IgnorePointer(
          ignoring: true,
          child: ExcludeSemantics(
            child: CustomPaint(
              size: screen,
              painter: _CoachMarkScrim(
                target: Rect.fromLTWH(
                  offset.dx,
                  offset.dy,
                  size.width,
                  size.height,
                ).inflate(4),
                radius: radius,
                color: voice.overlay,
              ),
            ),
          ),
        ),
        Positioned(
          left: offset.dx - 4,
          top: offset.dy - 4,
          width: size.width + 8,
          height: size.height + 8,
          child: IgnorePointer(
            child: ExcludeSemantics(
              child: DecoratedBox(
                decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(radius),
                  border: Border.all(color: voice.profileAccent, width: 2),
                ),
              ),
            ),
          ),
        ),
        Positioned(
          left: left,
          top: top,
          width: cardWidth,
          child: FocusScope(
            node: _focusScope,
            autofocus: true,
            child: Semantics(
              container: true,
              liveRegion: true,
              child: Material(
                key: OnboardingCoachMark.cardKey,
                elevation: 8,
                borderRadius: BorderRadius.circular(radius),
                color: voice.elevated,
                child: ConstrainedBox(
                  constraints: BoxConstraints(
                    minHeight: cardMinHeight,
                    maxHeight: safeBottom - top,
                  ),
                  child: SingleChildScrollView(
                    child: Padding(
                      padding: const EdgeInsets.all(20),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text(
                            widget.title,
                            style: Theme.of(context).textTheme.titleSmall
                                ?.copyWith(
                                  fontSize: 16,
                                  fontWeight: FontWeight.w600,
                                  height: 22 / 16,
                                ),
                          ),
                          const SizedBox(height: 8),
                          Text(
                            widget.body,
                            style: Theme.of(context).textTheme.bodyMedium
                                ?.copyWith(
                                  fontSize: 13,
                                  height: 18 / 13,
                                  color: voice.textSecondary,
                                ),
                          ),
                          const SizedBox(height: 12),
                          Semantics(
                            label: '${widget.step} / ${widget.totalSteps}',
                            child: ExcludeSemantics(
                              child: Row(
                                children: List.generate(
                                  widget.totalSteps,
                                  (index) => Padding(
                                    padding: const EdgeInsets.only(right: 5),
                                    child: Container(
                                      width: 6,
                                      height: 6,
                                      decoration: BoxDecoration(
                                        shape: BoxShape.circle,
                                        color: index + 1 == widget.step
                                            ? voice.profileAccent
                                            : voice.borderStrong,
                                      ),
                                    ),
                                  ),
                                ),
                              ),
                            ),
                          ),
                          const SizedBox(height: 8),
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
                                  style: TextButton.styleFrom(
                                    backgroundColor: voice.profileAccent,
                                    foregroundColor: Theme.of(
                                      context,
                                    ).colorScheme.onPrimary,
                                  ),
                                  onPressed: () =>
                                      _dismiss(widget.onSecondary!),
                                  child: Text(widget.secondaryLabel!),
                                ),
                              FilledButton(
                                style: widget.onSecondary == null
                                    ? null
                                    : FilledButton.styleFrom(
                                        backgroundColor: Colors.transparent,
                                        foregroundColor: voice.textSecondary,
                                      ),
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
          ),
        ),
      ],
    );
  }
}

class _CoachMarkScrim extends CustomPainter {
  const _CoachMarkScrim({
    required this.target,
    required this.radius,
    required this.color,
  });
  final Rect target;
  final double radius;
  final Color color;

  @override
  void paint(Canvas canvas, Size size) {
    final path = Path()
      ..fillType = PathFillType.evenOdd
      ..addRect(Offset.zero & size)
      ..addRRect(RRect.fromRectAndRadius(target, Radius.circular(radius)));
    canvas.drawPath(path, Paint()..color = color);
  }

  @override
  bool shouldRepaint(_CoachMarkScrim oldDelegate) =>
      target != oldDelegate.target ||
      radius != oldDelegate.radius ||
      color != oldDelegate.color;
}
