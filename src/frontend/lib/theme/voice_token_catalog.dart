import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// Loaded from [kVoiceTokensAssetPath] (synced with design/tokens/voice.tokens.json).
class VoiceTokenCatalog {
  VoiceTokenCatalog._({
    required this.dsVersion,
    required this.profileAccentDefaults,
    required this.space,
    required this.radius,
    required this.themes,
  });

  static const kVoiceTokensAssetPath = 'assets/design/voice.tokens.json';

  final String dsVersion;
  final List<Color> profileAccentDefaults;
  final Map<String, double> space;
  final Map<String, double> radius;
  final Map<String, Map<String, Color>> themes;

  static VoiceTokenCatalog? _cached;

  static Future<VoiceTokenCatalog> load() async {
    if (_cached != null) return _cached!;
    final json = await rootBundle.loadString(kVoiceTokensAssetPath);
    _cached = _parse(json);
    return _cached!;
  }

  @visibleForTesting
  static VoiceTokenCatalog parseJson(String json) {
    return _parse(json);
  }

  static VoiceTokenCatalog _parse(String json) {
    final root = jsonDecode(json) as Map<String, dynamic>;
    final accentList =
        (root['profileAccent'] as Map<String, dynamic>)['defaults']
            as List<dynamic>;
    final spaceRaw = root['space'] as Map<String, dynamic>;
    final radiusRaw = root['radius'] as Map<String, dynamic>;
    final themesRaw = root['themes'] as Map<String, dynamic>;

    return VoiceTokenCatalog._(
      dsVersion: root['dsVersion'] as String? ?? '0.0.0',
      profileAccentDefaults: accentList
          .map((e) => _parseColor(e as String))
          .toList(growable: false),
      space: spaceRaw.map((k, v) => MapEntry(k, (v as num).toDouble())),
      radius: radiusRaw.map((k, v) => MapEntry(k, (v as num).toDouble())),
      themes: themesRaw.map((mode, colors) {
        final map = colors as Map<String, dynamic>;
        return MapEntry(
          mode,
          map.map((k, v) => MapEntry(k, _parseColor(v as String))),
        );
      }),
    );
  }

  Map<String, Color> colorsFor(String mode) {
    final m = themes[mode];
    if (m == null) {
      throw StateError('Unknown theme mode: $mode');
    }
    return m;
  }

  Color profileAccentAt(int index) {
    if (profileAccentDefaults.isEmpty) {
      return const Color(0xFF7EC8E3);
    }
    return profileAccentDefaults[index % profileAccentDefaults.length];
  }

  static Color colorFromHex(String hex) => _parseColor(hex);

  static Color _parseColor(String hex) {
    var value = hex.replaceFirst('#', '');
    if (value.length == 6) {
      value = 'FF$value';
    }
    return Color(int.parse(value, radix: 16));
  }
}
