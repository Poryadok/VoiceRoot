import 'package:flutter/material.dart';

import '../../theme/voice_colors.dart';
import '../../theme/voice_metrics.dart';

/// Placeholder rows for list loading states.
class VoiceListSkeleton extends StatelessWidget {
  const VoiceListSkeleton({super.key, this.rowCount = 6});

  final int rowCount;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final pad = context.voiceMetrics.spacing('12', fallback: 12);
    return ListView.builder(
      physics: const NeverScrollableScrollPhysics(),
      itemCount: rowCount,
      itemBuilder: (context, index) {
        return Padding(
          padding: EdgeInsets.symmetric(horizontal: pad, vertical: pad / 2),
          child: Row(
            children: [
              _Bone(width: 40, height: 40, radius: 20, color: voice.muted),
              SizedBox(width: pad),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    _Bone(width: double.infinity, height: 12, color: voice.muted),
                    SizedBox(height: pad / 2),
                    _Bone(width: 120, height: 10, color: voice.elevated),
                  ],
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

class _Bone extends StatelessWidget {
  const _Bone({
    required this.width,
    required this.height,
    required this.color,
    this.radius = 4,
  });

  final double width;
  final double height;
  final Color color;
  final double radius;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: width,
      height: height,
      decoration: BoxDecoration(
        color: color,
        borderRadius: BorderRadius.circular(radius),
      ),
    );
  }
}
