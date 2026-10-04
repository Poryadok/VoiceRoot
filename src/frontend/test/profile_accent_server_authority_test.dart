import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/profile_context_controller.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/theme/profile_accent_storage.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/settings/settings_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  test('server accent wins over stale local override and index', () async {
    final storage = InMemoryProfileAccentStorage();
    await storage.writeOverride('prof-test', '#C9B8FF');
    await storage.writeProfileIndex('prof-test', 1);
    final users = _AccentUsersClient(
      profiles: {'prof-test': _profile('prof-test', accentColor: '#F0A8A8')},
    );
    final container = _container(users, storage);
    addTearDown(container.dispose);

    expect(
      await container.read(profileAccentColorProvider('prof-test').future),
      const Color(0xFFF0A8A8),
    );
  });

  testWidgets(
    'failed picker update keeps the server selection and legacy data',
    (tester) async {
      final storage = InMemoryProfileAccentStorage();
      await storage.writeOverride('prof-test', '#C9B8FF');
      await storage.writeProfileIndex('prof-test', 1);
      final users = _AccentUsersClient(
        profiles: {'prof-test': _profile('prof-test', accentColor: '#7EC8E3')},
      )..failUpdates = true;
      final container = _container(users, storage);
      addTearDown(container.dispose);

      await _pumpSettings(tester, container);
      expect(_selectedSwatch(tester), 0);

      await tester.tap(_swatch(2));
      await tester.pumpAndSettle();

      expect(_selectedSwatch(tester), 0);
      expect(users.updates, ['#F0A8A8']);
      expect(await storage.readProfileIndex('prof-test'), 1);
      expect(await storage.readOverride('prof-test'), '#C9B8FF');
    },
  );

  testWidgets('successful picker update follows returned server accent', (
    tester,
  ) async {
    final storage = InMemoryProfileAccentStorage();
    await storage.writeOverride('prof-test', '#C9B8FF');
    await storage.writeProfileIndex('prof-test', 1);
    final users = _AccentUsersClient(
      profiles: {'prof-test': _profile('prof-test', accentColor: '#7EC8E3')},
    );
    final container = _container(users, storage);
    addTearDown(container.dispose);

    await _pumpSettings(tester, container);
    await tester.tap(_swatch(2));
    await tester.pumpAndSettle();

    expect(users.updates, ['#F0A8A8']);
    expect(_selectedSwatch(tester), 2);
    expect(
      await container.read(profileAccentColorProvider('prof-test').future),
      const Color(0xFFF0A8A8),
    );
    expect(await storage.readProfileIndex('prof-test'), isNull);
    expect(await storage.readOverride('prof-test'), isNull);
  });

  testWidgets(
    'late picker response for the previous profile cannot replace the new profile selection',
    (tester) async {
      final storage = InMemoryProfileAccentStorage();
      final users = _AccentUsersClient(
        profiles: {
          'prof-test': _profile('prof-test', accentColor: '#7EC8E3'),
          'profile-b': _profile('profile-b', accentColor: '#9ED9A6'),
        },
      )..holdNextUpdate = true;
      final container = _container(users, storage);
      addTearDown(container.dispose);

      await _pumpSettings(tester, container);
      final pending = users.updateStarted.future;
      await tester.tap(_swatch(2));
      await pending;

      users.activeProfileId = 'profile-b';
      container.read(authControllerProvider.notifier).state = AuthState(
        session: _session('profile-b'),
      );
      await tester.pumpAndSettle();
      expect(_selectedSwatch(tester), 1);

      users.completeHeldUpdateSuccess();
      await tester.pumpAndSettle();

      expect(_selectedSwatch(tester), 1);
      expect(users.profiles['profile-b']!.accentColor, '#9ED9A6');
      expect(users.profiles['prof-test']!.accentColor, '#F0A8A8');
    },
  );

  test(
    'legacy accent migration clears local data after server success',
    () async {
      final storage = InMemoryProfileAccentStorage();
      await storage.writeOverride('profile-b', '#F5E6A3');
      await storage.writeProfileIndex('profile-b', 1);
      final users = _AccentUsersClient(
        profiles: {
          'profile-a': _profile('profile-a', accentColor: '#7EC8E3'),
          'profile-b': _profile('profile-b'),
        },
      );
      final container = _container(users, storage);
      addTearDown(container.dispose);
      container.read(profileContextCoordinatorProvider);

      await _switchToProfileB(container, users);
      await users.updateStarted.future.timeout(const Duration(seconds: 1));
      await _flush();

      expect(users.updates, ['#F5E6A3']);
      expect(await storage.readOverride('profile-b'), isNull);
      expect(await storage.readProfileIndex('profile-b'), isNull);
    },
  );

  test('failed legacy accent migration retains local data for retry', () async {
    final storage = InMemoryProfileAccentStorage();
    await storage.writeOverride('profile-b', '#F5E6A3');
    await storage.writeProfileIndex('profile-b', 1);
    final users = _AccentUsersClient(
      profiles: {
        'profile-a': _profile('profile-a', accentColor: '#7EC8E3'),
        'profile-b': _profile('profile-b'),
      },
    )..failUpdates = true;
    final container = _container(users, storage);
    addTearDown(container.dispose);
    container.read(profileContextCoordinatorProvider);

    await _switchToProfileB(container, users);
    await users.updateStarted.future.timeout(const Duration(seconds: 1));
    await _flush();

    expect(users.updates, ['#F5E6A3']);
    expect(await storage.readOverride('profile-b'), '#F5E6A3');
    expect(await storage.readProfileIndex('profile-b'), 1);
  });
}

ProviderContainer _container(
  _AccentUsersClient users,
  InMemoryProfileAccentStorage storage,
) => ProviderContainer(
  overrides: [
    ...voiceAppTestOverrides(
      client: MockClient((_) async => http.Response('{}', 404)),
    ),
    profileAccentStorageProvider.overrideWithValue(storage),
    voiceUsersClientProvider.overrideWithValue(users),
  ],
);

Future<void> _pumpSettings(
  WidgetTester tester,
  ProviderContainer container,
) async {
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: MaterialApp(
        theme: voiceTestTheme(),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const Scaffold(body: SettingsSheet()),
      ),
    ),
  );
  await tester.pumpAndSettle();
  await tester.ensureVisible(find.byKey(SettingsSheet.accentKey));
  await tester.pumpAndSettle();
}

Finder _swatch(int index) => find
    .descendant(
      of: find.byKey(SettingsSheet.accentKey),
      matching: find.byType(GestureDetector),
    )
    .at(index);

int _selectedSwatch(WidgetTester tester) {
  for (
    var index = 0;
    index < testVoiceTokenCatalog.profileAccentDefaults.length;
    index++
  ) {
    final circle = tester.widget<Container>(
      find
          .descendant(of: _swatch(index), matching: find.byType(Container))
          .first,
    );
    final decoration = circle.decoration! as BoxDecoration;
    if (decoration.border!.top.width == 2) return index;
  }
  throw StateError('No profile accent swatch is selected');
}

Future<void> _switchToProfileB(
  ProviderContainer container,
  _AccentUsersClient users,
) {
  users.activeProfileId = 'profile-b';
  container.read(authControllerProvider.notifier).state = AuthState(
    session: _session('profile-b'),
  );
  return Future<void>.value();
}

AuthSession _session(String profileId) => AuthSession(
  accessToken: 'token-$profileId',
  refreshToken: 'refresh-$profileId',
  accountId: 'account-a',
  activeProfileId: profileId,
  expiresInSeconds: 900,
);

VoiceProfile _profile(String id, {String? accentColor}) => VoiceProfile(
  id: id,
  accountId: 'account-a',
  username: id,
  discriminator: '0001',
  displayName: id,
  accentColor: accentColor,
);

Future<void> _flush() => Future<void>.delayed(Duration.zero);

class _AccentUsersClient extends VoiceUsersClient {
  _AccentUsersClient({required this.profiles})
    : super(
        gateway: GatewayHttpClient(
          httpClient: MockClient((_) async => http.Response('{}', 404)),
          config: const GatewayConfig(baseUrl: 'http://api.test'),
        ),
      );

  final Map<String, VoiceProfile> profiles;
  final List<String> updates = [];
  String activeProfileId = 'prof-test';
  bool failUpdates = false;
  bool holdNextUpdate = false;
  final Completer<void> updateStarted = Completer<void>();
  Completer<UsersApiResult<VoiceProfile>>? _heldUpdate;
  String? _heldProfileId;
  String? _heldAccent;

  @override
  Future<UsersApiResult<VoiceProfile>> getProfile({
    required String authorization,
    required String profileId,
  }) async => UsersApiOk(profiles[profileId] ?? _profile(profileId));

  @override
  Future<UsersApiResult<VoiceProfile>> updateProfile({
    required String authorization,
    String? displayName,
    String? bio,
    String? avatarUrl,
    String? accentColor,
  }) async {
    updates.add(accentColor ?? '');
    final targetProfile = activeProfileId;
    if (!updateStarted.isCompleted) updateStarted.complete();
    if (holdNextUpdate) {
      holdNextUpdate = false;
      _heldProfileId = targetProfile;
      _heldAccent = accentColor;
      _heldUpdate = Completer<UsersApiResult<VoiceProfile>>();
      return _heldUpdate!.future;
    }
    if (failUpdates) {
      return const UsersApiFailure(message: 'update failed');
    }
    final profile = _profile(targetProfile, accentColor: accentColor);
    profiles[targetProfile] = profile;
    return UsersApiOk(profile);
  }

  void completeHeldUpdateSuccess() {
    final profileId = _heldProfileId!;
    final profile = _profile(profileId, accentColor: _heldAccent);
    profiles[profileId] = profile;
    _heldUpdate!.complete(UsersApiOk(profile));
  }
}
