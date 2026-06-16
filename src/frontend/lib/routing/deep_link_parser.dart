/// Deep link URL parser (mirrors gateway deeplinks_parse.go — docs/features/deep-links.md).
library;

enum DeepLinkKind {
  invite,
  space,
  spaceChat,
  voiceRoom,
  spaceMessage,
  chat,
  chatMessage,
  profile,
  dm,
}

class DeepLinkParseException implements Exception {
  DeepLinkParseException(this.message);
  final String message;
  @override
  String toString() => message;
}

class DeepLinkTarget {
  const DeepLinkTarget({
    required this.kind,
    required this.rawUrl,
    this.spaceId,
    this.chatId,
    this.voiceRoomId,
    this.messageId,
    this.inviteCode,
    this.username,
    this.userId,
  });

  final DeepLinkKind kind;
  final String rawUrl;
  final String? spaceId;
  final String? chatId;
  final String? voiceRoomId;
  final String? messageId;
  final String? inviteCode;
  final String? username;
  final String? userId;

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is DeepLinkTarget &&
          kind == other.kind &&
          rawUrl == other.rawUrl &&
          spaceId == other.spaceId &&
          chatId == other.chatId &&
          voiceRoomId == other.voiceRoomId &&
          messageId == other.messageId &&
          inviteCode == other.inviteCode &&
          username == other.username &&
          userId == other.userId;

  @override
  int get hashCode => Object.hash(
        kind,
        rawUrl,
        spaceId,
        chatId,
        voiceRoomId,
        messageId,
        inviteCode,
        username,
        userId,
      );
}

DeepLinkTarget parseDeepLinkUrl(String raw) {
  final trimmed = raw.trim();
  if (trimmed.isEmpty) {
    throw DeepLinkParseException('empty url');
  }

  Uri uri;
  try {
    uri = Uri.parse(trimmed);
  } catch (_) {
    throw DeepLinkParseException('invalid url');
  }

  String path;
  if (uri.scheme == 'voice') {
    var voicePath = trimmed;
    if (voicePath.startsWith('voice://')) {
      voicePath = voicePath.substring('voice://'.length);
    } else if (voicePath.startsWith('voice:')) {
      voicePath = voicePath.substring('voice:'.length);
    }
    voicePath = voicePath.replaceFirst(RegExp(r'^/+'), '');
    path = voicePath;
  } else if (uri.scheme == 'https' || uri.scheme == 'http') {
    final host = uri.host.toLowerCase();
    if (host != 'voice.gg' && host != 'www.voice.gg') {
      throw DeepLinkParseException('foreign host');
    }
    path = uri.path.replaceFirst(RegExp(r'^/+'), '').replaceAll(RegExp(r'/+$'), '');
  } else {
    throw DeepLinkParseException('unsupported scheme');
  }

  return _parseDeepLinkPath(path, trimmed);
}

DeepLinkTarget _parseDeepLinkPath(String path, String raw) {
  if (path.isEmpty) {
    throw DeepLinkParseException('empty path');
  }

  final parts = path.split('/');
  switch (parts[0]) {
    case 'invite':
      if (parts.length != 2 || parts[1].isEmpty) {
        throw DeepLinkParseException('invalid invite');
      }
      return DeepLinkTarget(
        kind: DeepLinkKind.invite,
        inviteCode: parts[1],
        rawUrl: raw,
      );
    case 's':
      return _parseSpacePath(parts, raw);
    case 'ch':
      return _parseChatPath(parts, raw);
    case 'u':
      if (parts.length != 2 || parts[1].isEmpty) {
        throw DeepLinkParseException('invalid profile');
      }
      return DeepLinkTarget(
        kind: DeepLinkKind.profile,
        username: parts[1],
        rawUrl: raw,
      );
    case 'dm':
      if (parts.length != 2 || parts[1].isEmpty) {
        throw DeepLinkParseException('invalid dm');
      }
      return DeepLinkTarget(
        kind: DeepLinkKind.dm,
        userId: parts[1],
        rawUrl: raw,
      );
    default:
      throw DeepLinkParseException('unknown path');
  }
}

DeepLinkTarget _parseSpacePath(List<String> parts, String raw) {
  if (parts.length < 2 || parts[1].isEmpty) {
    throw DeepLinkParseException('invalid space');
  }
  final spaceId = parts[1];
  if (parts.length == 2) {
    return DeepLinkTarget(kind: DeepLinkKind.space, spaceId: spaceId, rawUrl: raw);
  }
  if (parts.length >= 4 && parts[2] == 'c' && parts[3].isNotEmpty) {
    if (parts.length == 4) {
      return DeepLinkTarget(
        kind: DeepLinkKind.spaceChat,
        spaceId: spaceId,
        chatId: parts[3],
        rawUrl: raw,
      );
    }
    if (parts.length == 6 && parts[4] == 'm' && parts[5].isNotEmpty) {
      return DeepLinkTarget(
        kind: DeepLinkKind.spaceMessage,
        spaceId: spaceId,
        chatId: parts[3],
        messageId: parts[5],
        rawUrl: raw,
      );
    }
  }
  if (parts.length == 4 && parts[2] == 'v' && parts[3].isNotEmpty) {
    return DeepLinkTarget(
      kind: DeepLinkKind.voiceRoom,
      spaceId: spaceId,
      voiceRoomId: parts[3],
      rawUrl: raw,
    );
  }
  throw DeepLinkParseException('invalid space path');
}

DeepLinkTarget _parseChatPath(List<String> parts, String raw) {
  if (parts.length == 2 && parts[1].isNotEmpty) {
    return DeepLinkTarget(kind: DeepLinkKind.chat, chatId: parts[1], rawUrl: raw);
  }
  if (parts.length == 4 && parts[1].isNotEmpty && parts[2] == 'm' && parts[3].isNotEmpty) {
    return DeepLinkTarget(
      kind: DeepLinkKind.chatMessage,
      chatId: parts[1],
      messageId: parts[3],
      rawUrl: raw,
    );
  }
  throw DeepLinkParseException('invalid chat path');
}

/// Extracts invite code from pasted link or raw code.
String? extractInviteCode(String input) {
  final trimmed = input.trim();
  if (trimmed.isEmpty) return null;
  try {
    return parseDeepLinkUrl(trimmed).inviteCode;
  } catch (_) {
    // Plain code paste.
    if (trimmed.contains('/')) {
      final segments = trimmed.split('/').where((s) => s.isNotEmpty).toList();
      if (segments.isNotEmpty) return segments.last;
    }
    return trimmed;
  }
}
