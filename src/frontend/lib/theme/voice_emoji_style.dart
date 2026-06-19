import 'package:flutter/material.dart';

/// Bundled [Noto Color Emoji] for reactions and emoji picker (Batch 9 / web).
abstract final class VoiceEmojiStyle {
  static const String fontFamily = 'Noto Color Emoji';

  /// Text style for emoji glyphs; avoids runtime Noto CDN fallback on web.
  static TextStyle textStyle({double fontSize = 14}) {
    return TextStyle(
      fontFamily: fontFamily,
      fontSize: fontSize,
    );
  }
}
