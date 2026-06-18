import 'dart:convert';

import 'package:flutter/material.dart';

import '../../theme/voice_colors.dart';

/// Resolves text-story background from `text_style_json` (`background`: accent|elevated|muted).
Color storyTextBackgroundColor(VoiceColors voice, String? textStyleJson) {
  if (textStyleJson == null || textStyleJson.trim().isEmpty) {
    return voice.profileAccent;
  }
  try {
    final map = jsonDecode(textStyleJson) as Map<String, dynamic>;
    final bg = map['background'] as String? ?? 'accent';
    return switch (bg) {
      'elevated' => voice.elevated,
      'muted' => voice.muted,
      _ => voice.profileAccent,
    };
  } catch (_) {
    return voice.profileAccent;
  }
}
