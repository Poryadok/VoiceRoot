/// Canonical https://voice.gg share URLs (docs/features/deep-links.md).
library;

const voiceWebOrigin = 'https://voice.gg';

String voiceDeepLinkUrl(String path) {
  final normalized = path.startsWith('/') ? path : '/$path';
  return '$voiceWebOrigin$normalized';
}

String spaceShareUrl(String spaceId) => voiceDeepLinkUrl('/s/$spaceId');

String spaceChatShareUrl({required String spaceId, required String chatId}) =>
    voiceDeepLinkUrl('/s/$spaceId/c/$chatId');

String spaceMessageShareUrl({
  required String spaceId,
  required String chatId,
  required String messageId,
}) =>
    voiceDeepLinkUrl('/s/$spaceId/c/$chatId/m/$messageId');

String chatShareUrl(String chatId) => voiceDeepLinkUrl('/ch/$chatId');

String chatMessageShareUrl({
  required String chatId,
  required String messageId,
}) =>
    voiceDeepLinkUrl('/ch/$chatId/m/$messageId');

String profileShareUrl(String username) => voiceDeepLinkUrl('/u/$username');

String dmShareUrl(String userId) => voiceDeepLinkUrl('/dm/$userId');
