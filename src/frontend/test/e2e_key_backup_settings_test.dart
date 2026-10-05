import 'dart:async';
import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/services.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/e2e_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/e2e/e2e_key_backup_v2.dart';
import 'package:voice_frontend/e2e/secure_signal_store.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/e2e_providers.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/ui/chat/e2e_chat_settings.dart';
import 'package:voice_frontend/ui/settings/e2e_key_backup_screen.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/in_memory_secure_signal_storage.dart';

class _SettingsClient extends VoiceE2eClient {
  _SettingsClient()
    : super(
        gateway: GatewayHttpClient(
          httpClient: MockClient((_) async => http.Response('', 204)),
          config: const GatewayConfig(baseUrl: 'https://voice.test'),
        ),
      );

  E2eKeyBackupData? backup;
  var saves = 0;
  var restores = 0;
  var deletions = 0;
  var getAttempts = 0;
  var failFirstGet = false;
  Completer<E2eApiResult<E2eKeyBackupData>>? delayedGet;
  Completer<void>? saveGate;

  @override
  Future<E2eApiResult<E2eKeyBackupData>> getKeyBackup({
    required String authorization,
  }) async {
    getAttempts++;
    if (delayedGet != null) return delayedGet!.future;
    if (failFirstGet && getAttempts == 1) {
      return const E2eApiFailure(
        message: 'private backend failure',
        statusCode: 503,
      );
    }
    return backup == null
        ? const E2eApiFailure(message: 'missing', statusCode: 404)
        : E2eApiOk(backup!);
  }

  @override
  Future<E2eApiResult<void>> putKeyBackup({
    required String authorization,
    required String password,
    String? passwordHint,
  }) async {
    saves++;
    if (saveGate != null) await saveGate!.future;
    backup = E2eKeyBackupData(
      encryptedBlob: 'opaque-test-value',
      passwordHint: passwordHint,
    );
    return const E2eApiOk(null);
  }

  @override
  Future<E2eApiResult<void>> restoreKeyBackup({
    required String authorization,
    required String password,
    required bool Function() isAuthorizationCurrent,
  }) async {
    if (!isAuthorizationCurrent()) {
      return const E2eApiFailure(message: 'session changed; try again');
    }
    restores++;
    return const E2eApiOk(null);
  }

  @override
  Future<E2eApiResult<void>> deleteKeyBackup({
    required String authorization,
  }) async {
    deletions++;
    backup = null;
    return const E2eApiOk(null);
  }
}

Widget _settingsApp(_SettingsClient client, {ThemeData? theme}) =>
    ProviderScope(
      overrides: [
        voiceE2eClientProvider.overrideWithValue(client),
        authorizationHeaderProvider.overrideWith(
          (ref) => _authorization('profile'),
        ),
      ],
      child: MaterialApp(
        theme: theme,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const E2eKeyBackupScreen(),
      ),
    );

String _authorization(String profileId) {
  final claims = base64Url
      .encode(utf8.encode(jsonEncode({'profile_id': profileId})))
      .replaceAll('=', '');
  return 'header.$claims.signature';
}

Future<VoiceE2eClient> _client({
  required String encryptedBlob,
  Future<http.Response> Function(http.Request request)? handler,
  Future<void> Function(String profileId, Map<String, dynamic> state)?
  importBackup,
  SecureSignalStorage? backupStorage,
}) async {
  final httpClient = MockClient(
    handler ??
        (_) async =>
            http.Response(jsonEncode({'encrypted_blob': encryptedBlob}), 200),
  );
  return VoiceE2eClient(
    gateway: GatewayHttpClient(
      httpClient: httpClient,
      config: const GatewayConfig(baseUrl: 'https://voice.test'),
    ),
    backupStorage: backupStorage,
    backupImporter: importBackup,
  );
}

Future<Map<String, dynamic>> _validPayload(String profileId) async {
  final state = await SecureSignalStore.exportForBackup(
    profileId,
    storage: InMemorySecureSignalStorage(),
  );
  return {'profile_id': profileId, 'version': 2, 'signal_state': state};
}

void main() {
  setUpAll(_loadCaptureFonts);

  group('E2E key backup Settings workflow', () {
    for (final capture in [
      (label: 'h', size: const Size(1280, 800)),
      (label: 'v', size: const Size(390, 844)),
    ]) {
      testWidgets('opaque VoiceTheme ${capture.label} capture', (tester) async {
        tester.view.physicalSize = capture.size;
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);
        final theme = await VoiceTheme.build(
          catalog: testVoiceTokenCatalog,
          mode: VoiceThemeMode.dark,
          profileAccent: testVoiceTokenCatalog.profileAccentAt(0),
        );
        final client = _SettingsClient()
          ..backup = const E2eKeyBackupData(
            encryptedBlob: 'opaque-visual-fixture',
            passwordHint: 'Use your backup password',
          );
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              voiceE2eClientProvider.overrideWithValue(client),
              authorizationHeaderProvider.overrideWith(
                (ref) => _authorization('profile'),
              ),
            ],
            child: MaterialApp(
              theme: theme,
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: RepaintBoundary(
                key: const ValueKey('e2e-backup-capture'),
                child: const E2eKeyBackupScreen(),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        await expectLater(
          find.byKey(const ValueKey('e2e-backup-capture')),
          matchesGoldenFile('goldens/e2e_key_backup_${capture.label}.png'),
        );
      });
    }

    testWidgets('disposal during status fetch ignores the late response', (
      tester,
    ) async {
      final client = _SettingsClient()
        ..delayedGet = Completer<E2eApiResult<E2eKeyBackupData>>();
      await tester.pumpWidget(_settingsApp(client));
      await tester.pump();
      expect(client.getAttempts, 1);

      await tester.pumpWidget(const MaterialApp(home: SizedBox.shrink()));
      client.delayedGet!.complete(
        const E2eApiFailure(message: 'late private error', statusCode: 503),
      );
      await tester.pump();
      expect(tester.takeException(), isNull);
    });

    testWidgets('disables repeat save while the request is pending', (
      tester,
    ) async {
      final client = _SettingsClient()..saveGate = Completer<void>();
      await tester.pumpWidget(_settingsApp(client));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(E2eKeyBackupScreen.passwordKey),
        'private test password',
      );
      await tester.tap(find.byKey(E2eKeyBackupScreen.saveKey));
      await tester.pump();
      expect(client.saves, 1);
      final button = tester.widget<FilledButton>(
        find.byKey(E2eKeyBackupScreen.saveKey),
      );
      expect(button.onPressed, isNull);

      client.saveGate!.complete();
      await tester.pumpAndSettle();
      expect(client.saves, 1);
    });

    testWidgets('hides raw load errors and retries the status request', (
      tester,
    ) async {
      final client = _SettingsClient()..failFirstGet = true;
      await tester.pumpWidget(_settingsApp(client));
      await tester.pumpAndSettle();
      expect(
        find.text('We couldn\'t update the encrypted key backup. Try again.'),
        findsOneWidget,
      );
      expect(find.text('private backend failure'), findsNothing);

      await tester.tap(find.text('Try again'));
      await tester.pumpAndSettle();
      expect(client.getAttempts, 2);
      expect(
        find.text('No encrypted key backup is saved for this account.'),
        findsOneWidget,
      );
    });

    testWidgets(
      'shows absent/present status, saves, restores, and confirms delete',
      (tester) async {
        final client = _SettingsClient();
        await tester.pumpWidget(_settingsApp(client));
        await tester.pumpAndSettle();
        expect(
          find.text('No encrypted key backup is saved for this account.'),
          findsOneWidget,
        );

        await tester.enterText(
          find.byKey(E2eKeyBackupScreen.passwordKey),
          'a strong password',
        );
        await tester.tap(find.byKey(E2eKeyBackupScreen.saveKey));
        await tester.pumpAndSettle();
        expect(client.saves, 1);
        expect(
          find.text('Encrypted key backup is available for this account.'),
          findsOneWidget,
        );

        await tester.enterText(
          find.byKey(E2eKeyBackupScreen.passwordKey),
          'a strong password',
        );
        await tester.tap(find.byKey(E2eKeyBackupScreen.restoreKey));
        await tester.pumpAndSettle();
        expect(client.restores, 1);

        await tester.tap(find.byKey(E2eKeyBackupScreen.deleteKey));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Cancel'));
        await tester.pumpAndSettle();
        expect(client.deletions, 0);
        expect(
          find.text('Encrypted key backup is available for this account.'),
          findsOneWidget,
        );

        await tester.tap(find.byKey(E2eKeyBackupScreen.deleteKey));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(E2eKeyBackupScreen.confirmDeleteKey));
        await tester.pumpAndSettle();
        expect(client.deletions, 1);
        expect(
          find.text('No encrypted key backup is saved for this account.'),
          findsOneWidget,
        );
      },
    );

    testWidgets('undecryptable history CTA opens the same Settings workflow', (
      tester,
    ) async {
      final client = _SettingsClient();
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            voiceE2eClientProvider.overrideWithValue(client),
            authorizationHeaderProvider.overrideWith(
              (ref) => _authorization('profile'),
            ),
          ],
          child: MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: E2eUndecryptableMessagePlaceholder()),
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('e2e_backup_restore_cta')));
      await tester.pumpAndSettle();
      expect(find.byKey(E2eKeyBackupScreen.screenKey), findsOneWidget);
    });
  });

  group('E2E key backup client restore integrity', () {
    const password = 'correct-horse';
    final codec = E2eKeyBackupCodecV2();

    test(
      'rejects a backup for another profile before importing or uploading',
      () async {
        final encryptedBlob = await codec.encryptPayload(
          password: password,
          payload: await _validPayload('other-profile'),
        );
        var imports = 0;
        final requests = <String>[];
        final client = await _client(
          encryptedBlob: encryptedBlob,
          handler: (request) async {
            requests.add('${request.method} ${request.url.path}');
            return http.Response(
              jsonEncode({'encrypted_blob': encryptedBlob}),
              200,
            );
          },
          importBackup: (_, _) async => imports++,
        );

        final result = await client.restoreKeyBackup(
          authorization: _authorization('current-profile'),
          password: password,
          isAuthorizationCurrent: () => true,
        );

        expect(result, isA<E2eApiFailure>());
        expect(imports, 0);
        expect(requests, ['GET /api/v1/auth/e2e-key-backup']);
      },
    );

    test(
      'does not import if the signed-in session changes during fetch',
      () async {
        final encryptedBlob = await codec.encryptPayload(
          password: password,
          payload: await _validPayload('current-profile'),
        );
        final requestStarted = Completer<void>();
        final response = Completer<http.Response>();
        var current = true;
        var imports = 0;
        final client = await _client(
          encryptedBlob: encryptedBlob,
          handler: (_) {
            requestStarted.complete();
            return response.future;
          },
          importBackup: (_, _) async => imports++,
        );

        final restore = client.restoreKeyBackup(
          authorization: _authorization('current-profile'),
          password: password,
          isAuthorizationCurrent: () => current,
        );
        await requestStarted.future;
        current = false;
        response.complete(
          http.Response(jsonEncode({'encrypted_blob': encryptedBlob}), 200),
        );

        final result = await restore;
        expect(result, isA<E2eApiFailure>());
        expect(imports, 0);
      },
    );

    test(
      'session change during import never uploads a pre-key for the next profile',
      () async {
        final encryptedBlob = await codec.encryptPayload(
          password: password,
          payload: await _validPayload('profile-before-switch'),
        );
        final importing = Completer<void>();
        final finishImport = Completer<void>();
        final requests = <String>[];
        var authorizationCurrent = true;
        String? importedProfile;
        final client = await _client(
          encryptedBlob: encryptedBlob,
          handler: (request) async {
            requests.add('${request.method} ${request.url.path}');
            return http.Response(
              jsonEncode({'encrypted_blob': encryptedBlob}),
              200,
            );
          },
          importBackup: (profileId, _) async {
            importedProfile = profileId;
            importing.complete();
            await finishImport.future;
          },
        );

        final restore = client.restoreKeyBackup(
          authorization: _authorization('profile-before-switch'),
          password: password,
          isAuthorizationCurrent: () => authorizationCurrent,
        );
        await importing.future;
        authorizationCurrent = false;
        finishImport.complete();

        final result = await restore;
        expect(result, isA<E2eApiFailure>());
        expect(importedProfile, 'profile-before-switch');
        expect(requests, ['GET /api/v1/auth/e2e-key-backup']);
      },
    );

    test('wrong password leaves the profile importer untouched', () async {
      final encryptedBlob = await codec.encryptPayload(
        password: password,
        payload: await _validPayload('current-profile'),
      );
      var imports = 0;
      final client = await _client(
        encryptedBlob: encryptedBlob,
        importBackup: (_, _) async => imports++,
      );

      final result = await client.restoreKeyBackup(
        authorization: _authorization('current-profile'),
        password: 'incorrect-password',
        isAuthorizationCurrent: () => true,
      );

      expect(result, isA<E2eApiFailure>());
      expect(imports, 0);
    });

    test(
      'malformed encrypted payload leaves import and pre-key upload untouched',
      () async {
        var imports = 0;
        final requests = <String>[];
        final client = await _client(
          encryptedBlob: 'not-a-valid-backup',
          handler: (request) async {
            requests.add('${request.method} ${request.url.path}');
            return http.Response(
              jsonEncode({'encrypted_blob': 'not-a-valid-backup'}),
              200,
            );
          },
          importBackup: (_, _) async => imports++,
        );

        final result = await client.restoreKeyBackup(
          authorization: _authorization('current-profile'),
          password: password,
          isAuthorizationCurrent: () => true,
        );

        expect(result, isA<E2eApiFailure>());
        expect(imports, 0);
        expect(requests, ['GET /api/v1/auth/e2e-key-backup']);
      },
    );

    test(
      'decryptable invalid Signal state cannot replace local keys or upload',
      () async {
        final encryptedBlob = await codec.encryptPayload(
          password: password,
          payload: const {
            'profile_id': 'current-profile',
            'version': 2,
            'signal_state': <String, dynamic>{},
          },
        );
        final storage = InMemorySecureSignalStorage();
        final existingStore = await SecureSignalStore.open(
          profileId: 'current-profile',
          storage: storage,
        );
        await existingStore.close();
        final before = await storage.read(key: 'state_v1');
        final requests = <String>[];
        final client = await _client(
          encryptedBlob: encryptedBlob,
          backupStorage: storage,
          handler: (request) async {
            requests.add('${request.method} ${request.url.path}');
            return http.Response(
              jsonEncode({'encrypted_blob': encryptedBlob}),
              200,
            );
          },
        );

        final result = await client.restoreKeyBackup(
          authorization: _authorization('current-profile'),
          password: password,
          isAuthorizationCurrent: () => true,
        );

        expect(result, isA<E2eApiFailure>());
        expect(requests, ['GET /api/v1/auth/e2e-key-backup']);
        expect(await storage.read(key: 'state_v1'), before);
        await expectLater(
          SecureSignalStore.importFromBackup(
            'current-profile',
            const <String, dynamic>{},
            storage: storage,
          ),
          throwsA(isA<Exception>()),
        );
        expect(await storage.read(key: 'state_v1'), before);
      },
    );

    test('sends server delete without local import or key mutation', () async {
      final requests = <http.Request>[];
      final client = await _client(
        encryptedBlob: '',
        handler: (request) async {
          requests.add(request);
          return http.Response('', 204);
        },
        importBackup: (_, _) async => fail('delete must not import'),
      );

      final result = await client.deleteKeyBackup(
        authorization: _authorization('current-profile'),
      );

      expect(result, isA<E2eApiOk<void>>());
      expect(requests, hasLength(1));
      expect(requests.single.method, 'DELETE');
      expect(requests.single.url.path, '/api/v1/auth/e2e-key-backup');
    });
  });
}

Future<void> _loadCaptureFonts() async {
  final materialIcons = FontLoader('MaterialIcons')
    ..addFont(rootBundle.load('fonts/MaterialIcons-Regular.otf'));
  await materialIcons.load();
  final notoSans = FontLoader(VoiceTheme.fontFamily)
    ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
    ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
    ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
    ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf'));
  await notoSans.load();
}
