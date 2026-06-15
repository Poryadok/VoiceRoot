import 'dart:typed_data';

import 'package:libsignal_protocol_dart/libsignal_protocol_dart.dart';

import '../backend/e2e_client.dart';
import '../backend/messages_client.dart';
import 'e2e_crypto_adapter.dart';
import 'e2e_prekey_sync.dart';
import 'e2e_store_factory.dart';
import 'e2e_verification_code.dart';

typedef PeerIdentityObserver = void Function(
  String peerProfileId,
  Uint8List identityKeyBytes,
);

/// Encrypt/decrypt chat message bodies for E2E-enabled DMs.
class E2eMessageService {
  E2eMessageService({
    E2eCryptoAdapter? adapter,
    VoiceE2eClient? e2eClient,
    PeerIdentityObserver? onPeerIdentityObserved,
    bool Function(String peerProfileId)? isPeerDistrusted,
  })  : _adapter = adapter ?? E2eCryptoAdapter(),
        _e2eClient = e2eClient,
        _onPeerIdentityObserved = onPeerIdentityObserved,
        _isPeerDistrusted = isPeerDistrusted;

  final E2eCryptoAdapter _adapter;
  final VoiceE2eClient? _e2eClient;
  final PeerIdentityObserver? _onPeerIdentityObserved;
  final bool Function(String peerProfileId)? _isPeerDistrusted;

  E2eCryptoAdapter get adapter => _adapter;

  E2ePreKeySync get _preKeys => E2ePreKeySync(
        sessionManager: _adapter.sessionManager,
      );

  Future<String> encryptOutgoing({
    required String localProfileId,
    required String peerProfileId,
    required String plaintext,
    String? authorization,
    String? chatId,
  }) async {
    try {
      if (_e2eClient != null &&
          authorization != null &&
          authorization.isNotEmpty &&
          chatId != null &&
          chatId.isNotEmpty) {
        return await _e2eClient.encryptForChat(
          authorization: authorization,
          chatId: chatId,
          peerProfileId: peerProfileId,
          plaintext: plaintext,
        );
      }

      final session = await _adapter.ensureSession(
        localProfileId: localProfileId,
        remoteProfileId: peerProfileId,
      );
      return _adapter.encryptToWire(session: session, plaintext: plaintext);
    } on E2eEncryptException {
      rethrow;
    } on Object catch (e) {
      throw E2eEncryptException('e2e_encrypt_failed', cause: e);
    }
  }

  Future<VoiceMessage> decryptForDisplay({
    required VoiceMessage message,
    required String localProfileId,
    required String peerProfileId,
    String? authorization,
  }) async {
    if (!message.isE2e) return message;
    final senderProfile = _peerForMessage(
      message: message,
      localProfileId: localProfileId,
      fallbackPeerId: peerProfileId,
    );
    if (_isPeerDistrusted?.call(senderProfile) ?? false) {
      return message.copyWith(decryptionFailed: true);
    }
    try {
      final remoteBundle = await _fetchRemoteBundle(
        authorization: authorization,
        profileId: senderProfile,
      );
      final plaintext = await _adapter.decryptFromWire(
        receiverProfileId: localProfileId,
        senderProfileId: senderProfile,
        wire: message.content,
        remoteBundle: remoteBundle,
      );
      return message.copyWith(content: plaintext);
    } on E2eDecryptException {
      return message.copyWith(decryptionFailed: true);
    }
  }

  Future<List<VoiceMessage>> decryptAllForDisplay({
    required List<VoiceMessage> messages,
    required String localProfileId,
    required String? peerProfileId,
    String? authorization,
  }) async {
    if (peerProfileId == null || peerProfileId.isEmpty) return messages;
    final out = <VoiceMessage>[];
    for (final message in messages) {
      out.add(
        await decryptForDisplay(
          message: message,
          localProfileId: localProfileId,
          peerProfileId: _peerForMessage(
            message: message,
            localProfileId: localProfileId,
            fallbackPeerId: peerProfileId,
          ),
          authorization: authorization,
        ),
      );
    }
    return out;
  }

  Future<PreKeyBundle?> _fetchRemoteBundle({
    required String? authorization,
    required String profileId,
  }) async {
    if (_e2eClient == null ||
        authorization == null ||
        authorization.isEmpty) {
      return null;
    }
    final result = await _e2eClient.getPreKeyBundle(
      authorization: authorization,
      profileId: profileId,
    );
    if (result is! E2eApiOk<String> || result.data.isEmpty) {
      return null;
    }
    final wire = result.data;
    final parsed = parseSerializedPreKeyBundle(wire);
    if (parsed != null) {
      _onPeerIdentityObserved?.call(
        profileId,
        identityKeyBytesFromSerialized(parsed.getIdentityKey().serialize()),
      );
    }
    return _preKeys.bundleFromWire(wire);
  }

  String _peerForMessage({
    required VoiceMessage message,
    required String localProfileId,
    required String fallbackPeerId,
  }) {
    if (message.senderProfileId != localProfileId) {
      return message.senderProfileId;
    }
    return fallbackPeerId;
  }
}

/// Local-only search over decrypted message bodies (E2E chats).
List<VoiceMessage> localE2eMessageSearch({
  required List<VoiceMessage> messages,
  required String query,
}) {
  final needle = query.trim().toLowerCase();
  if (needle.isEmpty) return const [];
  return messages
      .where(
        (m) =>
            !m.decryptionFailed &&
            m.content.toLowerCase().contains(needle),
      )
      .toList(growable: false);
}
