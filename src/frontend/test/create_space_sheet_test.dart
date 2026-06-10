import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_list_panel.dart';
import 'package:voice_frontend/ui/space/create_space_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  Widget testApp({
    required Widget home,
    required http.Client client,
    List<Override> extraOverrides = const [],
  }) {
    return ProviderScope(
      overrides: [
        ...voiceThemeTestOverrides(),
        profileAccentStorageProvider.overrideWithValue(
          testProfileAccentStorage,
        ),
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        authControllerProvider.overrideWith(authenticatedAuthController),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(client),
        realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
        ...extraOverrides,
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(body: home),
      ),
    );
  }

  http.Response listSpacesResponse() {
    return http.Response(
      jsonEncode({'space_list': {'spaces': []}}),
      200,
    );
  }

  Map<String, dynamic> spaceJson({
    required String id,
    required String name,
    String description = '',
    String? iconUrl,
  }) {
    return {
      'id': id,
      'name': name,
      'description': description,
      'icon_url': ?iconUrl,
      'visibility': 'private',
      'owner_profile_id': 'profile-me',
      'member_count': 1,
    };
  }

  testWidgets('ChatListPanel opens create space sheet', (tester) async {
    await tester.pumpWidget(
      testApp(
        home: const ChatListPanel(),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({'chat_list': {'items': []}}),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(ChatListPanel.createSpaceKey));
    await tester.pumpAndSettle();

    expect(find.byKey(CreateSpaceSheet.sheetKey), findsOneWidget);
    expect(find.text('New space'), findsOneWidget);
  });

  testWidgets('CreateSpaceSheet requires name before submit', (tester) async {
    await tester.pumpWidget(
      testApp(
        home: Builder(
          builder: (context) => TextButton(
            onPressed: () => CreateSpaceSheet.show(context),
            child: const Text('open'),
          ),
        ),
        client: MockClient((_) async => http.Response('{}', 404)),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    final submit = tester.widget<FilledButton>(
      find.byKey(CreateSpaceSheet.submitKey),
    );
    expect(submit.onPressed, isNull);

    await tester.enterText(
      find.byKey(CreateSpaceSheet.nameFieldKey),
      'Raid HQ',
    );
    await tester.pump();

    final enabled = tester.widget<FilledButton>(
      find.byKey(CreateSpaceSheet.submitKey),
    );
    expect(enabled.onPressed, isNotNull);
  });

  testWidgets('CreateSpaceSheet submits create + icon patch', (tester) async {
    final calls = <String>[];
    await tester.pumpWidget(
      testApp(
        home: Builder(
          builder: (context) => TextButton(
            onPressed: () => CreateSpaceSheet.show(context),
            child: const Text('open'),
          ),
        ),
        client: MockClient((req) async {
          calls.add('${req.method} ${req.url.path}');
          if (req.method == 'GET' && req.url.path == '/api/v1/spaces') {
            return listSpacesResponse();
          }
          if (req.method == 'POST' && req.url.path == '/api/v1/spaces') {
            return http.Response(
              jsonEncode({
                'space': spaceJson(id: 'space-new', name: 'Raid HQ'),
              }),
              200,
            );
          }
          if (req.method == 'PATCH' &&
              req.url.path == '/api/v1/spaces/space-new') {
            return http.Response(
              jsonEncode({
                'space': spaceJson(
                  id: 'space-new',
                  name: 'Raid HQ',
                  iconUrl: 'https://cdn.voice.gg/spaces/party.webp',
                ),
              }),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(CreateSpaceSheet.nameFieldKey),
      'Raid HQ',
    );
    await tester.enterText(
      find.byKey(CreateSpaceSheet.descriptionFieldKey),
      'We raid on Fridays',
    );
    await tester.enterText(
      find.byKey(CreateSpaceSheet.iconFieldKey),
      'https://cdn.voice.gg/spaces/party.webp',
    );
    await tester.pump();

    await tester.tap(find.byKey(CreateSpaceSheet.submitKey));
    await tester.pumpAndSettle();

    expect(calls, contains('POST /api/v1/spaces'));
    expect(calls, contains('PATCH /api/v1/spaces/space-new'));
    expect(find.byKey(CreateSpaceSheet.sheetKey), findsNothing);
  });

  testWidgets('CreateSpaceSheet skips icon patch when icon empty', (
    tester,
  ) async {
    final calls = <String>[];
    await tester.pumpWidget(
      testApp(
        home: Builder(
          builder: (context) => TextButton(
            onPressed: () => CreateSpaceSheet.show(context),
            child: const Text('open'),
          ),
        ),
        client: MockClient((req) async {
          calls.add('${req.method} ${req.url.path}');
          if (req.method == 'GET' && req.url.path == '/api/v1/spaces') {
            return listSpacesResponse();
          }
          if (req.method == 'POST' && req.url.path == '/api/v1/spaces') {
            return http.Response(
              jsonEncode({
                'space': spaceJson(id: 'space-new', name: 'Raid HQ'),
              }),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(CreateSpaceSheet.nameFieldKey),
      'Raid HQ',
    );
    await tester.pump();
    await tester.tap(find.byKey(CreateSpaceSheet.submitKey));
    await tester.pumpAndSettle();

    expect(calls, contains('POST /api/v1/spaces'));
    expect(calls.where((c) => c.startsWith('PATCH')), isEmpty);
    expect(find.byKey(CreateSpaceSheet.sheetKey), findsNothing);
  });
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}

  @override
  Future<void> dispose() async {}
}
