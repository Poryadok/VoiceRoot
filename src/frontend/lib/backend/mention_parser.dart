import 'messages_client.dart';

final _userMentionUuid = RegExp(
  r'@([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})',
);

/// Builds server-side [mentions_json] from compose text (PLAN Phase 6).
List<MessageMention> parseMentionsFromContent(
  String content, {
  Iterable<String> memberProfileIds = const [],
  Map<String, String> handleToProfileId = const {},
}) {
  final members = memberProfileIds.toSet();
  final out = <MessageMention>[];
  if (RegExp(r'@everyone\b', caseSensitive: false).hasMatch(content)) {
    out.add(const MessageMention(type: 'everyone'));
  }
  if (RegExp(r'@here\b', caseSensitive: false).hasMatch(content)) {
    out.add(const MessageMention(type: 'here'));
  }
  for (final entry in handleToProfileId.entries) {
    final handle = entry.key.trim();
    if (handle.isEmpty) continue;
    final pattern = RegExp('@${RegExp.escape(handle)}\\b', caseSensitive: false);
    if (pattern.hasMatch(content)) {
      out.add(MessageMention(type: 'user', targetId: entry.value));
    }
  }
  for (final match in _userMentionUuid.allMatches(content)) {
    final id = match.group(1)!;
    if (members.isEmpty || members.contains(id)) {
      out.add(MessageMention(type: 'user', targetId: id));
    }
  }
  return out;
}
