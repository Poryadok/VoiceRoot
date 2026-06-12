import '../messages_client.dart';
import 'message_cache_limits.dart';

List<VoiceMessage> sortMessagesForCache(Iterable<VoiceMessage> messages) {
  final sorted = [...messages];
  sorted.sort((a, b) {
    final at = a.createdAt ?? DateTime.fromMillisecondsSinceEpoch(0);
    final bt = b.createdAt ?? DateTime.fromMillisecondsSinceEpoch(0);
    final byTime = at.compareTo(bt);
    if (byTime != 0) return byTime;
    return a.id.compareTo(b.id);
  });
  return sorted;
}

List<VoiceMessage> trimMessagesToCacheLimit(Iterable<VoiceMessage> messages) {
  final sorted = sortMessagesForCache(messages);
  if (sorted.length <= kOfflineCacheMessageLimit) {
    return sorted;
  }
  return sorted.sublist(sorted.length - kOfflineCacheMessageLimit);
}
