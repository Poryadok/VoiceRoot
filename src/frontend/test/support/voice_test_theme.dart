import 'package:flutter/material.dart';
import 'package:voice_frontend/theme/voice_colors.dart';
import 'package:voice_frontend/theme/voice_metrics.dart';

/// Minimal [VoiceColors] for widget tests without loading token assets.
ThemeData voiceTestTheme() {
  const voice = VoiceColors(
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
    focusRing: Color(0xFF7EC8E3),
    profileAccent: Color(0xFF7EC8E3),
  );
  const metrics = VoiceMetrics(
    space: {'4': 4, '8': 8, '12': 12},
    radius: {'sm': 4, 'md': 6, 'lg': 8},
  );
  return ThemeData(
    useMaterial3: true,
    brightness: Brightness.dark,
    extensions: const [voice, metrics],
    colorScheme: ColorScheme.dark(
      primary: voice.profileAccent,
      surface: voice.surface,
    ),
  );
}
