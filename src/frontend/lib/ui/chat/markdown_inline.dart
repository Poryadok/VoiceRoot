import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../theme/voice_colors.dart';

/// Inline markdown spans (bold/italic/links) for embedding in mention-rich messages.
List<InlineSpan> buildChatMarkdownInlineSpans(
  BuildContext context,
  String text,
  TextStyle? base,
) {
  final spans = <InlineSpan>[];
  var i = 0;
  while (i < text.length) {
    final slice = text.substring(i);

    final link = RegExp(
      r'^\[([^\]]+)\]\(((?:[^()]|\([^()]*\))*)\)',
    ).firstMatch(slice);
    if (link != null) {
      spans.add(_linkSpan(context, link.group(1)!, link.group(2)!, base));
      i += link.end;
      continue;
    }

    final bold = RegExp(r'^\*\*([^*]+)\*\*').firstMatch(slice);
    if (bold != null) {
      spans.add(
        TextSpan(
          text: bold.group(1),
          style: base?.copyWith(fontWeight: FontWeight.bold),
        ),
      );
      i += bold.end;
      continue;
    }

    final italic = RegExp(r'^\*([^*]+)\*').firstMatch(slice);
    if (italic != null) {
      spans.add(
        TextSpan(
          text: italic.group(1),
          style: base?.copyWith(fontStyle: FontStyle.italic),
        ),
      );
      i += italic.end;
      continue;
    }

    final autoUrl = RegExp(r'^(https?://[^\s]+)').firstMatch(slice);
    if (autoUrl != null) {
      final url = autoUrl.group(1)!;
      spans.add(_linkSpan(context, url, url, base));
      i += autoUrl.end;
      continue;
    }

    final nextSpecial = RegExp(r'[\[\*]').firstMatch(slice)?.start ?? -1;
    if (nextSpecial < 0) {
      spans.add(TextSpan(text: slice, style: base));
      break;
    }
    if (nextSpecial > 0) {
      spans.add(TextSpan(text: slice.substring(0, nextSpecial), style: base));
    }
    i += nextSpecial == 0 ? 1 : nextSpecial;
  }
  return spans;
}

InlineSpan _linkSpan(
  BuildContext context,
  String label,
  String url,
  TextStyle? base,
) {
  final uri = Uri.tryParse(url);
  final safe = uri != null && (uri.scheme == 'http' || uri.scheme == 'https');
  final voice = VoiceColors.of(context);
  final style = base?.copyWith(
    color: voice.profileAccent,
    decoration: TextDecoration.underline,
  );
  if (!safe) {
    return TextSpan(text: label, style: style);
  }
  return TextSpan(
    text: label,
    style: style,
    recognizer: TapGestureRecognizer()
      ..onTap = () => launchUrl(uri, mode: LaunchMode.externalApplication),
  );
}
