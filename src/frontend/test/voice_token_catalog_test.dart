import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';

void main() {
  test('parseJson loads profile accent defaults and themes', () {
    const json = '''
{
  "dsVersion": "0.1.0",
  "profileAccent": { "defaults": ["#7EC8E3", "#9ED9A6"] },
  "space": { "8": 8 },
  "radius": { "sm": 4 },
  "themes": {
    "light": {
      "color.background.canvas": "#FFFFFF",
      "color.text.primary": "#1A1A1A"
    }
  }
}
''';
    final catalog = VoiceTokenCatalog.parseJson(json);
    expect(catalog.dsVersion, '0.1.0');
    expect(catalog.profileAccentAt(0), const Color(0xFF7EC8E3));
    expect(catalog.profileAccentAt(3), const Color(0xFF9ED9A6));
    expect(catalog.colorsFor('light')['color.background.canvas'],
        const Color(0xFFFFFFFF));
  });

  test('colorFromHex parses six-digit hex', () {
    expect(
      VoiceTokenCatalog.colorFromHex('#F0A8A8'),
      const Color(0xFFF0A8A8),
    );
  });
}
