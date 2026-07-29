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
    required this.success,
    required this.warning,
    required this.info,
    required this.badge,
    required this.presenceOnline,
    required this.presenceIdle,
    required this.presenceDnd,
    required this.presenceOffline,
    required this.overlay,
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
  final Color success;
  final Color warning;
  final Color info;
  final Color badge;
  final Color presenceOnline;
  final Color presenceIdle;
  final Color presenceDnd;
  final Color presenceOffline;
  final Color overlay;
  final Color focusRing;
  final Color profileAccent;

  static VoiceColors of(BuildContext context) {
    final ext = Theme.of(context).extension<VoiceColors>();
    return ext ?? _fallback;
  }

  static const _fallback = VoiceColors(
    canvas: Color(0xFF1E1E1E),
    surface: Color(0xFF2B2B2B),
    elevated: Color(0xFF333333),
    muted: Color(0xFF252525),
    textPrimary: Color(0xFFF0F0F0),
    textSecondary: Color(0xFFA8A8A8),
    textDisabled: Color(0xFF6E6E6E),
    borderDefault: Color(0xFF3D3D3D),
    borderStrong: Color(0xFFF0F0F0),
    error: Color(0xFFEF9A9A),
    success: Color(0xFF81C784),
    warning: Color(0xFFFFD54F),
    info: Color(0xFF64B5F6),
    badge: Color(0xFFEF5350),
    presenceOnline: Color(0xFF66BB6A),
    presenceIdle: Color(0xFFFFCA28),
    presenceDnd: Color(0xFFEF5350),
    presenceOffline: Color(0xFF6E6E6E),
    overlay: Color(0x8C000000),
    focusRing: Color(0xFF7EC8E3),
    profileAccent: Color(0xFF7EC8E3),
  );

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
      success: c('color.semantic.success'),
      warning: c('color.semantic.warning'),
      info: c('color.semantic.info'),
      badge: c('color.semantic.badge'),
      presenceOnline: c('color.presence.online'),
      presenceIdle: c('color.presence.idle'),
      presenceDnd: c('color.presence.dnd'),
      presenceOffline: c('color.presence.offline'),
      overlay: c('color.background.overlay'),
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
    Color? success,
    Color? warning,
    Color? info,
    Color? badge,
    Color? presenceOnline,
    Color? presenceIdle,
    Color? presenceDnd,
    Color? presenceOffline,
    Color? overlay,
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
      success: success ?? this.success,
      warning: warning ?? this.warning,
      info: info ?? this.info,
      badge: badge ?? this.badge,
      presenceOnline: presenceOnline ?? this.presenceOnline,
      presenceIdle: presenceIdle ?? this.presenceIdle,
      presenceDnd: presenceDnd ?? this.presenceDnd,
      presenceOffline: presenceOffline ?? this.presenceOffline,
      overlay: overlay ?? this.overlay,
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
      success: l(success, other.success),
      warning: l(warning, other.warning),
      info: l(info, other.info),
      badge: l(badge, other.badge),
      presenceOnline: l(presenceOnline, other.presenceOnline),
      presenceIdle: l(presenceIdle, other.presenceIdle),
      presenceDnd: l(presenceDnd, other.presenceDnd),
      presenceOffline: l(presenceOffline, other.presenceOffline),
      overlay: l(overlay, other.overlay),
      focusRing: l(focusRing, other.focusRing),
      profileAccent: l(profileAccent, other.profileAccent),
    );
  }
}
