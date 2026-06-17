import 'dart:typed_data';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:http/testing.dart';
import 'package:libsignal_protocol_dart/libsignal_protocol_dart.dart';
import 'package:voice_frontend/backend/e2e_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/e2e/e2e_crypto_adapter.dart';
import 'package:voice_frontend/e2e/e2e_file_crypto.dart';
import 'package:voice_frontend/e2e/e2e_message_service.dart';
import 'package:voice_frontend/e2e/e2e_store_factory.dart';
import 'package:voice_frontend/state/e2e_providers.dart';

/// Bilateral E2E session + encrypted file payload for widget tests.
class E2eAttachmentTestFixture {
  E2eAttachmentTestFixture._({
    required this.adapter,
    required this.messageService,
    required this.localProfileId,
    required this.peerProfileId,
  });

  final E2eCryptoAdapter adapter;
  final E2eMessageService messageService;
  final String localProfileId;
  final String peerProfileId;

  static Future<E2eAttachmentTestFixture> create({
    String localProfileId = 'prof-test',
    String peerProfileId = 'peer-b',
  }) async {
    final adapter = E2eCryptoAdapter.inMemoryForTest();
    final localStore =
        await adapter.sessionManager.storeForProfile(localProfileId);
    final peerStore = await adapter.sessionManager.storeForProfile(peerProfileId);
    await establishBilateralSession(
      localStore: localStore as InMemorySignalProtocolStore,
      remoteStore: peerStore as InMemorySignalProtocolStore,
      localAddress: signalAddressForProfile(localProfileId),
      remoteAddress: signalAddressForProfile(peerProfileId),
    );
    final gateway = GatewayHttpClient(
      httpClient: MockClient((_) async => throw StateError('unexpected HTTP')),
      config: const GatewayConfig(baseUrl: 'http://api.test'),
    );
    final messageService = E2eMessageService(
      adapter: adapter,
      e2eClient: VoiceE2eClient(gateway: gateway, adapter: adapter),
    );
    return E2eAttachmentTestFixture._(
      adapter: adapter,
      messageService: messageService,
      localProfileId: localProfileId,
      peerProfileId: peerProfileId,
    );
  }

  Future<({Uint8List ciphertext, String keyWire})> encryptFileForPeer({
    required Uint8List plaintext,
    String chatId = 'chat-e2e-attach',
  }) async {
    const crypto = E2eFileCrypto();
    final encrypted = await crypto.encryptBytes(
      plaintext: plaintext,
      messageService: messageService,
      localProfileId: peerProfileId,
      peerProfileId: localProfileId,
      authorization: '',
      chatId: '',
    );
    return (ciphertext: encrypted.ciphertext, keyWire: encrypted.keyWire);
  }

  List<Override> providerOverrides() => [
        e2eCryptoAdapterProvider.overrideWithValue(adapter),
        e2eMessageServiceProvider.overrideWithValue(messageService),
      ];
}
