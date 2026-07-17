import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// Typographic style from design tokens (`type.*` in voice.tokens.json).
class VoiceTypeToken {
  const VoiceTypeToken({
    required this.size,
    required this.weight,
    required this.lineHeight,
    this.letterSpacing,
  });

  final double size;
  final int weight;
  final double lineHeight;
  final double? letterSpacing;

  FontWeight get fontWeight {
    if (weight >= 700) return FontWeight.w700;
    if (weight >= 600) return FontWeight.w600;
    if (weight >= 500) return FontWeight.w500;
    if (weight >= 400) return FontWeight.w400;
    return FontWeight.w300;
  }

  TextStyle toTextStyle({Color? color}) {
    return TextStyle(
      fontSize: size,
      fontWeight: fontWeight,
      height: lineHeight / size,
      letterSpacing: letterSpacing,
      color: color,
    );
  }
}

/// Loaded from [kVoiceTokensAssetPath] (synced with design/tokens/voice.tokens.json).
class VoiceTokenCatalog {
  VoiceTokenCatalog._({
    required this.dsVersion,
    required this.profileAccentDefaults,
    required this.space,
    required this.radius,
    required this.layout,
    required this.type,
    required this.stroke,
    required this.themes,
  });

  static const kVoiceTokensAssetPath = 'assets/design/voice.tokens.json';

  final String dsVersion;
  final List<Color> profileAccentDefaults;
  final Map<String, double> space;
  final Map<String, double> radius;
  final Map<String, double> layout;
  final Map<String, VoiceTypeToken> type;
  final Map<String, double> stroke;
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
    final layoutRaw =
        root['layout'] as Map<String, dynamic>? ?? const <String, dynamic>{};
    final typeRaw =
        root['type'] as Map<String, dynamic>? ?? const <String, dynamic>{};
    final strokeRaw =
        root['stroke'] as Map<String, dynamic>? ?? const <String, dynamic>{};
    final themesRaw = root['themes'] as Map<String, dynamic>;

    return VoiceTokenCatalog._(
      dsVersion: root['dsVersion'] as String? ?? '0.0.0',
      profileAccentDefaults: accentList
          .map((e) => _parseColor(e as String))
          .toList(growable: false),
      space: spaceRaw.map((k, v) => MapEntry(k, (v as num).toDouble())),
      radius: radiusRaw.map((k, v) => MapEntry(k, (v as num).toDouble())),
      layout: layoutRaw.map((k, v) => MapEntry(k, (v as num).toDouble())),
      type: typeRaw.map((k, v) {
        final m = v as Map<String, dynamic>;
        return MapEntry(
          k,
          VoiceTypeToken(
            size: (m['size'] as num).toDouble(),
            weight: (m['weight'] as num).toInt(),
            lineHeight: (m['lineHeight'] as num).toDouble(),
            letterSpacing: (m['letterSpacing'] as num?)?.toDouble(),
          ),
        );
      }),
      stroke: strokeRaw.map((k, v) => MapEntry(k, (v as num).toDouble())),
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

  VoiceTypeToken? typeStyle(String key) => type[key];

  static Color colorFromHex(String hex) => _parseColor(hex);

  static Color _parseColor(String hex) {
    var value = hex.replaceFirst('#', '');
    if (value.length == 6) {
      value = 'FF$value';
    }
    return Color(int.parse(value, radix: 16));
  }
}
