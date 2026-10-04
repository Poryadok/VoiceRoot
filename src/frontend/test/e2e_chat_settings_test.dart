import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/e2e_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/e2e_providers.dart';
import 'package:voice_frontend/ui/chat/e2e_chat_settings.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

/// encryption (docs/features/encryption.md) red tests: E2E opt-in/opt-out confirmation dialogs (docs/features/encryption.md).
void main() {
  Widget e2eDialogTestApp({required Widget child}) {
    return ProviderScope(
      overrides: voiceThemeTestOverrides(),
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(body: child),
      ),
    );
  }

  Widget actionErrorTestApp({
    required _FakeE2eClient client,
    required bool enabled,
  }) {
    const chatId = 'dm-chat-test';
    return ProviderScope(
      overrides: [
        ...voiceThemeTestOverrides(),
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        authorizationHeaderProvider.overrideWith((ref) => 'Bearer test'),
        voiceE2eClientProvider.overrideWith((ref) => client),
        chatE2eEnabledProvider(chatId).overrideWith((ref) => enabled),
        dmPeerProfileByChatIdProvider.overrideWith(
          (ref) => {chatId: 'peer-test'},
        ),
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const Scaffold(body: DmE2eSettingsSection(chatId: chatId)),
      ),
    );
  }

  Future<void> confirmAction(WidgetTester tester) async {
    await tester.tap(find.byKey(const Key('chat_info_e2e_toggle')));
    await tester.pumpAndSettle();
    await tester.tap(
      find.byKey(E2eEnableConfirmDialog.confirmButtonKey).evaluate().isNotEmpty
          ? find.byKey(E2eEnableConfirmDialog.confirmButtonKey)
          : find.byKey(E2eDisableConfirmDialog.confirmButtonKey),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
  }

  testWidgets('prekey upload failure shows safe copy and stops before enable', (
    tester,
  ) async {
    const diagnostic = 'untrusted backend detail';
    final client = _FakeE2eClient(uploadStatus: 500);
    await tester.pumpWidget(actionErrorTestApp(client: client, enabled: false));
    await confirmAction(tester);

    final l10n = AppLocalizations.of(
      tester.element(find.byType(DmE2eSettingsSection)),
    )!;
    expect(find.text(l10n.commonActionFailed), findsOneWidget);
    expect(find.text(diagnostic), findsNothing);
    expect(client.transport.requests, [
      'GET /api/v1/messages/prekeys?profile_id=peer-test',
      'POST /api/v1/messages/prekeys',
    ]);
    expect(find.byType(DmE2eSettingsSection), findsOneWidget);
  });

  testWidgets('enable failure shows unavailable copy and preserves toggle', (
    tester,
  ) async {
    const diagnostic = 'untrusted backend detail';
    final client = _FakeE2eClient(enableStatus: 503);
    await tester.pumpWidget(actionErrorTestApp(client: client, enabled: false));
    await confirmAction(tester);

    final l10n = AppLocalizations.of(
      tester.element(find.byType(DmE2eSettingsSection)),
    )!;
    expect(find.text(l10n.backendUnavailable), findsOneWidget);
    expect(find.text(diagnostic), findsNothing);
    expect(client.transport.requests, [
      'GET /api/v1/messages/prekeys?profile_id=peer-test',
      'POST /api/v1/messages/prekeys',
      'POST /api/v1/chats/dm-chat-test/e2e-enable',
    ]);
    expect(
      tester
          .widget<SwitchListTile>(find.byKey(const Key('chat_info_e2e_toggle')))
          .value,
      isFalse,
    );
  });

  testWidgets('disable failure shows safe copy and preserves enabled toggle', (
    tester,
  ) async {
    const diagnostic = 'untrusted backend detail';
    final client = _FakeE2eClient(disableStatus: 500);
    await tester.pumpWidget(actionErrorTestApp(client: client, enabled: true));
    await confirmAction(tester);

    final l10n = AppLocalizations.of(
      tester.element(find.byType(DmE2eSettingsSection)),
    )!;
    expect(find.text(l10n.commonActionFailed), findsOneWidget);
    expect(find.text(diagnostic), findsNothing);
    expect(client.transport.requests, [
      'POST /api/v1/chats/dm-chat-test/e2e-disable',
    ]);
    expect(
      tester
          .widget<SwitchListTile>(find.byKey(const Key('chat_info_e2e_toggle')))
          .value,
      isTrue,
    );
  });

  testWidgets('E2eEnableConfirmDialog shows search limitation warning', (
    tester,
  ) async {
    await tester.pumpWidget(
      e2eDialogTestApp(
        child: E2eEnableConfirmDialog(
          chatId: 'dm-chat-1',
          onConfirmed: () {},
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(E2eEnableConfirmDialog.dialogKey), findsOneWidget);
    expect(
      find.textContaining('end-to-end encryption', findRichText: true),
      findsOneWidget,
    );
    expect(
      find.textContaining('Global search', findRichText: true),
      findsOneWidget,
    );
    expect(
      find.textContaining('server', findRichText: true),
      findsOneWidget,
    );
    expect(find.byKey(E2eEnableConfirmDialog.confirmButtonKey), findsOneWidget);
    expect(find.byKey(E2eEnableConfirmDialog.cancelButtonKey), findsOneWidget);
  });

  testWidgets('E2eDisableConfirmDialog warns opt-out reverts to plaintext', (
    tester,
  ) async {
    await tester.pumpWidget(
      e2eDialogTestApp(
        child: E2eDisableConfirmDialog(
          chatId: 'dm-chat-1',
          onConfirmed: () {},
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(E2eDisableConfirmDialog.dialogKey), findsOneWidget);
    expect(
      find.textContaining('disable', findRichText: true),
      findsOneWidget,
    );
    expect(
      find.textContaining('plain', findRichText: true),
      findsOneWidget,
    );
    expect(
      find.textContaining('search', findRichText: true),
      findsOneWidget,
    );
    expect(
      find.byKey(E2eDisableConfirmDialog.confirmButtonKey),
      findsOneWidget,
    );
  });

  testWidgets('confirming enable dialog invokes onConfirmed callback', (
    tester,
  ) async {
    var confirmed = false;
    await tester.pumpWidget(
      e2eDialogTestApp(
        child: E2eEnableConfirmDialog(
          chatId: 'dm-chat-1',
          onConfirmed: () => confirmed = true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(E2eEnableConfirmDialog.confirmButtonKey));
    await tester.pumpAndSettle();

    expect(confirmed, isTrue);
  });

  testWidgets('E2eKeyBackupSheet shows password fields and save action', (
    tester,
  ) async {
    await tester.pumpWidget(
      e2eDialogTestApp(
        child: E2eKeyBackupSheet(
          onSave: (_, _) async {},
          onRestore: (_) async {},
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(E2eKeyBackupSheet.sheetKey), findsOneWidget);
    expect(find.byKey(E2eKeyBackupSheet.passwordFieldKey), findsOneWidget);
    expect(find.byKey(E2eKeyBackupSheet.saveButtonKey), findsOneWidget);
    expect(find.byKey(E2eKeyBackupSheet.restoreButtonKey), findsOneWidget);
  });
}

class _FakeE2eClient extends VoiceE2eClient {
  factory _FakeE2eClient({
    int uploadStatus = 204,
    int enableStatus = 204,
    int disableStatus = 204,
  }) {
    return _FakeE2eClient._(
      _E2eRecordingTransport(
        uploadStatus: uploadStatus,
        enableStatus: enableStatus,
        disableStatus: disableStatus,
      ),
    );
  }

  _FakeE2eClient._(this.transport)
    : super(
        gateway: GatewayHttpClient(
          httpClient: transport.client,
          config: const GatewayConfig(baseUrl: 'https://voice.test'),
        ),
      );

  final _E2eRecordingTransport transport;

  @override
  Future<E2eApiResult<void>> uploadPreKeyBundle({
    required String authorization,
    String? bundle,
  }) => super.uploadPreKeyBundle(
    authorization: authorization,
    bundle: bundle ?? 'test-only-prekey-bundle',
  );
}

class _E2eRecordingTransport {
  _E2eRecordingTransport({
    required this.uploadStatus,
    required this.enableStatus,
    required this.disableStatus,
  }) {
    client = MockClient((request) async {
      final suffix = request.url.hasQuery ? '?${request.url.query}' : '';
      requests.add('${request.method} ${request.url.path}$suffix');
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/messages/prekeys') {
        return http.Response(jsonEncode({'bundle': ''}), 200);
      }
      final status = switch (request.url.path) {
        '/api/v1/messages/prekeys' => uploadStatus,
        '/api/v1/chats/dm-chat-test/e2e-enable' => enableStatus,
        '/api/v1/chats/dm-chat-test/e2e-disable' => disableStatus,
        _ => 404,
      };
      if (status >= 400) {
        return http.Response(
          jsonEncode({'message': 'untrusted backend detail'}),
          status,
        );
      }
      return http.Response('', status);
    });
  }

  final int uploadStatus;
  final int enableStatus;
  final int disableStatus;
  final List<String> requests = [];
  late final MockClient client;
}
