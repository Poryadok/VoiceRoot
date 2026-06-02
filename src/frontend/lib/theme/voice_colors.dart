import 'package:flutter/material.dart';

/// Semantic colors for the active [VoiceThemeMode] plus per-profile accent.
@immutable
class VoiceColors extends ThemeExtension<VoiceColors> {
  const VoiceColors({
    required this.canvas,
    required this.surface,
    required this.elevated,
    required this.muted,
    required this.textPrimary,
    required this.textSecondary,
    required this.textDisabled,
    required this.borderDefault,
    required this.borderStrong,
    required this.error,
    required this.focusRing,
    required this.profileAccent,
  });

  final Color canvas;
  final Color surface;
  final Color elevated;
  final Color muted;
  final Color textPrimary;
  final Color textSecondary;
  final Color textDisabled;
  final Color borderDefault;
  final Color borderStrong;
  final Color error;
  final Color focusRing;
  final Color profileAccent;

  static VoiceColors of(BuildContext context) {
    final ext = Theme.of(context).extension<VoiceColors>();
    assert(ext != null, 'VoiceColors ThemeExtension missing');
    return ext!;
  }

  factory VoiceColors.fromTokenMap(
    Map<String, Color> tokens, {
    required Color profileAccent,
  }) {
    Color c(String key) => tokens[key]!;
    return VoiceColors(
      canvas: c('color.background.canvas'),
      surface: c('color.background.surface'),
      elevated: c('color.background.elevated'),
      muted: c('color.background.muted'),
      textPrimary: c('color.text.primary'),
      textSecondary: c('color.text.secondary'),
      textDisabled: c('color.text.disabled'),
      borderDefault: c('color.border.default'),
      borderStrong: c('color.border.strong'),
      error: c('color.semantic.error'),
      focusRing: c('color.focus.ring'),
      profileAccent: profileAccent,
    );
  }

  @override
  VoiceColors copyWith({
    Color? canvas,
    Color? surface,
    Color? elevated,
    Color? muted,
    Color? textPrimary,
    Color? textSecondary,
    Color? textDisabled,
    Color? borderDefault,
    Color? borderStrong,
    Color? error,
    Color? focusRing,
    Color? profileAccent,
  }) {
    return VoiceColors(
      canvas: canvas ?? this.canvas,
      surface: surface ?? this.surface,
      elevated: elevated ?? this.elevated,
      muted: muted ?? this.muted,
      textPrimary: textPrimary ?? this.textPrimary,
      textSecondary: textSecondary ?? this.textSecondary,
      textDisabled: textDisabled ?? this.textDisabled,
      borderDefault: borderDefault ?? this.borderDefault,
      borderStrong: borderStrong ?? this.borderStrong,
      error: error ?? this.error,
      focusRing: focusRing ?? this.focusRing,
      profileAccent: profileAccent ?? this.profileAccent,
    );
  }

  @override
  VoiceColors lerp(ThemeExtension<VoiceColors>? other, double t) {
    if (other is! VoiceColors) return this;
    Color l(Color a, Color b) => Color.lerp(a, b, t)!;
    return VoiceColors(
      canvas: l(canvas, other.canvas),
      surface: l(surface, other.surface),
      elevated: l(elevated, other.elevated),
      muted: l(muted, other.muted),
      textPrimary: l(textPrimary, other.textPrimary),
      textSecondary: l(textSecondary, other.textSecondary),
      textDisabled: l(textDisabled, other.textDisabled),
      borderDefault: l(borderDefault, other.borderDefault),
      borderStrong: l(borderStrong, other.borderStrong),
      error: l(error, other.error),
      focusRing: l(focusRing, other.focusRing),
      profileAccent: l(profileAccent, other.profileAccent),
    );
  }
}
