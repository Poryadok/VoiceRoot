import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/theme/voice_theme.dart';

import 'support/test_voice_token_catalog.dart';

void main() {
  test('VoiceTheme uses bundled Noto Sans (no google_fonts CDN)', () async {
    final theme = await VoiceTheme.build(
      catalog: testVoiceTokenCatalog,
      mode: VoiceThemeMode.dark,
      profileAccent: testVoiceTokenCatalog.profileAccentAt(0),
    );

    expect(theme.textTheme.bodyMedium?.fontFamily, VoiceTheme.fontFamily);
    expect(theme.textTheme.titleLarge?.fontFamily, VoiceTheme.fontFamily);
    expect(theme.textTheme.titleLarge?.fontWeight, FontWeight.w600);
  });
}
