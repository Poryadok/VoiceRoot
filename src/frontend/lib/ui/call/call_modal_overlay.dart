import 'package:flutter/material.dart';

import '../../theme/voice_colors.dart';
import '../../theme/voice_metrics.dart';
import '../a11y/focus_trap.dart';
import '../a11y/voice_focus_return.dart';
import '../core/voice_avatar.dart';

/// Shared Penpot v2 overlay backdrop opacity (Dim fillOpacity 0.45).
const double kVoiceOverlayDimOpacity = 0.45;

/// Centered call card over dimmed backdrop (Penpot Overlay/Call/* · v2).
class CallModalOverlay extends StatefulWidget {
  const CallModalOverlay({
    super.key,
    required this.overlayKey,
    required this.title,
    required this.subtitle,
    required this.avatarLabel,
    this.avatarUrl,
    this.hint,
    required this.actions,
  });

  final Key overlayKey;
  final String title;
  final String subtitle;
  final String avatarLabel;
  final String? avatarUrl;
  final String? hint;
  final List<Widget> actions;

  @override
  State<CallModalOverlay> createState() => _CallModalOverlayState();
}

class _CallModalOverlayState extends State<CallModalOverlay> {
  late final VoiceFocusReturn _focusReturn = VoiceFocusReturn.capture();

  @override
  void dispose() {
    _focusReturn.restore();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final radius = context.voiceMetrics.corner('md', fallback: 6);
    return Positioned.fill(
      key: widget.overlayKey,
      child: VoiceFocusTrap(
        child: Material(
          color: voice.canvas.withValues(alpha: kVoiceOverlayDimOpacity),
          child: Center(
            child: Container(
              width: 360,
              constraints: const BoxConstraints(minHeight: 320),
              padding: const EdgeInsets.all(24),
              decoration: BoxDecoration(
                color: voice.elevated,
                borderRadius: BorderRadius.circular(radius),
                border: Border.all(color: voice.borderDefault),
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  VoiceAvatar(
                    imageUrl: widget.avatarUrl,
                    label: widget.avatarLabel,
                    radius: 36,
                  ),
                  const SizedBox(height: 16),
                  Text(
                    widget.title,
                    textAlign: TextAlign.center,
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                  const SizedBox(height: 4),
                  Text(
                    widget.subtitle,
                    textAlign: TextAlign.center,
                    style: TextStyle(color: voice.textSecondary),
                  ),
                  if (widget.hint != null && widget.hint!.isNotEmpty) ...[
                    const SizedBox(height: 8),
                    Text(
                      widget.hint!,
                      textAlign: TextAlign.center,
                      style: Theme.of(context).textTheme.bodySmall?.copyWith(
                            color: voice.textSecondary,
                          ),
                    ),
                  ],
                  const SizedBox(height: 20),
                  Wrap(
                    alignment: WrapAlignment.center,
                    spacing: 8,
                    runSpacing: 8,
                    children: widget.actions,
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
