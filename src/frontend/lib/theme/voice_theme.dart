import 'package:flutter/material.dart';

import 'voice_colors.dart';
import 'voice_token_catalog.dart';

enum VoiceThemeMode { light, dark, highContrast }

/// Builds [ThemeData] from design tokens + [profileAccent] for the active profile.
class VoiceTheme {
  VoiceTheme._();

  static const _modeKey = <VoiceThemeMode, String>{
    VoiceThemeMode.light: 'light',
    VoiceThemeMode.dark: 'dark',
    VoiceThemeMode.highContrast: 'highContrast',
  };

  static Future<ThemeData> build({
    required VoiceTokenCatalog catalog,
    required VoiceThemeMode mode,
    required Color profileAccent,
  }) async {
    final tokens = catalog.colorsFor(_modeKey[mode]!);
    final voiceColors = VoiceColors.fromTokenMap(
      tokens,
      profileAccent: profileAccent,
    );
    final scheme = _colorScheme(voiceColors, mode);
    final radiusSm = catalog.radius['sm'] ?? 4;

    return ThemeData(
      useMaterial3: true,
      brightness: mode == VoiceThemeMode.light
          ? Brightness.light
          : Brightness.dark,
      colorScheme: scheme,
      scaffoldBackgroundColor: voiceColors.canvas,
      extensions: [voiceColors],
      filledButtonTheme: FilledButtonThemeData(
        style: FilledButton.styleFrom(
          backgroundColor: profileAccent,
          foregroundColor: _onAccent(profileAccent),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(radiusSm),
          ),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          foregroundColor: voiceColors.textPrimary,
          side: BorderSide(color: voiceColors.borderDefault),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(radiusSm),
          ),
        ),
      ),
      textButtonTheme: TextButtonThemeData(
        style: TextButton.styleFrom(foregroundColor: voiceColors.textSecondary),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: voiceColors.surface,
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusSm),
          borderSide: BorderSide(color: voiceColors.borderDefault),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusSm),
          borderSide: BorderSide(color: voiceColors.borderDefault),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusSm),
          borderSide: BorderSide(color: voiceColors.focusRing, width: 2),
        ),
      ),
      dividerColor: voiceColors.borderDefault,
      appBarTheme: AppBarTheme(
        backgroundColor: voiceColors.surface,
        foregroundColor: voiceColors.textPrimary,
        elevation: 0,
      ),
    );
  }

  static ColorScheme _colorScheme(VoiceColors c, VoiceThemeMode mode) {
    final base = mode == VoiceThemeMode.light
        ? ColorScheme.light(
            surface: c.surface,
            onSurface: c.textPrimary,
            error: c.error,
          )
        : ColorScheme.dark(
            surface: c.surface,
            onSurface: c.textPrimary,
            error: c.error,
          );
    return base.copyWith(
      primary: c.profileAccent,
      onPrimary: _onAccent(c.profileAccent),
      primaryContainer: c.profileAccent.withValues(alpha: 0.25),
      onPrimaryContainer: c.textPrimary,
      surfaceContainerHighest: c.elevated,
    );
  }

  static Color _onAccent(Color accent) {
    final luminance = accent.computeLuminance();
    return luminance > 0.5 ? const Color(0xFF1A1A1A) : Colors.white;
  }
}
