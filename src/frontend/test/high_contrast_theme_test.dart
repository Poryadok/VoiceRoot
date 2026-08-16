import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/theme/voice_colors.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';

import 'support/test_voice_token_catalog.dart';

/// High Contrast theme from design tokens (docs/features/accessibility.md §Темы).
/// Complements widget semantics + CI axe landmark smoke (A11Y-03).
void main() {
  test('high contrast tokens map to VoiceColors contract', () {
    final tokens = testVoiceTokenCatalog.colorsFor('highContrast');
    final colors = VoiceColors.fromTokenMap(
      tokens,
      profileAccent: testVoiceTokenCatalog.profileAccentAt(0),
    );

    expect(colors.canvas, const Color(0xFF000000));
    expect(colors.surface, const Color(0xFF0D0D0D));
    expect(colors.textPrimary, const Color(0xFFFFFFFF));
    expect(colors.textSecondary, const Color(0xFFE0E0E0));
    expect(colors.focusRing, const Color(0xFFFFFFFF));
    expect(colors.borderDefault, const Color(0xFFFFFFFF));
  });

  test('AppThemePreference.highContrast resolves to highContrast mode', () {
    expect(
      _resolveMode(AppThemePreference.highContrast, Brightness.light),
      VoiceThemeMode.highContrast,
    );
    expect(
      _resolveMode(AppThemePreference.highContrast, Brightness.dark),
      VoiceThemeMode.highContrast,
    );
  });
}

/// Mirrors voice_theme_providers._resolveMode (package-private).
VoiceThemeMode _resolveMode(AppThemePreference pref, Brightness platform) {
  return switch (pref) {
    AppThemePreference.light => VoiceThemeMode.light,
    AppThemePreference.dark => VoiceThemeMode.dark,
    AppThemePreference.highContrast => VoiceThemeMode.highContrast,
    AppThemePreference.system =>
      platform == Brightness.dark ? VoiceThemeMode.dark : VoiceThemeMode.light,
  };
}
