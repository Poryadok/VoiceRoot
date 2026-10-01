import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/routing/deep_link_urls.dart';

void main() {
  test('share urls use the active web origin when supplied', () {
    final stagingOrigin = Uri.parse('https://app.comrade.click/');
    expect(
      spaceShareUrl('space-1', currentOrigin: stagingOrigin),
      'https://app.comrade.click/s/space-1',
    );
    expect(
      chatMessageShareUrl(
        chatId: 'chat-1',
        messageId: 'msg-1',
        currentOrigin: stagingOrigin,
      ),
      'https://app.comrade.click/ch/chat-1/m/msg-1',
    );
    expect(
      spaceInviteShareUrl('invite-1', currentOrigin: stagingOrigin),
      'https://app.comrade.click/invite/invite-1',
    );
  });

  test('space and chat share urls', () {
    expect(spaceShareUrl('space-1'), 'https://voice.gg/s/space-1');
    expect(
      spaceMessageShareUrl(
        spaceId: 'space-1',
        chatId: 'chat-1',
        messageId: 'msg-1',
      ),
      'https://voice.gg/s/space-1/c/chat-1/m/msg-1',
    );
    expect(
      chatMessageShareUrl(chatId: 'chat-1', messageId: 'msg-1'),
      'https://voice.gg/ch/chat-1/m/msg-1',
    );
    expect(profileShareUrl('vanya'), 'https://voice.gg/u/vanya');
  });
}
