import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import '../backend/sdk_authorization_client.dart';
import 'auth_providers.dart';

/// The owning SDK/device module must override this with its Auth-registered
/// device-key signer. Auth-issued `kid` selects the non-exportable P-256 key;
/// the adapter signs the exact UTF-8 payload and returns compact ES256 JWS.
final sdkDeviceProofSignerProvider = Provider<SdkDeviceProofSigner>((ref) {
  return (handle, payload) => throw StateError(
    'No registered SDK device signer is configured for $handle',
  );
});

final sdkAuthorizationHandoffStorageProvider =
    Provider<SdkAuthorizationHandoffStorage>((ref) {
      return SecureSdkAuthorizationHandoffStorage(const FlutterSecureStorage());
    });

final sdkLinkedSessionStorageProvider = Provider<SdkLinkedSessionStorage>((
  ref,
) {
  return SecureSdkLinkedSessionStorage(const FlutterSecureStorage());
});

final sdkAuthorizationClientProvider = Provider<SdkAuthorizationClient>((ref) {
  final handoffs = ref.watch(sdkAuthorizationHandoffStorageProvider);
  final sessions = ref.watch(sdkLinkedSessionStorageProvider);
  unawaited(handoffs.pruneExpired());
  unawaited(sessions.pruneExpired());
  return SdkAuthorizationClient(
    gateway: ref.watch(gatewayHttpClientProvider),
    handoffStorage: handoffs,
    linkedSessionStorage: sessions,
    proofSigner: ref.watch(sdkDeviceProofSignerProvider),
  );
});
