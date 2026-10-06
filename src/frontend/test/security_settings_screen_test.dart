import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
// ignore: depend_on_referenced_packages
import 'package:path_provider_platform_interface/path_provider_platform_interface.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/settings/e2e_key_backup_screen.dart';
import 'package:voice_frontend/ui/settings/security_settings_screen.dart';
import 'package:voice_frontend/ui/settings/settings_sheet.dart';

import 'support/test_voice_token_catalog.dart';

const _securityCaptureRootKey = Key('security_capture_root');
var _securityCaptureFontsLoaded = false;

class _MemoryAuthStorage
    implements AuthSessionStorage, ConditionalAuthSessionStorage {
  AuthSession? session;

  @override
  Future<void> clear() async {
    session = null;
  }

  @override
  Future<AuthSession?> read() async => session;

  @override
  Future<void> write(AuthSession value) async {
    session = value;
  }

  @override
  Future<bool> clearIfUnchanged(AuthSession expected) async {
    if (session != expected) return false;
    session = null;
    return true;
  }
}

Future<ThemeData?> _securityCaptureTheme(WidgetTester tester) async {
  if (!_securityCaptureFontsLoaded) {
    final loaded = await tester.runAsync(() async {
      final fonts = FontLoader(VoiceTheme.fontFamily)
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf'));
      await fonts.load();
      final flutterRoot = Platform.environment['FLUTTER_ROOT'];
      if (flutterRoot == null || flutterRoot.isEmpty) {
        throw StateError('FLUTTER_ROOT is required to load Material Icons');
      }
      final iconFont = File(
        [
          flutterRoot,
          'bin',
          'cache',
          'artifacts',
          'material_fonts',
          'MaterialIcons-Regular.otf',
        ].join(Platform.pathSeparator),
      );
      final bytes = await iconFont.readAsBytes();
      final iconData = ByteData.sublistView(Uint8List.fromList(bytes));
      await (FontLoader(
        'MaterialIcons',
      )..addFont(Future<ByteData>.value(iconData))).load();
      return true;
    });
    if (loaded != true) {
      throw StateError('Production font loading did not complete');
    }
    _securityCaptureFontsLoaded = true;
  }
  final catalog = await tester.runAsync(VoiceTokenCatalog.load);
  if (catalog == null) {
    throw StateError('Voice design tokens did not load for capture');
  }
  return VoiceTheme.build(
    catalog: catalog,
    mode: VoiceThemeMode.dark,
    profileAccent: catalog.profileAccentAt(0),
  );
}

Future<void> _captureSecurityState(WidgetTester tester, String filename) async {
  final directory = Platform.environment['VOICE_SECURITY_CAPTURE_DIR'];
  if (directory == null || directory.isEmpty) return;
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_securityCaptureRootKey),
  );
  final bytes = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      final data = await image.toByteData(format: ui.ImageByteFormat.png);
      if (data == null) throw StateError('PNG encoding returned null');
      final output = File('$directory${Platform.pathSeparator}$filename');
      await output.parent.create(recursive: true);
      await output.writeAsBytes(data.buffer.asUint8List(), flush: true);
      return data.buffer.asUint8List();
    } finally {
      image.dispose();
    }
  });
  if (bytes == null || bytes.isEmpty) {
    throw StateError('Security screen capture did not complete');
  }
}

class _SettingsSecurityEntryHost extends ConsumerWidget {
  const _SettingsSecurityEntryHost();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    ref.watch(authControllerProvider);
    return Scaffold(
      body: Center(
        child: TextButton(
          onPressed: () => showModalBottomSheet<void>(
            context: context,
            isScrollControlled: true,
            builder: (_) => const SettingsSheet(),
          ),
          child: const Text('Open settings'),
        ),
      ),
    );
  }
}

class _TestPathProviderPlatform extends PathProviderPlatform {
  _TestPathProviderPlatform(this.directory);

  final String directory;

  @override
  Future<String?> getDownloadsPath() async => directory;

  @override
  Future<String?> getApplicationDocumentsPath() async => directory;
}

Future<AuthController> _pumpSecuritySettings(
  WidgetTester tester,
  MockClient client, {
  ThemeData? theme,
  Widget? home,
  AuthSession? session = const AuthSession(
    accessToken: 'token',
    refreshToken: 'refresh',
    expiresInSeconds: 900,
    accountId: 'account-1',
    activeProfileId: 'profile-primary',
  ),
}) async {
  late AuthController controller;
  final gateway = GatewayHttpClient(
    httpClient: client,
    config: const GatewayConfig(baseUrl: 'http://api.test'),
    authorizationProvider: () => 'Bearer token',
  );
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        ...voiceThemeTestOverrides(),
        authSessionStorageProvider.overrideWithValue(_MemoryAuthStorage()),
        guestCredentialsStorageProvider.overrideWithValue(
          InMemoryGuestCredentialsStorage(),
        ),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        gatewayHttpClientProvider.overrideWithValue(gateway),
        voiceAuthClientProvider.overrideWithValue(
          VoiceAuthClient(gateway: gateway),
        ),
        authControllerProvider.overrideWith((ref) {
          controller = AuthController(
            authClient: ref.watch(voiceAuthClientProvider),
            storage: ref.watch(authSessionStorageProvider),
            guestCredentialsStorage: ref.watch(guestCredentialsStorageProvider),
          );
          controller.state = AuthState(session: session);
          if (session != null) {
            unawaited(ref.read(authSessionStorageProvider).write(session));
          }
          return controller;
        }),
      ],
      child: RepaintBoundary(
        key: _securityCaptureRootKey,
        child: MaterialApp(
          debugShowCheckedModeBanner: false,
          theme: theme,
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: home ?? const SecuritySettingsScreen(),
        ),
      ),
    ),
  );
  await tester.pump();
  return controller;
}

void main() {
  testWidgets(
    'Settings security entry exposes the documented password-change action',
    (tester) async {
      final client = MockClient((request) async {
        if (request.method == 'GET' &&
            request.url.path == '/api/v1/auth/2fa/status') {
          return http.Response(jsonEncode({'enabled': false}), 200);
        }
        return http.Response('not found', 404);
      });

      await _pumpSecuritySettings(
        tester,
        client,
        home: const _SettingsSecurityEntryHost(),
      );
      await tester.tap(find.text('Open settings'));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('settings_security')));
      await tester.pumpAndSettle();

      expect(find.text('Change password'), findsOneWidget);
    },
  );

  testWidgets(
    'unknown 2FA status is unavailable until retry confirms disabled',
    (tester) async {
      final pendingStatus = Completer<http.Response>();
      var statusCalls = 0;
      final client = MockClient((request) async {
        if (request.method == 'GET' &&
            request.url.path == '/api/v1/auth/2fa/status') {
          statusCalls++;
          if (statusCalls == 1) return pendingStatus.future;
          return http.Response(jsonEncode({'enabled': false}), 200);
        }
        return http.Response('not found', 404);
      });

      final controller = await _pumpSecuritySettings(tester, client);
      expect(statusCalls, 1);
      expect(find.byKey(SecuritySettingsScreen.enableButtonKey), findsNothing);

      pendingStatus.complete(http.Response('unavailable', 503));
      await tester.pumpAndSettle();
      expect(
        find.byKey(const Key('security_2fa_status_unavailable')),
        findsOneWidget,
      );
      expect(find.byKey(SecuritySettingsScreen.enableButtonKey), findsNothing);

      await tester.tap(find.byKey(const Key('security_2fa_status_retry')));
      await tester.pumpAndSettle();
      expect(statusCalls, 2);
      expect(
        find.byKey(SecuritySettingsScreen.enableButtonKey),
        findsOneWidget,
      );

      controller.state = controller.state.copyWith(clearSession: true);
      await tester.pumpAndSettle();
      expect(
        find.byKey(SecuritySettingsScreen.statusUnavailableKey),
        findsOneWidget,
      );
      expect(find.byKey(SecuritySettingsScreen.enableButtonKey), findsNothing);
    },
  );

  testWidgets('manual authenticator key comes from the provisioning URI', (
    tester,
  ) async {
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/auth/2fa/status') {
        return http.Response(jsonEncode({'enabled': false}), 200);
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/2fa/enable') {
        return http.Response(
          jsonEncode({
            'totp_uri':
                'otpauth://totp/Voice:test?secret=DUMMY-MANUAL-KEY&issuer=Voice',
            'secret_backup_hint': 'NOT-THE-SETUP-KEY',
            'backup_codes': ['DUMMY-BACKUP-ONE'],
          }),
          200,
        );
      }
      return http.Response('not found', 404);
    });

    await _pumpSecuritySettings(tester, client);
    await tester.enterText(
      find.byKey(SecuritySettingsScreen.passwordFieldKey),
      'test-password',
    );
    await tester.tap(find.byKey(SecuritySettingsScreen.enableButtonKey));
    await tester.pumpAndSettle();

    expect(find.byKey(SecuritySettingsScreen.qrKey), findsOneWidget);
    expect(find.text('DUMMY-MANUAL-KEY'), findsOneWidget);
    expect(find.text('NOT-THE-SETUP-KEY'), findsNothing);
  });

  testWidgets('backup codes stay hidden until the server verifies enrollment', (
    tester,
  ) async {
    var verifyCalls = 0;
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/auth/2fa/status') {
        return http.Response(jsonEncode({'enabled': false}), 200);
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/2fa/enable') {
        return http.Response(
          jsonEncode({
            'totp_uri':
                'otpauth://totp/Voice:test?secret=DUMMY-SETUP-KEY&issuer=Voice',
            'secret_backup_hint': 'DUMMY-HINT',
            'backup_codes': ['DUMMY-BACKUP-ONE', 'DUMMY-BACKUP-TWO'],
          }),
          200,
        );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/2fa/verify') {
        verifyCalls++;
        if (verifyCalls == 1) {
          return http.Response(jsonEncode({'error': 'invalid_totp'}), 401);
        }
        return http.Response(
          jsonEncode({
            'access_token': 'new-token',
            'refresh_token': 'new-refresh',
            'account_id': 'account-1',
            'profile_id': 'profile-primary',
            'expires_in_seconds': 900,
          }),
          200,
        );
      }
      return http.Response('not found', 404);
    });

    final controller = await _pumpSecuritySettings(tester, client);
    await tester.enterText(
      find.byKey(SecuritySettingsScreen.passwordFieldKey),
      'test-password',
    );
    await tester.tap(find.byKey(SecuritySettingsScreen.enableButtonKey));
    await tester.pumpAndSettle();
    expect(find.text('DUMMY-BACKUP-ONE'), findsNothing);

    await tester.tap(find.text('Continue'));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(SecuritySettingsScreen.totpFieldKey),
      '111111',
    );
    await tester.tap(find.byKey(SecuritySettingsScreen.verifyButtonKey));
    await tester.pumpAndSettle();
    expect(verifyCalls, 1);
    expect(find.text('DUMMY-BACKUP-ONE'), findsNothing);
    expect(controller.state.session?.accessToken, 'token');

    await tester.enterText(
      find.byKey(SecuritySettingsScreen.totpFieldKey),
      '222222',
    );
    await tester.tap(find.byKey(SecuritySettingsScreen.verifyButtonKey));
    await tester.pumpAndSettle();
    expect(verifyCalls, 2);
    expect(controller.state.session?.accessToken, 'new-token');
    expect(find.text('DUMMY-BACKUP-ONE'), findsOneWidget);
    expect(find.text('DUMMY-BACKUP-TWO'), findsOneWidget);
  });

  testWidgets('malformed provisioning data never exposes the hint as a key', (
    tester,
  ) async {
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/auth/2fa/status') {
        return http.Response(jsonEncode({'enabled': false}), 200);
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/2fa/enable') {
        return http.Response(
          jsonEncode({
            'totp_uri': 'otpauth://totp/Voice:test?issuer=Voice',
            'secret_backup_hint': 'DUMMY-NON-SECRET-HINT',
            'backup_codes': ['DUMMY-BACKUP'],
          }),
          200,
        );
      }
      return http.Response('not found', 404);
    });

    await _pumpSecuritySettings(tester, client);
    await tester.enterText(
      find.byKey(SecuritySettingsScreen.passwordFieldKey),
      'test-password',
    );
    await tester.tap(find.byKey(SecuritySettingsScreen.enableButtonKey));
    await tester.pumpAndSettle();

    expect(find.byKey(SecuritySettingsScreen.qrKey), findsNothing);
    expect(find.text('DUMMY-NON-SECRET-HINT'), findsNothing);
    expect(find.text('Could not complete this action.'), findsOneWidget);
  });

  testWidgets('ambiguous provisioning secret fails closed', (tester) async {
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/auth/2fa/status') {
        return http.Response(jsonEncode({'enabled': false}), 200);
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/2fa/enable') {
        return http.Response(
          jsonEncode({
            'totp_uri':
                'otpauth://totp/Voice:test?secret=DUMMY-AMBIGUOUS-ONE&secret=DUMMY-AMBIGUOUS-TWO&issuer=Voice',
            'secret_backup_hint': 'DUMMY-AMBIGUOUS-HINT',
            'backup_codes': ['DUMMY-AMBIGUOUS-CODE'],
          }),
          200,
        );
      }
      return http.Response('not found', 404);
    });

    await _pumpSecuritySettings(tester, client);
    await tester.enterText(
      find.byKey(SecuritySettingsScreen.passwordFieldKey),
      'test-password',
    );
    await tester.tap(find.byKey(SecuritySettingsScreen.enableButtonKey));
    await tester.pumpAndSettle();

    expect(find.byKey(SecuritySettingsScreen.qrKey), findsNothing);
    expect(find.text('DUMMY-AMBIGUOUS-ONE'), findsNothing);
    expect(find.text('DUMMY-AMBIGUOUS-TWO'), findsNothing);
    expect(find.text('DUMMY-AMBIGUOUS-HINT'), findsNothing);
    expect(find.text('DUMMY-AMBIGUOUS-CODE'), findsNothing);
    expect(find.text('Could not complete this action.'), findsOneWidget);
  });

  testWidgets('late enrollment response is discarded after account changes', (
    tester,
  ) async {
    final pendingEnrollment = Completer<http.Response>();
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/auth/2fa/status') {
        return http.Response(jsonEncode({'enabled': false}), 200);
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/2fa/enable') {
        return pendingEnrollment.future;
      }
      return http.Response('not found', 404);
    });

    final controller = await _pumpSecuritySettings(tester, client);
    await tester.enterText(
      find.byKey(SecuritySettingsScreen.passwordFieldKey),
      'test-password',
    );
    await tester.tap(find.byKey(SecuritySettingsScreen.enableButtonKey));
    await tester.pump();
    controller.state = const AuthState(
      session: AuthSession(
        accessToken: 'other-token',
        refreshToken: 'other-refresh',
        expiresInSeconds: 900,
        accountId: 'account-2',
        activeProfileId: 'profile-primary',
      ),
    );
    await tester.pumpAndSettle();

    pendingEnrollment.complete(
      http.Response(
        jsonEncode({
          'totp_uri':
              'otpauth://totp/Voice:old?secret=DUMMY-STALE-KEY&issuer=Voice',
          'secret_backup_hint': 'DUMMY-STALE-HINT',
          'backup_codes': ['DUMMY-STALE-CODE'],
        }),
        200,
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byKey(SecuritySettingsScreen.qrKey), findsNothing);
    expect(find.text('DUMMY-STALE-KEY'), findsNothing);
    expect(find.text('DUMMY-STALE-HINT'), findsNothing);
    expect(find.text('DUMMY-STALE-CODE'), findsNothing);
  });

  testWidgets('late enrollment response after disposal does not update UI', (
    tester,
  ) async {
    final pendingEnrollment = Completer<http.Response>();
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/auth/2fa/status') {
        return http.Response(jsonEncode({'enabled': false}), 200);
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/2fa/enable') {
        return pendingEnrollment.future;
      }
      return http.Response('not found', 404);
    });

    await _pumpSecuritySettings(tester, client);
    await tester.enterText(
      find.byKey(SecuritySettingsScreen.passwordFieldKey),
      'test-password',
    );
    await tester.tap(find.byKey(SecuritySettingsScreen.enableButtonKey));
    await tester.pump();
    await tester.pumpWidget(const SizedBox.shrink());
    pendingEnrollment.complete(
      http.Response(
        jsonEncode({
          'totp_uri':
              'otpauth://totp/Voice:disposed?secret=DUMMY-DISPOSED-KEY&issuer=Voice',
          'secret_backup_hint': 'DUMMY-DISPOSED-HINT',
          'backup_codes': ['DUMMY-DISPOSED-CODE'],
        }),
        200,
      ),
    );
    await tester.pump();
    expect(tester.takeException(), isNull);
  });

  testWidgets('verified backup codes can be copied and explicitly downloaded', (
    tester,
  ) async {
    final tempDirectory = await tester.runAsync(
      () => Directory.systemTemp.createTemp('voice-security-2fa-test-'),
    );
    final downloadDirectory = tempDirectory!;
    addTearDown(
      () => tester.runAsync(() => downloadDirectory.delete(recursive: true)),
    );
    final messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    final copiedValues = <String>[];
    final previousPathProvider = PathProviderPlatform.instance;
    PathProviderPlatform.instance = _TestPathProviderPlatform(
      downloadDirectory.path,
    );
    messenger.setMockMethodCallHandler(SystemChannels.platform, (call) async {
      if (call.method == 'Clipboard.setData') {
        final payload = call.arguments as Map<Object?, Object?>;
        copiedValues.add(payload['text'] as String);
      }
      return null;
    });
    addTearDown(() {
      PathProviderPlatform.instance = previousPathProvider;
      messenger.setMockMethodCallHandler(SystemChannels.platform, null);
    });

    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/auth/2fa/status') {
        return http.Response(jsonEncode({'enabled': false}), 200);
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/2fa/enable') {
        return http.Response(
          jsonEncode({
            'totp_uri':
                'otpauth://totp/Voice:test?secret=DUMMY-SETUP-KEY&issuer=Voice',
            'secret_backup_hint': 'DUMMY-HINT',
            'backup_codes': ['DUMMY-BACKUP-ONE', 'DUMMY-BACKUP-TWO'],
          }),
          200,
        );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/2fa/verify') {
        return http.Response(
          jsonEncode({
            'access_token': 'new-token',
            'refresh_token': 'new-refresh',
            'account_id': 'account-1',
            'profile_id': 'profile-primary',
            'expires_in_seconds': 900,
          }),
          200,
        );
      }
      return http.Response('not found', 404);
    });

    await _pumpSecuritySettings(tester, client);
    await tester.enterText(
      find.byKey(SecuritySettingsScreen.passwordFieldKey),
      'test-password',
    );
    await tester.tap(find.byKey(SecuritySettingsScreen.enableButtonKey));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Continue'));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(SecuritySettingsScreen.totpFieldKey),
      '333333',
    );
    await tester.tap(find.byKey(SecuritySettingsScreen.verifyButtonKey));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(SecuritySettingsScreen.copyBackupCodesKey));
    await tester.pump();
    expect(copiedValues, ['DUMMY-BACKUP-ONE\nDUMMY-BACKUP-TWO']);

    await tester.tap(find.byKey(SecuritySettingsScreen.downloadBackupCodesKey));
    await tester.pumpAndSettle();
    await tester.runAsync(
      () => Future<void>.delayed(const Duration(milliseconds: 20)),
    );
    await tester.pump();
    final export = File(
      '${downloadDirectory.path}${Platform.pathSeparator}voice-2fa-backup-codes.txt',
    );
    final exportContents = await tester.runAsync(() async {
      for (var attempt = 0; attempt < 100; attempt++) {
        if (await export.exists()) {
          final contents = await export.readAsString();
          if (contents.isNotEmpty) return contents;
        }
        await Future<void>.delayed(const Duration(milliseconds: 10));
      }
      return null;
    });
    expect(exportContents, 'DUMMY-BACKUP-ONE\nDUMMY-BACKUP-TWO');
  });

  testWidgets(
    '2FA setup and recovery states render in desktop and narrow layouts',
    (tester) async {
      final captureDirectory =
          Platform.environment['VOICE_SECURITY_CAPTURE_DIR'];
      final theme = captureDirectory == null || captureDirectory.isEmpty
          ? null
          : await _securityCaptureTheme(tester);
      for (final (name, size) in <(String, Size)>[
        ('desktop', const Size(1280, 800)),
        ('narrow', const Size(390, 844)),
      ]) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        tester.binding.handleMetricsChanged();
        final verifyCalls = <int>[];
        final client = MockClient((request) async {
          if (request.method == 'GET' &&
              request.url.path == '/api/v1/auth/2fa/status') {
            return http.Response(jsonEncode({'enabled': false}), 200);
          }
          if (request.method == 'POST' &&
              request.url.path == '/api/v1/auth/2fa/enable') {
            return http.Response(
              jsonEncode({
                'totp_uri':
                    'otpauth://totp/Voice:test?secret=DUMMY-CAPTURE-KEY&issuer=Voice',
                'secret_backup_hint': 'DUMMY-CAPTURE-HINT',
                'backup_codes': [
                  'DUMMY-CAPTURE-CODE-ONE',
                  'DUMMY-CAPTURE-CODE-TWO',
                ],
              }),
              200,
            );
          }
          if (request.method == 'POST' &&
              request.url.path == '/api/v1/auth/2fa/verify') {
            verifyCalls.add(verifyCalls.length + 1);
            if (verifyCalls.length == 1) {
              return http.Response(jsonEncode({'error': 'invalid_totp'}), 401);
            }
            return http.Response(
              jsonEncode({
                'access_token': 'capture-token',
                'refresh_token': 'capture-refresh',
                'account_id': 'account-1',
                'profile_id': 'profile-primary',
                'expires_in_seconds': 900,
              }),
              200,
            );
          }
          return http.Response('not found', 404);
        });

        await _pumpSecuritySettings(tester, client, theme: theme);
        await tester.enterText(
          find.byKey(SecuritySettingsScreen.passwordFieldKey),
          'test-password',
        );
        await tester.tap(find.byKey(SecuritySettingsScreen.enableButtonKey));
        await tester.pumpAndSettle();
        expect(find.byKey(SecuritySettingsScreen.qrKey), findsOneWidget);
        expect(find.text('DUMMY-CAPTURE-KEY'), findsOneWidget);
        expect(find.text('DUMMY-CAPTURE-HINT'), findsNothing);
        await _captureSecurityState(tester, '$name-setup.png');

        await tester.tap(find.text('Continue'));
        await tester.pumpAndSettle();
        await tester.enterText(
          find.byKey(SecuritySettingsScreen.totpFieldKey),
          '111111',
        );
        await tester.tap(find.byKey(SecuritySettingsScreen.verifyButtonKey));
        await tester.pumpAndSettle();
        expect(find.text('DUMMY-CAPTURE-CODE-ONE'), findsNothing);
        expect(find.byKey(SecuritySettingsScreen.totpFieldKey), findsOneWidget);
        await _captureSecurityState(tester, '$name-verify-error.png');

        await tester.enterText(
          find.byKey(SecuritySettingsScreen.totpFieldKey),
          '222222',
        );
        await tester.tap(find.byKey(SecuritySettingsScreen.verifyButtonKey));
        await tester.pumpAndSettle();
        expect(find.text('DUMMY-CAPTURE-CODE-ONE'), findsOneWidget);
        expect(find.text('DUMMY-CAPTURE-CODE-TWO'), findsOneWidget);
        await tester.pump(const Duration(seconds: 5));
        tester
            .state<ScaffoldMessengerState>(find.byType(ScaffoldMessenger))
            .hideCurrentSnackBar();
        await tester.pumpAndSettle();
        await _captureSecurityState(tester, '$name-verified-codes.png');

        await tester.pumpWidget(const SizedBox.shrink());
        await tester.pump();
      }
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(() => tester.binding.setSurfaceSize(null));
    },
  );

  testWidgets('failed disable retains session and success clears it', (
    tester,
  ) async {
    var disableCalls = 0;
    var logoutCalls = 0;
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/auth/2fa/status') {
        return http.Response(jsonEncode({'enabled': true}), 200);
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/2fa/disable') {
        disableCalls++;
        final body = jsonDecode(request.body) as Map<String, dynamic>;
        expect(body['password'], 'test-password');
        expect(body['totp_code'], disableCalls == 1 ? '111111' : '222222');
        if (disableCalls == 1) {
          return http.Response(jsonEncode({'error': 'invalid_totp'}), 401);
        }
        return http.Response('', 204);
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/logout') {
        logoutCalls++;
        return http.Response('', 204);
      }
      return http.Response('not found', 404);
    });

    final controller = await _pumpSecuritySettings(tester, client);
    await tester.enterText(
      find.byKey(SecuritySettingsScreen.passwordFieldKey),
      'test-password',
    );
    await tester.enterText(
      find.byKey(SecuritySettingsScreen.totpFieldKey),
      '111111',
    );
    await tester.tap(find.byKey(SecuritySettingsScreen.disableButtonKey));
    await tester.pumpAndSettle();
    expect(disableCalls, 1);
    expect(controller.state.session?.accountId, 'account-1');
    expect(find.byKey(SecuritySettingsScreen.disableButtonKey), findsOneWidget);

    await tester.enterText(
      find.byKey(SecuritySettingsScreen.totpFieldKey),
      '222222',
    );
    await tester.tap(find.byKey(SecuritySettingsScreen.disableButtonKey));
    await tester.pumpAndSettle();
    expect(disableCalls, 2);
    expect(logoutCalls, 1);
    expect(controller.state.session, isNull);
  });

  testWidgets(
    'security settings opens the backup screen and loads status with current session',
    (tester) async {
      final requests = <http.Request>[];
      final mock = MockClient((request) async {
        requests.add(request);
        if (request.method == 'GET' &&
            request.url.path == '/api/v1/auth/2fa/status') {
          return http.Response(jsonEncode({'enabled': false}), 200);
        }
        if (request.method == 'GET' &&
            request.url.path == '/api/v1/auth/e2e-key-backup') {
          return http.Response(jsonEncode({'error': 'not found'}), 404);
        }
        return http.Response('not found', 404);
      });

      final gateway = GatewayHttpClient(
        httpClient: mock,
        config: const GatewayConfig(baseUrl: 'http://api.test'),
        authorizationProvider: () => 'Bearer token',
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            ...voiceThemeTestOverrides(),
            authSessionStorageProvider.overrideWithValue(_MemoryAuthStorage()),
            guestCredentialsStorageProvider.overrideWithValue(
              InMemoryGuestCredentialsStorage(),
            ),
            gatewayConfigProvider.overrideWithValue(
              const GatewayConfig(baseUrl: 'http://api.test'),
            ),
            gatewayHttpClientProvider.overrideWithValue(gateway),
            voiceAuthClientProvider.overrideWithValue(
              VoiceAuthClient(gateway: gateway),
            ),
            authControllerProvider.overrideWith((ref) {
              final controller = AuthController(
                authClient: ref.watch(voiceAuthClientProvider),
                storage: ref.watch(authSessionStorageProvider),
                guestCredentialsStorage: ref.watch(
                  guestCredentialsStorageProvider,
                ),
              );
              controller.state = const AuthState(
                session: AuthSession(
                  accessToken: 'token',
                  refreshToken: 'refresh',
                  expiresInSeconds: 900,
                  accountId: 'account-1',
                  activeProfileId: 'profile-primary',
                ),
              );
              return controller;
            }),
          ],
          child: MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const SecuritySettingsScreen(),
          ),
        ),
      );
      await tester.pumpAndSettle();

      await tester.scrollUntilVisible(
        find.byKey(const Key('security_e2e_key_backup')),
        120,
        scrollable: find
            .descendant(
              of: find.byKey(SecuritySettingsScreen.screenKey),
              matching: find.byType(Scrollable),
            )
            .first,
      );
      await tester.tap(find.byKey(const Key('security_e2e_key_backup')));
      await tester.pumpAndSettle();

      expect(find.byKey(E2eKeyBackupScreen.screenKey), findsOneWidget);
      expect(
        find.text('No encrypted key backup is saved for this account.'),
        findsOneWidget,
      );
      final backupRequest = requests.singleWhere(
        (request) => request.url.path == '/api/v1/auth/e2e-key-backup',
      );
      expect(backupRequest.method, 'GET');
      expect(backupRequest.headers['authorization'], 'Bearer token');
      expect(
        requests.where(
          (request) => request.url.path == '/api/v1/auth/e2e-key-backup',
        ),
        hasLength(1),
      );
    },
  );

  testWidgets('security settings deletes account after password confirm', (
    tester,
  ) async {
    var deleteCalled = false;
    final mock = MockClient((req) async {
      if (req.method == 'POST' &&
          req.url.path == '/api/v1/auth/delete-account') {
        deleteCalled = true;
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        expect(body['password'], 'secret');
        return http.Response('', 204);
      }
      if (req.method == 'POST' && req.url.path == '/api/v1/auth/logout') {
        return http.Response('', 204);
      }
      return http.Response('not found', 404);
    });

    final gateway = GatewayHttpClient(
      httpClient: mock,
      config: const GatewayConfig(baseUrl: 'http://api.test'),
      authorizationProvider: () => 'Bearer token',
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          authSessionStorageProvider.overrideWithValue(_MemoryAuthStorage()),
          guestCredentialsStorageProvider.overrideWithValue(
            InMemoryGuestCredentialsStorage(),
          ),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          gatewayHttpClientProvider.overrideWithValue(gateway),
          voiceAuthClientProvider.overrideWithValue(
            VoiceAuthClient(gateway: gateway),
          ),
          authControllerProvider.overrideWith((ref) {
            final controller = AuthController(
              authClient: ref.watch(voiceAuthClientProvider),
              storage: ref.watch(authSessionStorageProvider),
              guestCredentialsStorage: ref.watch(
                guestCredentialsStorageProvider,
              ),
            );
            controller.state = const AuthState(
              session: AuthSession(
                accessToken: 'token',
                refreshToken: 'refresh',
                expiresInSeconds: 900,
                accountId: 'account-1',
                activeProfileId: 'profile-primary',
              ),
            );
            return controller;
          }),
        ],
        child: MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const SecuritySettingsScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.scrollUntilVisible(
      find.byKey(SecuritySettingsScreen.deleteAccountButtonKey),
      120,
      scrollable: find
          .descendant(
            of: find.byKey(SecuritySettingsScreen.screenKey),
            matching: find.byType(Scrollable),
          )
          .first,
    );
    await tester.tap(find.byKey(SecuritySettingsScreen.deleteAccountButtonKey));
    await tester.pumpAndSettle();

    expect(
      find.byKey(SecuritySettingsScreen.deleteAccountDialogKey),
      findsOneWidget,
    );
    await tester.enterText(
      find.byKey(SecuritySettingsScreen.deleteAccountPasswordKey),
      'secret',
    );
    await tester.tap(
      find.descendant(
        of: find.byKey(SecuritySettingsScreen.deleteAccountDialogKey),
        matching: find.text('Delete'),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 500));

    expect(deleteCalled, isTrue);
  });

  // T056 RED seam: deleteAccountTotpKey is intentionally introduced by the
  // minimal production dialog change after these accepted tests.
  testWidgets(
    'totp_required keeps deletion pending and retries with authenticator code',
    (tester) async {
      var deleteAttempts = 0;
      var logoutCalled = false;
      late AuthController controller;
      final mock = MockClient((req) async {
        if (req.method == 'POST' &&
            req.url.path == '/api/v1/auth/delete-account') {
          deleteAttempts++;
          final body = jsonDecode(req.body) as Map<String, dynamic>;
          expect(body['password'], 'secret');
          if (deleteAttempts == 1) {
            expect(body.containsKey('totp_code'), isFalse);
            return http.Response(jsonEncode({'error': 'totp_required'}), 401);
          }
          if (deleteAttempts == 2) {
            expect(body['totp_code'], 'invalid-code');
            return http.Response(jsonEncode({'error': 'invalid_totp'}), 401);
          }
          expect(body['totp_code'], '654321');
          return http.Response('', 204);
        }
        if (req.method == 'POST' && req.url.path == '/api/v1/auth/logout') {
          logoutCalled = true;
          return http.Response('', 204);
        }
        return http.Response('not found', 404);
      });

      final gateway = GatewayHttpClient(
        httpClient: mock,
        config: const GatewayConfig(baseUrl: 'http://api.test'),
        authorizationProvider: () => 'Bearer token',
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            ...voiceThemeTestOverrides(),
            authSessionStorageProvider.overrideWithValue(_MemoryAuthStorage()),
            guestCredentialsStorageProvider.overrideWithValue(
              InMemoryGuestCredentialsStorage(),
            ),
            gatewayConfigProvider.overrideWithValue(
              const GatewayConfig(baseUrl: 'http://api.test'),
            ),
            gatewayHttpClientProvider.overrideWithValue(gateway),
            voiceAuthClientProvider.overrideWithValue(
              VoiceAuthClient(gateway: gateway),
            ),
            authControllerProvider.overrideWith((ref) {
              controller = AuthController(
                authClient: ref.watch(voiceAuthClientProvider),
                storage: ref.watch(authSessionStorageProvider),
                guestCredentialsStorage: ref.watch(
                  guestCredentialsStorageProvider,
                ),
              );
              controller.state = const AuthState(
                session: AuthSession(
                  accessToken: 'token',
                  refreshToken: 'refresh',
                  expiresInSeconds: 900,
                  accountId: 'account-1',
                  activeProfileId: 'profile-primary',
                ),
              );
              return controller;
            }),
          ],
          child: MaterialApp(
            locale: const Locale('ru'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const SecuritySettingsScreen(),
          ),
        ),
      );
      await tester.pumpAndSettle();

      await tester.scrollUntilVisible(
        find.byKey(SecuritySettingsScreen.deleteAccountButtonKey),
        120,
        scrollable: find
            .descendant(
              of: find.byKey(SecuritySettingsScreen.screenKey),
              matching: find.byType(Scrollable),
            )
            .first,
      );
      await tester.tap(
        find.byKey(SecuritySettingsScreen.deleteAccountButtonKey),
      );
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(SecuritySettingsScreen.deleteAccountPasswordKey),
        'secret',
      );
      await tester.tap(
        find
            .descendant(
              of: find.byKey(SecuritySettingsScreen.deleteAccountDialogKey),
              matching: find.byType(TextButton),
            )
            .last,
      );
      await tester.pumpAndSettle();

      expect(deleteAttempts, 1);
      expect(
        find.byKey(SecuritySettingsScreen.deleteAccountTotpKey),
        findsOneWidget,
      );
      expect(
        find.bySemanticsLabel('Код аутентификатора или резервный'),
        findsOneWidget,
      );
      expect(
        find.text(
          'Введите код из приложения-аутентификатора или резервный код.',
        ),
        findsOneWidget,
      );
      expect(logoutCalled, isFalse);

      await tester.enterText(
        find.byKey(SecuritySettingsScreen.deleteAccountTotpKey),
        'invalid-code',
      );
      await tester.tap(
        find
            .descendant(
              of: find.byKey(SecuritySettingsScreen.deleteAccountDialogKey),
              matching: find.byType(TextButton),
            )
            .last,
      );
      await tester.pumpAndSettle();

      expect(deleteAttempts, 2);
      expect(
        find.text('Неверный код аутентификатора или резервный код.'),
        findsOneWidget,
      );
      expect(logoutCalled, isFalse);

      await tester.enterText(
        find.byKey(SecuritySettingsScreen.deleteAccountTotpKey),
        '654321',
      );
      await tester.tap(
        find
            .descendant(
              of: find.byKey(SecuritySettingsScreen.deleteAccountDialogKey),
              matching: find.byType(TextButton),
            )
            .last,
      );
      await tester.pumpAndSettle();

      expect(deleteAttempts, 3);
      expect(logoutCalled, isTrue);
      expect(controller.state.session, isNull);
    },
  );

  testWidgets(
    'totp_required accepts a backup code and logs out after success',
    (tester) async {
      var deleteAttempts = 0;
      var logoutCalled = false;
      late AuthController controller;
      final mock = MockClient((req) async {
        if (req.method == 'POST' &&
            req.url.path == '/api/v1/auth/delete-account') {
          deleteAttempts++;
          final body = jsonDecode(req.body) as Map<String, dynamic>;
          expect(body['password'], 'secret');
          if (deleteAttempts == 1) {
            expect(body.containsKey('totp_code'), isFalse);
            return http.Response(jsonEncode({'error': 'totp_required'}), 401);
          }
          expect(body['totp_code'], 'backup-code-123');
          return http.Response('', 204);
        }
        if (req.method == 'POST' && req.url.path == '/api/v1/auth/logout') {
          logoutCalled = true;
          return http.Response('', 204);
        }
        return http.Response('not found', 404);
      });

      final gateway = GatewayHttpClient(
        httpClient: mock,
        config: const GatewayConfig(baseUrl: 'http://api.test'),
        authorizationProvider: () => 'Bearer token',
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            ...voiceThemeTestOverrides(),
            authSessionStorageProvider.overrideWithValue(_MemoryAuthStorage()),
            guestCredentialsStorageProvider.overrideWithValue(
              InMemoryGuestCredentialsStorage(),
            ),
            gatewayConfigProvider.overrideWithValue(
              const GatewayConfig(baseUrl: 'http://api.test'),
            ),
            gatewayHttpClientProvider.overrideWithValue(gateway),
            voiceAuthClientProvider.overrideWithValue(
              VoiceAuthClient(gateway: gateway),
            ),
            authControllerProvider.overrideWith((ref) {
              controller = AuthController(
                authClient: ref.watch(voiceAuthClientProvider),
                storage: ref.watch(authSessionStorageProvider),
                guestCredentialsStorage: ref.watch(
                  guestCredentialsStorageProvider,
                ),
              );
              controller.state = const AuthState(
                session: AuthSession(
                  accessToken: 'token',
                  refreshToken: 'refresh',
                  expiresInSeconds: 900,
                  accountId: 'account-1',
                  activeProfileId: 'profile-primary',
                ),
              );
              return controller;
            }),
          ],
          child: MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const SecuritySettingsScreen(),
          ),
        ),
      );
      await tester.pumpAndSettle();

      await tester.scrollUntilVisible(
        find.byKey(SecuritySettingsScreen.deleteAccountButtonKey),
        120,
        scrollable: find
            .descendant(
              of: find.byKey(SecuritySettingsScreen.screenKey),
              matching: find.byType(Scrollable),
            )
            .first,
      );
      await tester.tap(
        find.byKey(SecuritySettingsScreen.deleteAccountButtonKey),
      );
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(SecuritySettingsScreen.deleteAccountPasswordKey),
        'secret',
      );
      await tester.tap(
        find.descendant(
          of: find.byKey(SecuritySettingsScreen.deleteAccountDialogKey),
          matching: find.text('Delete'),
        ),
      );
      await tester.pumpAndSettle();

      expect(deleteAttempts, 1);
      await tester.enterText(
        find.byKey(SecuritySettingsScreen.deleteAccountTotpKey),
        'backup-code-123',
      );
      await tester.tap(
        find.descendant(
          of: find.byKey(SecuritySettingsScreen.deleteAccountDialogKey),
          matching: find.text('Delete'),
        ),
      );
      await tester.pumpAndSettle();

      expect(deleteAttempts, 2);
      expect(logoutCalled, isTrue);
      expect(controller.state.session, isNull);
    },
  );
}
