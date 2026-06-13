import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/messages_client.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';
import 'markdown_inline.dart';
import 'markdown_message_content.dart';

/// Message body with @mention tokens highlighted (PLAN Phase 6).
class MentionMessageContent extends ConsumerWidget {
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
  Widget build(BuildContext context, WidgetRef ref) {
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
      final token = match.group(0)!;
      final label = _labelForToken(ref, token);
      spans.add(
        TextSpan(
          text: label,
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

  String _labelForToken(WidgetRef ref, String token) {
    if (token.toLowerCase() == '@everyone' || token.toLowerCase() == '@here') {
      return token;
    }
    final uuidMatch = RegExp(
      r'@([0-9a-fA-F-]{36})',
      caseSensitive: false,
    ).firstMatch(token);
    if (uuidMatch == null) return token;
    final profileId = uuidMatch.group(1)!;
    final profile = ref.watch(profileProvider(profileId)).valueOrNull;
    final handle = profile?.handle;
    if (handle != null && handle.isNotEmpty) {
      return '@$handle';
    }
    final name = profile?.displayName;
    if (name != null && name.isNotEmpty) {
      return '@$name';
    }
    return token;
  }
}
