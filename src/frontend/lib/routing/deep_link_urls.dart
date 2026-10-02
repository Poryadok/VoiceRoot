/// Canonical https://voice.gg share URLs (docs/features/deep-links.md).
library;

import 'package:flutter/foundation.dart' show kIsWeb;

const voiceWebOrigin = 'https://voice.gg';
const voiceInviteShareOrigin = String.fromEnvironment(
  'VOICE_INVITE_SHARE_ORIGIN',
  defaultValue: voiceWebOrigin,
);

String voiceDeepLinkUrl(String path, {Uri? currentOrigin}) {
  final normalized = path.startsWith('/') ? path : '/$path';
  final origin = _shareOrigin(currentOrigin);
  return '$origin$normalized';
}

String spaceShareUrl(String spaceId, {Uri? currentOrigin}) =>
    voiceDeepLinkUrl('/s/$spaceId', currentOrigin: currentOrigin);

String spaceChatShareUrl({
  required String spaceId,
  required String chatId,
  Uri? currentOrigin,
}) => voiceDeepLinkUrl('/s/$spaceId/c/$chatId', currentOrigin: currentOrigin);

String spaceMessageShareUrl({
  required String spaceId,
  required String chatId,
  required String messageId,
  Uri? currentOrigin,
}) => voiceDeepLinkUrl(
  '/s/$spaceId/c/$chatId/m/$messageId',
  currentOrigin: currentOrigin,
);

String chatShareUrl(String chatId, {Uri? currentOrigin}) =>
    voiceDeepLinkUrl('/ch/$chatId', currentOrigin: currentOrigin);

String chatMessageShareUrl({
  required String chatId,
  required String messageId,
  Uri? currentOrigin,
}) =>
    voiceDeepLinkUrl('/ch/$chatId/m/$messageId', currentOrigin: currentOrigin);

String profileShareUrl(String username, {Uri? currentOrigin}) =>
    voiceDeepLinkUrl('/u/$username', currentOrigin: currentOrigin);

String dmShareUrl(String userId, {Uri? currentOrigin}) =>
    voiceDeepLinkUrl('/dm/$userId', currentOrigin: currentOrigin);

String spaceInviteShareUrl(String code, {Uri? currentOrigin}) =>
    voiceDeepLinkUrl(
      '/invite/$code',
      currentOrigin:
          currentOrigin ??
          (kIsWeb ? Uri.base : Uri.tryParse(voiceInviteShareOrigin)),
    );

String _shareOrigin(Uri? explicitOrigin) {
  final candidate = explicitOrigin ?? (kIsWeb ? Uri.base : null);
  if (candidate == null ||
      (candidate.scheme != 'https' && candidate.scheme != 'http') ||
      candidate.host.isEmpty) {
    return voiceWebOrigin;
  }
  return candidate.origin;
}
