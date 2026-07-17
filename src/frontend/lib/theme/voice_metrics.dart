import 'package:flutter/material.dart';

import 'voice_token_catalog.dart';

/// Spacing, radius, layout and stroke from design tokens.
class VoiceMetrics extends ThemeExtension<VoiceMetrics> {
  const VoiceMetrics({
    required this.space,
    required this.radius,
    required this.layout,
    required this.stroke,
    required this.type,
  });

  final Map<String, double> space;
  final Map<String, double> radius;
  final Map<String, double> layout;
  final Map<String, double> stroke;
  final Map<String, VoiceTypeToken> type;

  double spacing(String key, {double fallback = 8}) =>
      space[key] ?? fallback;

  double corner(String key, {double fallback = 4}) =>
      radius[key] ?? fallback;

  double layoutSize(String key, {double fallback = 0}) =>
      layout[key] ?? fallback;

  double strokeWidth(String key, {double fallback = 1}) =>
      stroke[key] ?? fallback;

  VoiceTypeToken? typeStyle(String key) => type[key];

  EdgeInsets inset(String key, {double fallback = 8}) =>
      EdgeInsets.all(spacing(key, fallback: fallback));

  static VoiceMetrics fromCatalog(VoiceTokenCatalog catalog) {
    return VoiceMetrics(
      space: catalog.space,
      radius: catalog.radius,
      layout: catalog.layout,
      stroke: catalog.stroke,
      type: catalog.type,
    );
  }

  @override
  VoiceMetrics copyWith({
    Map<String, double>? space,
    Map<String, double>? radius,
    Map<String, double>? layout,
    Map<String, double>? stroke,
    Map<String, VoiceTypeToken>? type,
  }) {
    return VoiceMetrics(
      space: space ?? this.space,
      radius: radius ?? this.radius,
      layout: layout ?? this.layout,
      stroke: stroke ?? this.stroke,
      type: type ?? this.type,
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
      const VoiceMetrics(
        space: {'4': 4, '8': 8, '12': 12},
        radius: {'sm': 4},
        layout: {},
        stroke: {'hairline': 1},
        type: {},
      );
}
