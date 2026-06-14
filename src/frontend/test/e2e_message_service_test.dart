import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/e2e/e2e_crypto_adapter.dart';
import 'package:voice_frontend/e2e/e2e_message_service.dart';
import 'package:voice_frontend/e2e/e2e_prekey_sync.dart';

void main() {
  group('E2eMessageService', () {
    late E2eMessageService service;

    setUp(() {
      service = E2eMessageService(adapter: E2eCryptoAdapter.inMemoryForTest());
    });

    test('encryptOutgoing and decryptForDisplay roundtrip', () async {
      const plaintext = 'service-layer-secret';
      final wire = await service.encryptOutgoing(
        localProfileId: 'me',
        peerProfileId: 'peer',
        plaintext: plaintext,
      );
      final message = VoiceMessage(
        id: 'm1',
        chatId: 'c1',
        senderProfileId: 'me',
        content: wire,
        isE2e: true,
      );
      final shown = await service.decryptForDisplay(
        message: message,
        localProfileId: 'peer',
        peerProfileId: 'me',
      );
      expect(shown.content, plaintext);
      expect(shown.decryptionFailed, isFalse);
    });
  });

  group('E2ePreKeySync', () {
    test('bundleForProfile returns opaque wire blob', () async {
      final adapter = E2eCryptoAdapter.inMemoryForTest();
      final sync = E2ePreKeySync(sessionManager: adapter.sessionManager);
      final wire = await sync.bundleForProfile('profile-1');
      expect(wire, isNotEmpty);
      expect(sync.bundleFromWire(wire), isNotNull);
    });
  });

  group('localE2eMessageSearch', () {
    const messages = [
      VoiceMessage(
        id: 'm1',
        chatId: 'c1',
        senderProfileId: 'a',
        content: 'hello secret world',
        isE2e: true,
      ),
      VoiceMessage(
        id: 'm2',
        chatId: 'c1',
        senderProfileId: 'b',
        content: 'other text',
        isE2e: true,
      ),
      VoiceMessage(
        id: 'm3',
        chatId: 'c1',
        senderProfileId: 'a',
        content: 'blocked',
        isE2e: true,
        decryptionFailed: true,
      ),
    ];

    test('finds matches in decrypted local cache only', () {
      final hits = localE2eMessageSearch(messages: messages, query: 'secret');
      expect(hits.map((m) => m.id).toList(), ['m1']);
    });

    test('skips undecryptable messages', () {
      final hits = localE2eMessageSearch(messages: messages, query: 'blocked');
      expect(hits, isEmpty);
    });
  });
}
