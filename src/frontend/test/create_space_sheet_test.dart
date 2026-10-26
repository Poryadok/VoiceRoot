import 'dart:convert';
import 'dart:async';
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
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
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/chat/chat_list_panel.dart';
import 'package:voice_frontend/ui/space/create_space_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  ThemeData? captureTheme;
  setUpAll(() async {
    if (const bool.fromEnvironment('VOICE_CAPTURE_SPACE_CREATE')) {
      await (FontLoader(VoiceTheme.fontFamily)
            ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
            ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
            ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf')))
          .load();
      await (FontLoader(
        'MaterialIcons',
      )..addFont(rootBundle.load('fonts/MaterialIcons-Regular.otf'))).load();
      final catalog = await VoiceTokenCatalog.load();
      captureTheme = await VoiceTheme.build(
        catalog: catalog,
        mode: VoiceThemeMode.light,
        profileAccent: catalog.profileAccentAt(0),
      );
    }
  });
  Widget testApp({
    required Widget home,
    required http.Client client,
    Locale locale = const Locale('en'),
    List<Override> extraOverrides = const [],
    double textScale = 1,
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
        builder: (context, child) => MediaQuery(
          data: MediaQuery.of(
            context,
          ).copyWith(textScaler: TextScaler.linear(textScale)),
          child: child!,
        ),
        theme: captureTheme ?? voiceTestTheme(),
        locale: locale,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(body: home),
      ),
    );
  }

  http.Response listSpacesResponse() {
    return http.Response(
      jsonEncode({
        'space_list': {'spaces': []},
      }),
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

  Widget openButton() => Builder(
    builder: (context) => TextButton(
      onPressed: () => CreateSpaceSheet.show(context),
      child: const Text('open'),
    ),
  );

  for (final size in [
    const Size(1440, 900),
    const Size(390, 844),
    const Size(320, 568),
  ]) {
    testWidgets(
      'responsive creation panel at ${size.width} has reachable controls',
      (tester) async {
        tester.view.devicePixelRatio = 1;
        tester.view.physicalSize = size;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);
        var requests = 0;
        await tester.pumpWidget(
          testApp(
            home: openButton(),
            locale: const Locale('ru'),
            textScale: size.width == 320 ? 1.5 : 1,
            client: MockClient((_) async {
              requests++;
              return http.Response('{}', 500);
            }),
          ),
        );
        await tester.tap(find.text('open'));
        await tester.pumpAndSettle();
        expect(
          tester.getSize(find.byKey(CreateSpaceSheet.sheetKey)).width,
          size.width >= 840 ? 320 : size.width,
        );
        expect(tester.takeException(), isNull);
        await tester.enterText(
          find.byKey(CreateSpaceSheet.iconFieldKey),
          'invalid',
        );
        await tester.pump();
        expect(
          tester
              .widget<FilledButton>(find.byKey(CreateSpaceSheet.submitKey))
              .onPressed,
          isNull,
        );
        final l10n = AppLocalizations.of(
          tester.element(find.byKey(CreateSpaceSheet.sheetKey)),
        )!;
        expect(find.text(l10n.spaceCreateIconInvalid), findsOneWidget);
        expect(find.text(l10n.spaceCreateTemplatesUnavailable), findsOneWidget);
        if (const bool.fromEnvironment('VOICE_CAPTURE_SPACE_CREATE')) {
          await tester.enterText(find.byKey(CreateSpaceSheet.iconFieldKey), '');
          await tester.pumpAndSettle();
          final boundary = tester.renderObject<RenderRepaintBoundary>(
            find
                .ancestor(
                  of: find.byKey(CreateSpaceSheet.sheetKey),
                  matching: find.byType(RepaintBoundary),
                )
                .first,
          );
          await tester.runAsync(() async {
            final image = await boundary.toImage();
            final png = await image.toByteData(format: ui.ImageByteFormat.png);
            final file = File(
              '../../tmp/swarm/anton159-space-create-${size.width.toInt()}.png',
            );
            await file.parent.create(recursive: true);
            await file.writeAsBytes(png!.buffer.asUint8List());
            image.dispose();
          });
        }
        await tester.ensureVisible(
          find.byKey(CreateSpaceSheet.descriptionFieldKey),
        );
        await tester.pumpAndSettle();
        expect(tester.takeException(), isNull);
        expect(
          tester.getBottomRight(find.byKey(CreateSpaceSheet.submitKey)).dy,
          lessThanOrEqualTo(size.height),
        );
        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
        await tester.pumpAndSettle();
        expect(find.byKey(CreateSpaceSheet.sheetKey), findsNothing);
        expect(requests, 0);
      },
    );
  }

  testWidgets('rejected creation keeps input and can retry safely', (
    tester,
  ) async {
    var posts = 0;
    await tester.pumpWidget(
      testApp(
        home: openButton(),
        client: MockClient((req) async {
          if (req.method == 'POST') {
            posts++;
            if (posts == 1) return http.Response('{}', 400);
            return http.Response(
              jsonEncode({'space': spaceJson(id: 'new', name: 'Team')}),
              200,
            );
          }
          return listSpacesResponse();
        }),
      ),
    );
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(CreateSpaceSheet.nameFieldKey), 'Team');
    await tester.pump();
    await tester.tap(find.byKey(CreateSpaceSheet.submitKey));
    await tester.pumpAndSettle();
    expect(
      tester
          .widget<TextField>(find.byKey(CreateSpaceSheet.nameFieldKey))
          .controller!
          .text,
      'Team',
    );
    await tester.tap(find.byKey(CreateSpaceSheet.submitKey));
    await tester.pumpAndSettle();
    expect(posts, 2);
    expect(find.byKey(CreateSpaceSheet.sheetKey), findsNothing);
  });

  testWidgets('visibility choice reaches the create request', (tester) async {
    Map<String, dynamic>? body;
    await tester.pumpWidget(
      testApp(
        home: openButton(),
        client: MockClient((req) async {
          if (req.method == 'POST') {
            body = jsonDecode(req.body) as Map<String, dynamic>;
            return http.Response(
              jsonEncode({'space': spaceJson(id: 'new', name: 'Team')}),
              200,
            );
          }
          return listSpacesResponse();
        }),
      ),
    );
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(CreateSpaceSheet.nameFieldKey), 'Team');
    await tester.tap(find.text('Public'));
    await tester.pump();
    await tester.tap(find.byKey(CreateSpaceSheet.submitKey));
    await tester.pumpAndSettle();
    expect(body?['visibility'], 'public');
  });

  testWidgets('icon recovery updates the created space without another POST', (
    tester,
  ) async {
    var posts = 0;
    var patches = 0;
    await tester.pumpWidget(
      testApp(
        home: openButton(),
        client: MockClient((req) async {
          if (req.method == 'POST') {
            posts++;
            return http.Response(
              jsonEncode({'space': spaceJson(id: 'new', name: 'Team')}),
              200,
            );
          }
          if (req.method == 'PATCH') {
            patches++;
            expect(req.url.path, '/api/v1/spaces/new');
            if (patches == 1) return http.Response('{}', 503);
            return http.Response(
              jsonEncode({'space': spaceJson(id: 'new', name: 'Team')}),
              200,
            );
          }
          return listSpacesResponse();
        }),
      ),
    );
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(CreateSpaceSheet.nameFieldKey), 'Team');
    await tester.enterText(
      find.byKey(CreateSpaceSheet.iconFieldKey),
      'https://cdn.test/icon.png',
    );
    await tester.tap(find.byKey(CreateSpaceSheet.submitKey));
    await tester.pumpAndSettle();
    expect(find.text('Retry icon'), findsOneWidget);
    expect(
      tester
          .widget<TextField>(find.byKey(CreateSpaceSheet.nameFieldKey))
          .enabled,
      isFalse,
    );
    await tester.tap(find.byKey(CreateSpaceSheet.submitKey));
    await tester.pumpAndSettle();
    expect(posts, 1);
    expect(patches, 2);
    expect(find.byKey(CreateSpaceSheet.sheetKey), findsNothing);
  });

  testWidgets('unknown POST outcome cannot create a second space', (
    tester,
  ) async {
    var posts = 0;
    await tester.pumpWidget(
      testApp(
        home: openButton(),
        client: MockClient((req) async {
          if (req.method == 'POST') {
            posts++;
            return http.Response('{}', 503);
          }
          return listSpacesResponse();
        }),
      ),
    );
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(CreateSpaceSheet.nameFieldKey), 'Team');
    await tester.pump();
    await tester.tap(find.byKey(CreateSpaceSheet.submitKey));
    await tester.pumpAndSettle();
    expect(
      tester
          .widget<FilledButton>(find.byKey(CreateSpaceSheet.submitKey))
          .onPressed,
      isNull,
    );
    expect(find.textContaining('Check your spaces'), findsOneWidget);
    expect(posts, 1);
    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();
    expect(find.byKey(CreateSpaceSheet.sheetKey), findsNothing);
  });

  testWidgets('pending creation disables edits and duplicate submission', (
    tester,
  ) async {
    final response = Completer<http.Response>();
    var posts = 0;
    await tester.pumpWidget(
      testApp(
        home: openButton(),
        client: MockClient((req) async {
          if (req.method == 'POST') {
            posts++;
            return response.future;
          }
          return listSpacesResponse();
        }),
      ),
    );
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(CreateSpaceSheet.nameFieldKey), 'Team');
    await tester.pump();
    await tester.tap(find.byKey(CreateSpaceSheet.submitKey));
    await tester.pump();
    expect(
      tester
          .widget<TextField>(find.byKey(CreateSpaceSheet.nameFieldKey))
          .enabled,
      isFalse,
    );
    expect(
      tester
          .widget<FilledButton>(find.byKey(CreateSpaceSheet.submitKey))
          .onPressed,
      isNull,
    );
    expect(posts, 1);
    response.complete(
      http.Response(
        jsonEncode({'space': spaceJson(id: 'new', name: 'Team')}),
        200,
      ),
    );
    await tester.pumpAndSettle();
  });

  testWidgets('ChatListPanel opens create space sheet', (tester) async {
    await tester.pumpWidget(
      testApp(
        home: const ChatListPanel(),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {'items': []},
              }),
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
    var requestCount = 0;
    await tester.pumpWidget(
      testApp(
        home: Builder(
          builder: (context) => TextButton(
            onPressed: () => CreateSpaceSheet.show(context),
            child: const Text('open'),
          ),
        ),
        client: MockClient((_) async {
          requestCount++;
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    final submit = tester.widget<FilledButton>(
      find.byKey(CreateSpaceSheet.submitKey),
    );
    expect(submit.onPressed, isNull);

    await tester.enterText(find.byKey(CreateSpaceSheet.nameFieldKey), '   ');
    await tester.pump();
    expect(
      tester
          .widget<FilledButton>(find.byKey(CreateSpaceSheet.submitKey))
          .onPressed,
      isNull,
    );
    expect(requestCount, 0);

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
  for (final language in ['en', 'ru']) {
    for (final statusCode in [500, 503]) {
      for (final failIconPatch in [false, true]) {
        final failedStage = failIconPatch ? 'icon PATCH' : 'POST';
        testWidgets(
          'CreateSpaceSheet $language $statusCode $failedStage hides diagnostics '
          'and preserves recovery',
          (tester) async {
            const privateMarker = 'private-space-diagnostic-4f2b';
            const iconUrl = 'https://cdn.voice.gg/spaces/party.webp';
            final calls = <String>[];
            final createBodies = <Map<String, dynamic>>[];
            final patchBodies = <Map<String, dynamic>>[];
            await tester.pumpWidget(
              testApp(
                locale: Locale(language),
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
                  if (req.method == 'POST' &&
                      req.url.path == '/api/v1/spaces') {
                    createBodies.add(
                      jsonDecode(req.body) as Map<String, dynamic>,
                    );
                    if (!failIconPatch && createBodies.length == 1) {
                      return http.Response(
                        jsonEncode({'message': privateMarker}),
                        statusCode,
                      );
                    }
                    return http.Response(
                      jsonEncode({
                        'space': spaceJson(id: 'space-new', name: 'Raid HQ'),
                      }),
                      200,
                    );
                  }
                  if (req.method == 'PATCH' &&
                      req.url.path == '/api/v1/spaces/space-new') {
                    patchBodies.add(
                      jsonDecode(req.body) as Map<String, dynamic>,
                    );
                    if (failIconPatch) {
                      return http.Response(
                        jsonEncode({'message': privateMarker}),
                        statusCode,
                      );
                    }
                    return http.Response(
                      jsonEncode({
                        'space': spaceJson(
                          id: 'space-new',
                          name: 'Raid HQ',
                          iconUrl: iconUrl,
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
              '  Raid HQ  ',
            );
            await tester.enterText(
              find.byKey(CreateSpaceSheet.descriptionFieldKey),
              '  We raid on Fridays  ',
            );
            await tester.enterText(
              find.byKey(CreateSpaceSheet.iconFieldKey),
              '  $iconUrl  ',
            );
            await tester.pump();
            await tester.tap(find.byKey(CreateSpaceSheet.submitKey));
            await tester.pumpAndSettle();

            final l10n = AppLocalizations.of(
              tester.element(find.byKey(CreateSpaceSheet.sheetKey)),
            )!;
            expect(
              find.text(
                failIconPatch
                    ? l10n.spaceCreateIconFailed
                    : l10n.spaceCreateUncertain,
              ),
              findsOneWidget,
            );
            expect(find.textContaining(privateMarker), findsNothing);
            expect(find.textContaining('Exception'), findsNothing);
            for (final field in [
              (CreateSpaceSheet.nameFieldKey, '  Raid HQ  '),
              (CreateSpaceSheet.descriptionFieldKey, '  We raid on Fridays  '),
              (CreateSpaceSheet.iconFieldKey, '  $iconUrl  '),
            ]) {
              final input = tester.widget<TextField>(find.byKey(field.$1));
              expect(input.controller!.text, field.$2);
              expect(
                input.enabled,
                failIconPatch && field.$1 == CreateSpaceSheet.iconFieldKey,
              );
            }
            expect(
              tester
                  .widget<FilledButton>(find.byKey(CreateSpaceSheet.submitKey))
                  .onPressed,
              failIconPatch ? isNotNull : isNull,
            );
            expect(createBodies, [
              {
                'name': 'Raid HQ',
                'description': 'We raid on Fridays',
                'visibility': 'private',
              },
            ]);
            expect(
              patchBodies,
              failIconPatch
                  ? [
                      {'icon_url': iconUrl},
                    ]
                  : isEmpty,
            );
            final firstAttemptCalls = [
              'POST /api/v1/spaces',
              if (failIconPatch) 'PATCH /api/v1/spaces/space-new',
            ];
            expect(
              calls.where((call) => !call.startsWith('GET')),
              firstAttemptCalls,
            );
            await tester.tap(find.byKey(CreateSpaceSheet.closeKey));
            await tester.pumpAndSettle();
            expect(
              calls.where((call) => !call.startsWith('GET')),
              firstAttemptCalls,
            );
            expect(find.byKey(CreateSpaceSheet.sheetKey), findsNothing);
          },
        );
      }
    }

    testWidgets('CreateSpaceSheet $language no-session failure is safe', (
      tester,
    ) async {
      var requestCount = 0;
      await tester.pumpWidget(
        testApp(
          locale: Locale(language),
          home: const CreateSpaceSheet(),
          extraOverrides: [authorizationHeaderProvider.overrideWithValue(null)],
          client: MockClient((_) async {
            requestCount++;
            return http.Response('{}', 500);
          }),
        ),
      );
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(CreateSpaceSheet.nameFieldKey),
        'Raid HQ',
      );
      await tester.pump();
      await tester.tap(find.byKey(CreateSpaceSheet.submitKey));
      await tester.pumpAndSettle();

      final l10n = AppLocalizations.of(
        tester.element(find.byKey(CreateSpaceSheet.sheetKey)),
      )!;
      expect(
        find.text(l10n.spaceCreateError(l10n.commonActionFailed)),
        findsOneWidget,
      );
      expect(find.textContaining('not_authenticated'), findsNothing);
      expect(requestCount, 0);
      expect(
        tester
            .widget<TextField>(find.byKey(CreateSpaceSheet.nameFieldKey))
            .controller!
            .text,
        'Raid HQ',
      );
      expect(
        tester
            .widget<FilledButton>(find.byKey(CreateSpaceSheet.submitKey))
            .onPressed,
        isNotNull,
      );
    });
  }
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
