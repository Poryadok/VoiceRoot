import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:libsignal_protocol_dart/libsignal_protocol_dart.dart';
import 'package:voice_frontend/e2e/e2e_crypto_adapter.dart';
import 'package:voice_frontend/e2e/e2e_file_crypto.dart';
import 'package:voice_frontend/e2e/e2e_message_service.dart';
import 'package:voice_frontend/e2e/e2e_store_factory.dart';

void main() {
  group('E2eFileCrypto', () {
    test('encryptBytes then decryptBytes roundtrip', () async {
      final adapter = E2eCryptoAdapter.inMemoryForTest();
      const localId = 'profile-local';
      const peerId = 'profile-peer';
      final localStore = await adapter.sessionManager.storeForProfile(localId);
      final peerStore = await adapter.sessionManager.storeForProfile(peerId);
      await establishBilateralSession(
        localStore: localStore as InMemorySignalProtocolStore,
        remoteStore: peerStore as InMemorySignalProtocolStore,
        localAddress: signalAddressForProfile(localId),
        remoteAddress: signalAddressForProfile(peerId),
      );

      final messageService = E2eMessageService(adapter: adapter);
      const crypto = E2eFileCrypto();
      final plaintext = Uint8List.fromList([1, 2, 3, 4, 5, 0xff]);
      final encrypted = await crypto.encryptBytes(
        plaintext: plaintext,
        messageService: messageService,
        localProfileId: localId,
        peerProfileId: peerId,
        authorization: 'Bearer test',
        chatId: 'chat-1',
      );
      expect(encrypted.ciphertext, isNot(equals(plaintext)));

      final decrypted = await crypto.decryptBytes(
        ciphertext: encrypted.ciphertext,
        keyWire: encrypted.keyWire,
        messageService: messageService,
        localProfileId: peerId,
        peerProfileId: localId,
      );
      expect(decrypted, plaintext);
    });
  });
}
