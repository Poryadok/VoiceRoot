import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/routing/deep_link_parser.dart';

void main() {
  const spaceId = '550e8400-e29b-41d4-a716-446655440000';
  const chatId = '6ba7b810-9dad-11d1-80b4-00c04fd430c8';
  const voiceRoomId = '6ba7b811-9dad-11d1-80b4-00c04fd430c8';
  const messageId = '6ba7b812-9dad-11d1-80b4-00c04fd430c8';
  const userId = '6ba7b813-9dad-11d1-80b4-00c04fd430c8';
  const inviteCode = 'abc123XYZ';
  const username = 'vanya';

  group('parseDeepLinkUrl', () {
    final cases = <({String name, String raw, DeepLinkTarget? want})>[
      (
        name: 'voice invite',
        raw: 'voice://invite/$inviteCode',
        want: DeepLinkTarget(
          kind: DeepLinkKind.invite,
          inviteCode: inviteCode,
          rawUrl: 'voice://invite/$inviteCode',
        ),
      ),
      (
        name: 'https invite',
        raw: 'https://voice.gg/invite/$inviteCode',
        want: DeepLinkTarget(
          kind: DeepLinkKind.invite,
          inviteCode: inviteCode,
          rawUrl: 'https://voice.gg/invite/$inviteCode',
        ),
      ),
      (
        name: 'voice space',
        raw: 'voice://s/$spaceId',
        want: DeepLinkTarget(
          kind: DeepLinkKind.space,
          spaceId: spaceId,
          rawUrl: 'voice://s/$spaceId',
        ),
      ),
      (
        name: 'https space chat',
        raw: 'https://voice.gg/s/$spaceId/c/$chatId',
        want: DeepLinkTarget(
          kind: DeepLinkKind.spaceChat,
          spaceId: spaceId,
          chatId: chatId,
          rawUrl: 'https://voice.gg/s/$spaceId/c/$chatId',
        ),
      ),
      (
        name: 'voice voice room',
        raw: 'voice://s/$spaceId/v/$voiceRoomId',
        want: DeepLinkTarget(
          kind: DeepLinkKind.voiceRoom,
          spaceId: spaceId,
          voiceRoomId: voiceRoomId,
          rawUrl: 'voice://s/$spaceId/v/$voiceRoomId',
        ),
      ),
      (
        name: 'https space message',
        raw: 'https://voice.gg/s/$spaceId/c/$chatId/m/$messageId',
        want: DeepLinkTarget(
          kind: DeepLinkKind.spaceMessage,
          spaceId: spaceId,
          chatId: chatId,
          messageId: messageId,
          rawUrl: 'https://voice.gg/s/$spaceId/c/$chatId/m/$messageId',
        ),
      ),
      (
        name: 'voice chat',
        raw: 'voice://ch/$chatId',
        want: DeepLinkTarget(
          kind: DeepLinkKind.chat,
          chatId: chatId,
          rawUrl: 'voice://ch/$chatId',
        ),
      ),
      (
        name: 'https chat message',
        raw: 'https://voice.gg/ch/$chatId/m/$messageId',
        want: DeepLinkTarget(
          kind: DeepLinkKind.chatMessage,
          chatId: chatId,
          messageId: messageId,
          rawUrl: 'https://voice.gg/ch/$chatId/m/$messageId',
        ),
      ),
      (
        name: 'voice profile',
        raw: 'voice://u/$username',
        want: DeepLinkTarget(
          kind: DeepLinkKind.profile,
          username: username,
          rawUrl: 'voice://u/$username',
        ),
      ),
      (
        name: 'https dm',
        raw: 'https://voice.gg/dm/$userId',
        want: DeepLinkTarget(
          kind: DeepLinkKind.dm,
          userId: userId,
          rawUrl: 'https://voice.gg/dm/$userId',
        ),
      ),
    ];

    for (final c in cases) {
      test(c.name, () {
        final got = parseDeepLinkUrl(c.raw);
        expect(got, c.want);
      });
    }

    final invalid = [
      '',
      'not-a-url',
      'https://example.com/invite/foo',
      'voice://unknown/foo',
      'https://voice.gg/',
      'voice://invite/',
      'ftp://voice.gg/s/$spaceId',
    ];

    for (final raw in invalid) {
      test('invalid: $raw', () {
        expect(() => parseDeepLinkUrl(raw), throwsA(isA<DeepLinkParseException>()));
      });
    }
  });
}
