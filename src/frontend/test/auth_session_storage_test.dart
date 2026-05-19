import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';

void main() {
  group('InMemoryAuthSessionStorage', () {
    test('write read round-trip', () async {
      final storage = InMemoryAuthSessionStorage();
      const session = AuthSession(
        accessToken: 'a',
        refreshToken: 'r',
        accountId: 'acc',
        activeProfileId: 'prof',
        expiresInSeconds: 900,
      );
      await storage.write(session);
      final loaded = await storage.read();
      expect(loaded, session);
    });

    test('clear removes session', () async {
      final storage = InMemoryAuthSessionStorage();
      await storage.write(
        const AuthSession(
          accessToken: 'a',
          refreshToken: 'r',
          accountId: 'acc',
          activeProfileId: 'prof',
          expiresInSeconds: 900,
        ),
      );
      await storage.clear();
      expect(await storage.read(), isNull);
    });
  });
}
