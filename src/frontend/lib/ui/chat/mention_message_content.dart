import 'package:flutter/material.dart';

import '../../backend/messages_client.dart';
import '../../theme/voice_colors.dart';
import 'markdown_inline.dart';
import 'markdown_message_content.dart';

/// Message body with @mention tokens highlighted (PLAN Phase 6).
class MentionMessageContent extends StatelessWidget {
  const MentionMessageContent({
    super.key,
    required this.content,
    this.mentions = const [],
  });

  final String content;
  final List<MessageMention> mentions;

  static final _tokenPattern = RegExp(
    r'@everyone|@here|@[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}',
    caseSensitive: false,
  );

  @override
  Widget build(BuildContext context) {
    if (mentions.isEmpty || !_tokenPattern.hasMatch(content)) {
      return MarkdownMessageContent(content: content);
    }
    final voice = VoiceColors.of(context);
    final accent = voice.profileAccent;
    final baseStyle = Theme.of(context).textTheme.bodyMedium;
    final spans = <InlineSpan>[];
    var index = 0;
    for (final match in _tokenPattern.allMatches(content)) {
      if (match.start > index) {
        spans.addAll(
          buildChatMarkdownInlineSpans(
            context,
            content.substring(index, match.start),
            baseStyle,
          ),
        );
      }
      spans.add(
        TextSpan(
          text: match.group(0),
          style: baseStyle?.copyWith(
            color: accent,
            fontWeight: FontWeight.w600,
          ),
        ),
      );
      index = match.end;
    }
    if (index < content.length) {
      spans.addAll(
        buildChatMarkdownInlineSpans(
          context,
          content.substring(index),
          baseStyle,
        ),
      );
    }
    return RichText(text: TextSpan(style: baseStyle, children: spans));
  }
}
