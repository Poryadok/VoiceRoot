import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/discover_hint_storage.dart';
import 'package:voice_frontend/state/auth_providers.dart';

final testDiscoverHintStorage = InMemoryDiscoverHintStorage();

/// Pre-authenticated [AuthController] for widget tests of the main shell.
AuthController authenticatedAuthController(Ref ref) {
  final controller = AuthController(
    authClient: ref.watch(voiceAuthClientProvider),
    storage: ref.watch(authSessionStorageProvider),
  );
  controller.state = const AuthState(
    session: AuthSession(
      accessToken: 'test-access',
      refreshToken: 'test-refresh',
      accountId: 'acc-test',
      activeProfileId: 'prof-test',
      expiresInSeconds: 900,
    ),
  );
  return controller;
}
