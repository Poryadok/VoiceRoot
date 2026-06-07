import 'package:flutter/material.dart';

import '../../theme/voice_colors.dart';
import '../../theme/voice_metrics.dart';
import '../core/voice_avatar.dart';

/// Centered call card over dimmed backdrop.
class CallModalOverlay extends StatelessWidget {
  const CallModalOverlay({
    super.key,
    required this.overlayKey,
    required this.title,
    required this.subtitle,
    required this.avatarLabel,
    this.avatarUrl,
    required this.actions,
  });

  final Key overlayKey;
  final String title;
  final String subtitle;
  final String avatarLabel;
  final String? avatarUrl;
  final List<Widget> actions;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final radius = context.voiceMetrics.corner('lg', fallback: 8);
    return Positioned.fill(
      key: overlayKey,
      child: Material(
        color: voice.canvas.withValues(alpha: 0.88),
        child: Center(
          child: Container(
            width: 360,
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
                  imageUrl: avatarUrl,
                  label: avatarLabel,
                  radius: 36,
                ),
                const SizedBox(height: 16),
                Text(
                  title,
                  textAlign: TextAlign.center,
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 4),
                Text(
                  subtitle,
                  textAlign: TextAlign.center,
                  style: TextStyle(color: voice.textSecondary),
                ),
                const SizedBox(height: 20),
                Wrap(
                  alignment: WrapAlignment.center,
                  spacing: 8,
                  runSpacing: 8,
                  children: actions,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
