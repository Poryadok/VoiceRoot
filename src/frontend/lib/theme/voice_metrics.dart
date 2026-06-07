import 'package:flutter/material.dart';

import 'voice_token_catalog.dart';

/// Spacing and radius from design tokens, exposed via [ThemeData.extensions].
class VoiceMetrics extends ThemeExtension<VoiceMetrics> {
  const VoiceMetrics({required this.space, required this.radius});

  final Map<String, double> space;
  final Map<String, double> radius;

  double spacing(String key, {double fallback = 8}) =>
      space[key] ?? fallback;

  double corner(String key, {double fallback = 4}) =>
      radius[key] ?? fallback;

  EdgeInsets inset(String key, {double fallback = 8}) =>
      EdgeInsets.all(spacing(key, fallback: fallback));

  static VoiceMetrics fromCatalog(VoiceTokenCatalog catalog) {
    return VoiceMetrics(space: catalog.space, radius: catalog.radius);
  }

  @override
  VoiceMetrics copyWith({
    Map<String, double>? space,
    Map<String, double>? radius,
  }) {
    return VoiceMetrics(
      space: space ?? this.space,
      radius: radius ?? this.radius,
    );
  }

  @override
  VoiceMetrics lerp(ThemeExtension<VoiceMetrics>? other, double t) {
    if (other is! VoiceMetrics) return this;
    return this;
  }
}

extension VoiceMetricsContext on BuildContext {
  VoiceMetrics get voiceMetrics =>
      Theme.of(this).extension<VoiceMetrics>() ??
      const VoiceMetrics(space: {'4': 4, '8': 8, '12': 12}, radius: {'sm': 4});
}
