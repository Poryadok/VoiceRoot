import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';

/// Parsed catalog for tests (no asset bundle required).
final VoiceTokenCatalog testVoiceTokenCatalog = VoiceTokenCatalog.parseJson(
  _kVoiceTokensJson,
);

List<Override> voiceThemeTestOverrides() => [
  voiceTokenCatalogProvider.overrideWith((ref) async => testVoiceTokenCatalog),
  voiceMaterialThemeProvider.overrideWith(
    (ref) => VoiceTheme.build(
      catalog: testVoiceTokenCatalog,
      mode: VoiceThemeMode.dark,
      profileAccent: testVoiceTokenCatalog.profileAccentAt(0),
    ),
  ),
];

// Mirrors design/tokens/voice.tokens.json
const _kVoiceTokensJson = '''
{
  "dsVersion": "0.1.0",
  "profileAccent": {
    "defaults": [
      "#7EC8E3",
      "#9ED9A6",
      "#F0A8A8",
      "#F5E6A3",
      "#FFCC99",
      "#C9B8FF",
      "#FFB3E6"
    ]
  },
  "space": { "4": 4, "8": 8, "12": 12, "16": 16, "24": 24, "32": 32 },
  "radius": { "sm": 4, "md": 6, "lg": 8 },
  "themes": {
    "light": {
      "color.background.canvas": "#FFFFFF",
      "color.background.surface": "#F5F5F5",
      "color.background.elevated": "#EEEEEE",
      "color.background.muted": "#E8E8E8",
      "color.text.primary": "#1A1A1A",
      "color.text.secondary": "#5C5C5C",
      "color.text.disabled": "#9E9E9E",
      "color.border.default": "#E0E0E0",
      "color.border.strong": "#1A1A1A",
      "color.semantic.error": "#C62828",
      "color.focus.ring": "#7EC8E3"
    },
    "dark": {
      "color.background.canvas": "#1E1E1E",
      "color.background.surface": "#2B2B2B",
      "color.background.elevated": "#333333",
      "color.background.muted": "#252525",
      "color.text.primary": "#F0F0F0",
      "color.text.secondary": "#A8A8A8",
      "color.text.disabled": "#6E6E6E",
      "color.border.default": "#3D3D3D",
      "color.border.strong": "#F0F0F0",
      "color.semantic.error": "#EF9A9A",
      "color.focus.ring": "#7EC8E3"
    },
    "highContrast": {
      "color.background.canvas": "#000000",
      "color.background.surface": "#0D0D0D",
      "color.background.elevated": "#1A1A1A",
      "color.background.muted": "#141414",
      "color.text.primary": "#FFFFFF",
      "color.text.secondary": "#E0E0E0",
      "color.text.disabled": "#9E9E9E",
      "color.border.default": "#FFFFFF",
      "color.border.strong": "#FFFFFF",
      "color.semantic.error": "#FF6B6B",
      "color.focus.ring": "#FFFFFF"
    }
  }
}
''';
