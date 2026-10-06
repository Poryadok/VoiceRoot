import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
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

    test('conditional clear does not remove a newer session', () async {
      final storage = InMemoryAuthSessionStorage();
      const oldSession = AuthSession(
        accessToken: 'old-access',
        refreshToken: 'old-refresh',
        accountId: 'acc',
        activeProfileId: 'prof',
        expiresInSeconds: 900,
      );
      const newSession = AuthSession(
        accessToken: 'new-access',
        refreshToken: 'new-refresh',
        accountId: 'acc',
        activeProfileId: 'prof',
        expiresInSeconds: 900,
      );
      await storage.write(newSession);

      expect(await storage.clearIfUnchanged(oldSession), isFalse);
      expect(await storage.read(), newSession);
    });
  });

  test(
    'shared preferences serializes an older conditional clear before a new save',
    () async {
      SharedPreferences.setMockInitialValues({});
      final storage = SharedPreferencesAuthSessionStorage(
        await SharedPreferences.getInstance(),
      );
      const oldSession = AuthSession(
        accessToken: 'old-access',
        refreshToken: 'old-refresh',
        accountId: 'acc',
        activeProfileId: 'prof',
        expiresInSeconds: 900,
      );
      const newSession = AuthSession(
        accessToken: 'new-access',
        refreshToken: 'new-refresh',
        accountId: 'acc',
        activeProfileId: 'prof',
        expiresInSeconds: 900,
      );
      await storage.write(oldSession);

      final clear = storage.clearIfUnchanged(oldSession);
      final save = storage.write(newSession);
      expect(await clear, isTrue);
      await save;
      expect(await storage.read(), newSession);
    },
  );
}
