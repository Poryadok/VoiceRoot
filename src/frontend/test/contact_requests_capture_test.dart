import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/core/voice_skeleton.dart';
import 'package:voice_frontend/ui/social/social_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';

const _rootKey = Key('contact_requests_capture_root');
const _captureEnv = 'VOICE_CONTACT_REQUESTS_CAPTURE_DIR';

Future<ThemeData> _theme(WidgetTester tester, bool capture) async {
  if (!capture) {
    return VoiceTheme.build(
      catalog: testVoiceTokenCatalog,
      mode: VoiceThemeMode.dark,
      profileAccent: testVoiceTokenCatalog.profileAccentAt(0),
    );
  }
  final theme = await tester.runAsync(() async {
    await (FontLoader(VoiceTheme.fontFamily)
          ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
          ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
          ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
          ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf')))
        .load();
    final flutterRoot = Platform.environment['FLUTTER_ROOT'];
    if (flutterRoot == null || flutterRoot.isEmpty) {
      throw StateError('FLUTTER_ROOT is required for Material Icons');
    }
    final bytes = await File(
      '$flutterRoot/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
    ).readAsBytes();
    await (FontLoader('MaterialIcons')
          ..addFont(Future.value(ByteData.sublistView(bytes))))
        .load();
    final catalog = await VoiceTokenCatalog.load();
    return VoiceTheme.build(
      catalog: catalog,
      mode: VoiceThemeMode.dark,
      profileAccent: catalog.profileAccentAt(0),
    );
  });
  if (theme == null) throw StateError('Capture theme loading did not finish');
  return theme;
}

Future<void> _capture(WidgetTester tester, String? directory, String name) async {
  if (directory == null || directory.isEmpty) return;
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_rootKey),
  );
  final written = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
      if (bytes == null) throw StateError('PNG encoding returned no bytes');
      final file = File('$directory/$name');
      await file.parent.create(recursive: true);
      await file.writeAsBytes(bytes.buffer.asUint8List(), flush: true);
      return true;
    } finally {
      image.dispose();
    }
  });
  if (written != true) throw StateError('Capture did not finish');
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}

void main() {
  testWidgets('actual pending Friends panel H/V state captures', (tester) async {
    final directory = Platform.environment[_captureEnv];
    final theme = await _theme(tester, directory != null && directory.isNotEmpty);
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    for (final layout in ['h', 'v']) {
      final size = layout == 'h' ? const Size(1280, 800) : const Size(390, 844);
      tester.view.physicalSize = size;
      tester.view.devicePixelRatio = 1;
      for (final scenario in [
        'populated', 'busy', 'empty', 'loading', 'error', 'ru',
        'profile-error', 'profile-missing', 'profile-loading',
      ]) {
        final pending = Completer<http.Response>();
        final client = MockClient((request) async {
          if (request.url.path == '/api/v1/friends/requests') {
            if (scenario == 'loading') return pending.future;
            if (scenario == 'error') {
              return http.Response('{"message":"private load diagnostic"}', 503);
            }
            return http.Response(
              jsonEncode({
                'friend_request_list': {
                  'incoming': [
                    if (scenario != 'empty') {'profile_id': 'incoming'},
                  ],
                  'outgoing': [
                    if (scenario != 'empty') ...[
                      {'profile_id': 'outgoing', 'status': 'pending'},
                      {'profile_id': 'declined', 'status': 'declined'},
                    ],
                  ],
                },
              }),
              200,
            );
          }
          if (request.url.path.endsWith('/incoming/decline')) {
            return pending.future;
          }
          if (request.url.path.startsWith('/api/v1/users/profiles/')) {
            final id = request.url.pathSegments.last;
            if (id == 'incoming') {
              if (scenario == 'profile-loading') return pending.future;
              if (scenario == 'profile-error' || scenario == 'profile-missing') {
                return http.Response(
                  '{"message":"private profile diagnostic"}',
                  scenario == 'profile-error' ? 503 : 404,
                );
              }
            }
            return http.Response(
              jsonEncode({
                'profile': {
                  'id': id,
                  'account_id': 'account-$id',
                  'username': id,
                  'discriminator': '0001',
                  'display_name': switch (id) {
                    'incoming' => 'Alex',
                    'outgoing' => 'Maria',
                    _ => 'Denis',
                  },
                },
              }),
              200,
            );
          }
          return http.Response('{}', 200);
        });
        final container = ProviderContainer(overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          discoverHintStorageProvider.overrideWithValue(testDiscoverHintStorage),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(client),
          realtimeLinkStatusProvider.overrideWith(
            (ref) => RealtimeLinkStatus.disconnected,
          ),
          realtimeEventProvider.overrideWith((ref) => const Stream.empty()),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
        ]);
        const panel = Scaffold(body: SocialPanel(initialTabIndex: 4));
        await tester.pumpWidget(
          RepaintBoundary(
            key: _rootKey,
            child: UncontrolledProviderScope(
              container: container,
              child: MaterialApp(
                debugShowCheckedModeBanner: false,
                theme: theme,
                locale: Locale(scenario == 'ru' ? 'ru' : 'en'),
                localizationsDelegates: AppLocalizations.localizationsDelegates,
                supportedLocales: AppLocalizations.supportedLocales,
                home: layout == 'h'
                    ? const Scaffold(
                        body: Align(
                          alignment: Alignment.centerLeft,
                          child: SizedBox(width: 360, child: panel),
                        ),
                      )
                    : panel,
              ),
            ),
          ),
        );
        if (scenario == 'loading') {
          await tester.pump();
          expect(find.byType(VoiceListSkeleton), findsOneWidget);
        } else {
          await tester.pumpAndSettle();
          if (scenario == 'busy') {
            await tester.tap(
              find.byKey(SocialPanel.requestDeclineKey('incoming')),
            );
            await tester.pump();
            await tester.pump(const Duration(milliseconds: 150));
            expect(
              find.byKey(SocialPanel.requestActionProgressKey('incoming')),
              findsOneWidget,
            );
            for (final key in [
              SocialPanel.requestAcceptKey('incoming'),
              SocialPanel.requestDeclineKey('incoming'),
            ]) {
              expect(tester.widget<IconButton>(find.byKey(key)).onPressed, isNull);
            }
          } else if (scenario == 'empty') {
            expect(find.text('No friend requests'), findsOneWidget);
          } else if (scenario == 'error') {
            expect(find.byKey(SocialPanel.requestsUnavailableKey), findsOneWidget);
            expect(find.text('Try again'), findsOneWidget);
            expect(find.textContaining('private load diagnostic'), findsNothing);
          } else if (scenario.startsWith('profile-')) {
            final l10n = AppLocalizations.of(
              tester.element(find.byKey(SocialPanel.panelKey)),
            )!;
            expect(
              find.text(
                scenario == 'profile-loading'
                    ? l10n.commonLoading
                    : l10n.socialProfileUnavailable,
              ),
              findsOneWidget,
            );
            expect(find.text('incoming'), findsNothing);
            expect(
              find.textContaining('private profile diagnostic'),
              findsNothing,
            );
            for (final key in [
              SocialPanel.requestAcceptKey('incoming'),
              SocialPanel.requestDeclineKey('incoming'),
            ]) {
              expect(
                tester.widget<IconButton>(find.byKey(key)).onPressed,
                isNotNull,
              );
            }
          } else {
            expect(find.text('Alex'), findsOneWidget);
            expect(find.text('Maria'), findsOneWidget);
            expect(find.text('Denis'), findsOneWidget);
            expect(
              find.byKey(SocialPanel.requestCancelKey('outgoing')),
              findsOneWidget,
            );
            expect(
              find.byKey(SocialPanel.requestCancelKey('declined')),
              findsNothing,
            );
          }
        }
        expect(tester.takeException(), isNull);
        await _capture(tester, directory, 'contact-requests-$scenario-$layout.png');
        await tester.pumpWidget(const SizedBox.shrink());
        container.dispose();
        if (!pending.isCompleted) pending.complete(http.Response('{}', 200));
        client.close();
      }
    }
  });
}
