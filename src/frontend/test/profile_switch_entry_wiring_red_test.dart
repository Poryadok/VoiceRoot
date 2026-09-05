import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/profile_switch_coordinator.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/profile/create_profile_sheet.dart';
import 'package:voice_frontend/ui/profile/profile_avatar_menu.dart';
import 'package:voice_frontend/ui/profile/profile_avatar_switcher.dart';
import 'package:voice_frontend/ui/profile/profile_edit_sheet.dart';
import 'package:voice_frontend/ui/shell/desktop_shell_rail.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

/// T-053 Cycle 2b RED contract.
///
/// These are the four mounted product entry paths from multi-profile.md: the
/// desktop rail menu, the mobile avatar menu, mobile swipe, and profile
/// creation. They use the real coordinator provider; only HTTP, durable
/// storage, and the Realtime boundary are external test boundaries.
void main() {
  group('mounted profile-switch entry wiring', () {
    testWidgets('desktop rail menu selection owns one coordinator transition', (
      tester,
    ) async {
      final harness = _EntryHarness();
      _disposeHarnessAfterWidget(tester, harness);
      await _pumpVoiceApp(tester, harness, const Size(1280, 800));

      expect(find.byKey(DesktopShellRail.railKey), findsOneWidget);
      await tester.tap(find.byKey(ProfileAvatarMenuButton.railKey));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Gaming').last);

      await _expectOnePausedCoordinatorTransition(tester, harness);
      await _completeCoordinatorTransition(tester, harness);
      expect(
        harness.container.read(authControllerProvider).activeProfileId,
        'profile-alt',
      );
      await _disposeMountedHarness(tester, harness);
    });

    testWidgets(
      'mobile avatar tap menu selection owns one coordinator transition',
      (tester) async {
        final harness = _EntryHarness();
        _disposeHarnessAfterWidget(tester, harness);
        await _pumpVoiceApp(tester, harness, const Size(390, 800));

        await tester.tap(find.byKey(ProfileAvatarSwitcher.switcherKey));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Gaming').last);

        await _expectOnePausedCoordinatorTransition(tester, harness);
        await _completeCoordinatorTransition(tester, harness);
        expect(
          harness.container.read(authControllerProvider).activeProfileId,
          'profile-alt',
        );
        await _disposeMountedHarness(tester, harness);
      },
    );

    testWidgets('mobile avatar swipe owns one coordinator transition', (
      tester,
    ) async {
      final harness = _EntryHarness();
      _disposeHarnessAfterWidget(tester, harness);
      await _pumpVoiceApp(tester, harness, const Size(390, 800));

      await tester.fling(
        find.byKey(ProfileAvatarSwitcher.switcherKey),
        const Offset(-200, 0),
        1000,
      );

      await _expectOnePausedCoordinatorTransition(tester, harness);
      await _completeCoordinatorTransition(tester, harness);
      expect(
        harness.container.read(authControllerProvider).activeProfileId,
        'profile-alt',
      );
      await _disposeMountedHarness(tester, harness);
    });

    testWidgets(
      'successful CreateProfileSheet switches through coordinator before closing',
      (tester) async {
        final harness = _EntryHarness(profiles: const [_primaryProfile]);
        _disposeHarnessAfterWidget(tester, harness);
        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: harness.container,
            child: MaterialApp(
              theme: voiceTestTheme(),
              locale: const Locale('en'),
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: Scaffold(
                body: Builder(
                  builder: (context) => FilledButton(
                    key: const Key('open_create_profile_sheet'),
                    onPressed: () => showCreateProfileSheet(context),
                    child: const Text('Open create profile'),
                  ),
                ),
              ),
            ),
          ),
        );
        await tester.tap(find.byKey(const Key('open_create_profile_sheet')));
        await tester.pumpAndSettle();
        await tester.enterText(
          find.byKey(CreateProfileSheet.displayNameFieldKey),
          'Created profile',
        );
        await tester.tap(find.byKey(CreateProfileSheet.submitKey));

        await _expectOnePausedCoordinatorTransition(
          tester,
          harness,
          expectedProfileId: 'profile-created',
        );
        expect(harness.createRequests, 1);
        expect(find.byKey(CreateProfileSheet.sheetKey), findsOneWidget);

        await _completeCoordinatorTransition(tester, harness);
        expect(find.byKey(CreateProfileSheet.sheetKey), findsNothing);
        expect(
          harness.container.read(authControllerProvider).activeProfileId,
          'profile-created',
        );
        await _disposeMountedHarness(tester, harness);
      },
    );

    testWidgets(
      'failed CreateProfileSheet keeps its picked avatar and never switches',
      (tester) async {
        final harness = _EntryHarness(
          profiles: const [_primaryProfile],
          rejectCreate: true,
        );
        _disposeHarnessAfterWidget(tester, harness);
        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: harness.container,
            child: MaterialApp(
              theme: voiceTestTheme(),
              locale: const Locale('en'),
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: Scaffold(
                body: CreateProfileSheet(
                  avatarPicker: () async => ProfileAvatarFile(
                    bytes: base64Decode(
                      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScL8+wAAAABJRU5ErkJggg==',
                    ),
                    contentType: 'image/png',
                    name: 'avatar.png',
                  ),
                ),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(CreateProfileSheet.avatarButtonKey));
        await tester.pump();
        await tester.enterText(
          find.byKey(CreateProfileSheet.displayNameFieldKey),
          'Rejected profile',
        );
        await tester.tap(find.byKey(CreateProfileSheet.submitKey));
        await tester.pumpAndSettle();

        expect(harness.createRequests, 1);
        expect(harness.authSwitchRequests, 0);
        expect(harness.realtime.handoffs, isEmpty);
        expect(harness.avatarRequests, 0);
        expect(
          harness.container.read(profileSwitchInProgressProvider),
          isFalse,
        );
        expect(find.byKey(CreateProfileSheet.sheetKey), findsOneWidget);
        expect(find.text('create denied'), findsOneWidget);
        expect(
          tester
              .widget<CircleAvatar>(
                find.descendant(
                  of: find.byKey(CreateProfileSheet.sheetKey),
                  matching: find.byType(CircleAvatar),
                ),
              )
              .backgroundImage,
          isA<MemoryImage>(),
        );
        expect(
          harness.container.read(authControllerProvider).activeProfileId,
          'profile-primary',
        );
        await _disposeMountedHarness(tester, harness);
      },
    );
  });
}

Future<void> _pumpVoiceApp(
  WidgetTester tester,
  _EntryHarness harness,
  Size surfaceSize,
) async {
  final messenger =
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
  const voipChannel = MethodChannel('voice/voip');
  messenger.setMockMethodCallHandler(voipChannel, (_) async => null);
  await tester.binding.setSurfaceSize(surfaceSize);
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: harness.container,
      child: const VoiceApp(locale: Locale('en')),
    ),
  );
  await tester.pumpAndSettle();
}

void _disposeHarnessAfterWidget(
  WidgetTester tester,
  _EntryHarness harness,
) {
  addTearDown(harness.dispose);
}

Future<void> _disposeMountedHarness(
  WidgetTester tester,
  _EntryHarness harness,
) async {
  await tester.pumpWidget(const SizedBox.shrink());
  harness.dispose();
  await tester.pump();
}

Future<void> _expectOnePausedCoordinatorTransition(
  WidgetTester tester,
  _EntryHarness harness, {
  String expectedProfileId = 'profile-alt',
}) async {
  await tester.pump();

  expect(harness.authSwitchRequests, 1);
  expect(harness.realtime.handoffs, hasLength(1));
  expect(
    harness.realtime.handoffs.single.nextSession.activeProfileId,
    expectedProfileId,
  );
  expect(harness.container.read(profileSwitchInProgressProvider), isTrue);
}

Future<void> _completeCoordinatorTransition(
  WidgetTester tester,
  _EntryHarness harness,
) async {
  harness.realtime.complete();
  await tester.pumpAndSettle();
  expect(harness.container.read(profileSwitchInProgressProvider), isFalse);
}

const _primaryProfile = VoiceProfile(
  id: 'profile-primary',
  accountId: 'account-1',
  username: 'primary',
  discriminator: '0001',
  displayName: 'Primary',
  isPrimary: true,
);

const _altProfile = VoiceProfile(
  id: 'profile-alt',
  accountId: 'account-1',
  username: 'gaming',
  discriminator: '0002',
  displayName: 'Gaming',
);

const _createdProfile = VoiceProfile(
  id: 'profile-created',
  accountId: 'account-1',
  username: 'created',
  discriminator: '0003',
  displayName: 'Created profile',
);

const _primarySession = AuthSession(
  accessToken: 'access-primary',
  refreshToken: 'refresh-primary',
  expiresInSeconds: 900,
  accountId: 'account-1',
  activeProfileId: 'profile-primary',
);

class _EntryHarness {
  _EntryHarness({
    this.profiles = const [_primaryProfile, _altProfile],
    this.rejectCreate = false,
  }) {
    final client = MockClient(_respond);
    storage = _MemoryAuthStorage(_primarySession);
    realtime = _PausedProfileSwitchRealtimeBoundary();
    container = ProviderContainer(
      overrides: [
        ...voiceThemeTestOverrides(),
        profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
        authSessionStorageProvider.overrideWithValue(storage),
        guestCredentialsStorageProvider.overrideWithValue(
          InMemoryGuestCredentialsStorage(),
        ),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(client),
        realtimeAutoConnectProvider.overrideWithValue(false),
        authControllerProvider.overrideWith((ref) {
          final controller = AuthController(
            authClient: ref.watch(voiceAuthClientProvider),
            storage: ref.watch(authSessionStorageProvider),
            guestCredentialsStorage: ref.watch(guestCredentialsStorageProvider),
          );
          controller.state = const AuthState(session: _primarySession);
          return controller;
        }),
        profileSwitchRealtimeBoundaryProvider.overrideWithValue(realtime),
      ],
    );
  }

  final List<VoiceProfile> profiles;
  final bool rejectCreate;
  late final ProviderContainer container;
  late final _MemoryAuthStorage storage;
  late final _PausedProfileSwitchRealtimeBoundary realtime;
  var authSwitchRequests = 0;
  var createRequests = 0;
  var avatarRequests = 0;
  var _disposed = false;

  Future<http.Response> _respond(http.Request request) async {
    if (request.url.path == '/health') return http.Response('OK', 200);
    if (request.url.path == '/api/v1/auth/switch-profile') {
      authSwitchRequests++;
      final profileId =
          (jsonDecode(request.body) as Map<String, dynamic>)['profile_id']
              as String?;
      final session = switch (profileId) {
        'profile-alt' => _sessionFor('profile-alt'),
        'profile-created' => _sessionFor('profile-created'),
        _ => null,
      };
      if (session == null) return http.Response('unknown profile', 400);
      return http.Response(jsonEncode(session.toJson()), 200);
    }
    if (request.url.path == '/api/v1/users/profiles' &&
        request.method == 'POST') {
      createRequests++;
      if (rejectCreate) {
        return http.Response(
          jsonEncode({'error': 'create_denied', 'message': 'create denied'}),
          403,
        );
      }
      return http.Response(
        jsonEncode({'profile': _profileJson(_createdProfile)}),
        200,
      );
    }
    if (request.url.path == '/api/v1/users/profiles') {
      return http.Response(
        jsonEncode({
          'profile_list': {'profiles': profiles.map(_profileJson).toList()},
        }),
        200,
      );
    }
    if (request.url.path.startsWith('/api/v1/users/profiles/')) {
      final id = request.url.path.split('/').last;
      final profile = [
        ...profiles,
        _createdProfile,
      ].where((item) => item.id == id);
      if (profile.isEmpty) return http.Response('not found', 404);
      return http.Response(
        jsonEncode({'profile': _profileJson(profile.single)}),
        200,
      );
    }
    if (request.url.path.contains('avatar')) avatarRequests++;
    if (request.url.host == 'upload.test') avatarRequests++;
    return http.Response('not found', 404);
  }

  void dispose() {
    if (_disposed) return;
    _disposed = true;
    container.dispose();
  }
}

AuthSession _sessionFor(String profileId) => AuthSession(
  accessToken: 'access-$profileId',
  refreshToken: 'refresh-$profileId',
  expiresInSeconds: 900,
  accountId: 'account-1',
  activeProfileId: profileId,
);

Map<String, Object?> _profileJson(VoiceProfile profile) => {
  'id': profile.id,
  'account_id': profile.accountId,
  'username': profile.username,
  'discriminator': profile.discriminator,
  'display_name': profile.displayName,
  'is_primary': profile.isPrimary,
  'verification_type': profile.verificationType,
};

class _MemoryAuthStorage implements AuthSessionStorage {
  _MemoryAuthStorage(this._session);

  AuthSession? _session;

  @override
  Future<void> clear() async => _session = null;

  @override
  Future<AuthSession?> read() async => _session;

  @override
  Future<void> write(AuthSession session) async => _session = session;
}

class _PausedProfileSwitchRealtimeBoundary
    implements ProfileSwitchRealtimeBoundary {
  @override
  final Set<String> activeSubscriptions = {};

  final List<ProfileSwitchHandoff> handoffs = [];
  final Completer<void> _completion = Completer<void>();

  @override
  Future<void> retireAndReconnect(ProfileSwitchHandoff handoff) {
    handoffs.add(handoff);
    return _completion.future;
  }

  void complete() {
    if (!_completion.isCompleted) _completion.complete();
  }
}
