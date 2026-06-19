import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/routing/deep_link_parser.dart';
import 'package:voice_frontend/state/deep_link_navigation.dart';

void main() {
  test('shareUrlForChat builds space message link', () {
    expect(
      shareUrlForChat(
        chatId: 'chat-1',
        spaceId: 'space-1',
        messageId: 'msg-1',
      ),
      'https://voice.gg/s/space-1/c/chat-1/m/msg-1',
    );
  });

  test('parseDeepLinkUrl profile and dm kinds', () {
    final profile = parseDeepLinkUrl('https://voice.gg/u/alice');
    expect(profile.kind, DeepLinkKind.profile);
    expect(profile.username, 'alice');

    final dm = parseDeepLinkUrl('https://voice.gg/dm/user-1');
    expect(dm.kind, DeepLinkKind.dm);
    expect(dm.userId, 'user-1');
  });
}
