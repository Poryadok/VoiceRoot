import 'messages_client.dart';

final _userMentionUuid = RegExp(
  r'@([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})',
);

/// Builds server-side [mentions_json] from compose text (PLAN Phase 6).
List<MessageMention> parseMentionsFromContent(
  String content, {
  Iterable<String> memberProfileIds = const [],
}) {
  final members = memberProfileIds.toSet();
  final out = <MessageMention>[];
  if (RegExp(r'@everyone\b', caseSensitive: false).hasMatch(content)) {
    out.add(const MessageMention(type: 'everyone'));
  }
  if (RegExp(r'@here\b', caseSensitive: false).hasMatch(content)) {
    out.add(const MessageMention(type: 'here'));
  }
  for (final match in _userMentionUuid.allMatches(content)) {
    final id = match.group(1)!;
    if (members.isEmpty || members.contains(id)) {
      out.add(MessageMention(type: 'user', targetId: id));
    }
  }
  return out;
}
