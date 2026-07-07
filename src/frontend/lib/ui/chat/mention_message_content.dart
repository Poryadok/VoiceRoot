import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/messages_client.dart';
import '../../state/social_providers.dart';
import '../social/profile_detail_sheet.dart';
import 'markdown_inline.dart';
import 'markdown_message_content.dart';

/// Message body with @mention tokens highlighted (text-chat.md).
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
    final linkColor = Theme.of(context).colorScheme.primary;
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
      final mention = _mentionForToken(token);
      final label = _labelForToken(ref, token);
      if (mention?.type == 'everyone' || mention?.type == 'here') {
        spans.add(
          WidgetSpan(
            child: KeyedSubtree(
              key: Key('mention_broadcast_${mention!.type}'),
              child: const SizedBox.shrink(),
            ),
          ),
        );
        spans.add(
          TextSpan(
            text: label,
            style: baseStyle?.copyWith(
              color: linkColor,
              fontWeight: FontWeight.w600,
            ),
          ),
        );
      } else {
        final profileId = _profileIdFromToken(token);
        if (profileId != null) {
          spans.add(
            WidgetSpan(
              child: KeyedSubtree(
                key: Key('mention_user_link_$profileId'),
                child: const SizedBox.shrink(),
              ),
            ),
          );
        }
        final recognizer = profileId == null
            ? null
            : (TapGestureRecognizer()
                ..onTap = () => _openProfile(context, ref, profileId));
        spans.add(
          TextSpan(
            text: label,
            style: baseStyle?.copyWith(
              color: linkColor,
              decoration: TextDecoration.underline,
              fontWeight: FontWeight.w600,
            ),
            recognizer: recognizer,
          ),
        );
      }
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

  MessageMention? _mentionForToken(String token) {
    final lower = token.toLowerCase();
    for (final mention in mentions) {
      if (mention.type == 'everyone' && lower == '@everyone') return mention;
      if (mention.type == 'here' && lower == '@here') return mention;
      final profileId = mention.targetId;
      if (profileId != null && token.toLowerCase() == '@$profileId'.toLowerCase()) {
        return mention;
      }
    }
    return null;
  }

  String? _profileIdFromToken(String token) {
    final uuidMatch = RegExp(
      r'@([0-9a-fA-F-]{36})',
      caseSensitive: false,
    ).firstMatch(token);
    return uuidMatch?.group(1);
  }

  void _openProfile(BuildContext context, WidgetRef ref, String profileId) {
    final container = ProviderScope.containerOf(context);
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => UncontrolledProviderScope(
        container: container,
        child: ProfileDetailSheet(profileId: profileId),
      ),
    );
  }

  String _labelForToken(WidgetRef ref, String token) {
    if (token.toLowerCase() == '@everyone' || token.toLowerCase() == '@here') {
      return token;
    }
    final profileId = _profileIdFromToken(token);
    if (profileId == null) return token;
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
