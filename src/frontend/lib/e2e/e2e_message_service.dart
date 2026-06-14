import '../backend/messages_client.dart';
import 'e2e_crypto_adapter.dart';

/// Encrypt/decrypt chat message bodies for E2E-enabled DMs.
class E2eMessageService {
  E2eMessageService({E2eCryptoAdapter? adapter})
      : _adapter = adapter ?? E2eCryptoAdapter();

  final E2eCryptoAdapter _adapter;

  E2eCryptoAdapter get adapter => _adapter;

  Future<String> encryptOutgoing({
    required String localProfileId,
    required String peerProfileId,
    required String plaintext,
  }) async {
    final session = await _adapter.ensureSession(
      localProfileId: localProfileId,
      remoteProfileId: peerProfileId,
    );
    return _adapter.encryptToWire(session: session, plaintext: plaintext);
  }

  Future<VoiceMessage> decryptForDisplay({
    required VoiceMessage message,
    required String localProfileId,
    required String peerProfileId,
  }) async {
    if (!message.isE2e) return message;
    try {
      final senderId = message.senderProfileId;
      final plaintext = await _adapter.decryptFromWire(
        receiverProfileId: localProfileId,
        senderProfileId: senderId != localProfileId
            ? senderId
            : peerProfileId,
        wire: message.content,
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
        ),
      );
    }
    return out;
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
