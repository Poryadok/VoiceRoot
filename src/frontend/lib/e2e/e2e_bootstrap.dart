import '../backend/e2e_client.dart';

/// Uploads local pre-keys after auth (best-effort, non-blocking for login UX).
class E2eBootstrapService {
  const E2eBootstrapService({required VoiceE2eClient e2eClient})
      : _e2eClient = e2eClient;

  final VoiceE2eClient _e2eClient;

  Future<void> ensurePreKeysUploaded({required String authorization}) async {
    final result = await _e2eClient.uploadPreKeyBundle(
      authorization: authorization,
    );
    if (result is E2eApiFailure) {
      throw StateError(result.message);
    }
  }
}
