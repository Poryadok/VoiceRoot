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
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/core/voice_skeleton.dart';
import 'package:voice_frontend/ui/search/global_search_panel.dart';
import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';

void main() {
  testWidgets('global search real-theme H V and RU states', (tester) async {
    final directory = Platform.environment['VOICE_GLOBAL_SEARCH_CAPTURE_DIR'];
    final capture = directory != null && directory.isNotEmpty;
    // Real Gateway answers always carry a UTF-8 JSON content type; package:http
    // encodes a String body with the charset from that header (latin1 otherwise).
    const jsonHeaders = {'content-type': 'application/json; charset=utf-8'};
    final themes = <VoiceThemeMode, ThemeData>{};
    for (final mode in [VoiceThemeMode.light, VoiceThemeMode.dark]) {
      final theme = await tester.runAsync(() async {
        if (capture && themes.isEmpty) {
          await (FontLoader(VoiceTheme.fontFamily)
            ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
            ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
            ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
            ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf'))).load();
          final root = Platform.environment['FLUTTER_ROOT']!;
          final icons = await File('$root/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf').readAsBytes();
          await (FontLoader('MaterialIcons')..addFont(Future.value(ByteData.sublistView(icons)))).load();
        }
        final catalog = capture ? await VoiceTokenCatalog.load() : testVoiceTokenCatalog;
        return VoiceTheme.build(catalog: catalog, mode: mode, profileAccent: catalog.profileAccentAt(0));
      });
      themes[mode] = theme!;
    }
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    for (final layout in ['h', 'v']) {
      final size = layout == 'h' ? const Size(1280, 800) : const Size(390, 844);
      tester.view.physicalSize = size;
      tester.view.devicePixelRatio = 1;
      for (final scenario in ['mixed', 'ru', 'dark', 'empty', 'loading', 'error']) {
        final pending = Completer<http.Response>();
        final client = MockClient((request) async {
          if (request.url.path == '/api/v1/search/global') {
            if (scenario == 'loading') return pending.future;
            if (scenario == 'error') return http.Response('{"message":"private upstream diagnostic"}', 503);
            return http.Response(jsonEncode({'global_search_results': {
              'profile_ids': scenario == 'empty' ? [] : ['alex'],
              'matched_chats': scenario == 'empty' ? [] : [{'id':'design'}],
              'space_ids': scenario == 'empty' ? [] : ['hq'],
              'messages': scenario == 'empty' ? [] : [{'chat_id':'design','message_id':'m1','snippet':'Финальный <mark>design</mark> уже готов'}],
              'next_cursor': scenario == 'empty' ? '' : 'next',
            }}), 200, headers: jsonHeaders);
          }
          if (request.url.path.contains('/users/profiles/')) return http.Response(jsonEncode({'profile': {
            'id':'alex','account_id':'a','username':'alex','display_name':'Alex','discriminator':'0001',
          }}), 200);
          if (request.url.path == '/api/v1/chats') return http.Response(jsonEncode({'chat_list': {'items': [
            {'chat': {'id':'design','name':'Design Team','type':'CHAT_TYPE_GROUP'},'last_message_preview':'Новый макет shell'},
          ]}}), 200, headers: jsonHeaders);
          if (request.url.path == '/api/v1/spaces/hq') return http.Response(jsonEncode({'space': {'id':'hq','name':'Voice HQ','member_count':1248}}), 200);
          return http.Response('{}', 200);
        });
        final key = GlobalKey();
        final container = ProviderContainer(overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
          authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(const GatewayConfig(baseUrl:'http://api.test')),
          httpClientProvider.overrideWithValue(client),
        ]);
        await tester.pumpWidget(RepaintBoundary(key:key, child: UncontrolledProviderScope(container:container,
          child: MaterialApp(debugShowCheckedModeBanner:false,
            theme:themes[scenario == 'dark' ? VoiceThemeMode.dark : VoiceThemeMode.light],
            locale:Locale(scenario == 'ru' ? 'ru' : 'en'),
            localizationsDelegates:AppLocalizations.localizationsDelegates,
            supportedLocales:AppLocalizations.supportedLocales,
            home:Scaffold(body:Align(alignment:Alignment.centerRight,
              child:SizedBox(width:layout == 'h' ? 320 : size.width,
                child:GlobalSearchPanel(onClose:() {})))),
          ))));
        await tester.pumpAndSettle();
        await tester.enterText(find.byKey(GlobalSearchPanel.searchFieldKey), 'design');
        await tester.pump(const Duration(milliseconds:350));
        if (scenario == 'loading') {
          await tester.pump();
          expect(find.byType(VoiceListSkeleton), findsOneWidget);
        } else {
          await tester.pumpAndSettle();
          if (scenario == 'error') {
            expect(find.byKey(const Key('global_search_retry')), findsOneWidget);
            expect(find.textContaining('private upstream'), findsNothing);
          } else {
            expect(find.byKey(GlobalSearchPanel.contactsSectionKey), findsOneWidget);
            if (scenario != 'empty') expect(find.text('Voice HQ'), findsOneWidget);
          }
        }
        expect(tester.takeException(), isNull);
        if (capture) {
          final boundary = tester.renderObject<RenderRepaintBoundary>(find.byKey(key));
          await tester.runAsync(() async {
            final image = await boundary.toImage(pixelRatio:1);
            final bytes = await image.toByteData(format:ui.ImageByteFormat.png);
            final file = File('$directory/global-search-$scenario-$layout.png');
            await file.parent.create(recursive:true);
            await file.writeAsBytes(bytes!.buffer.asUint8List(), flush:true);
            image.dispose();
          });
        }
        await tester.pumpWidget(const SizedBox.shrink());
        container.dispose();
        if (!pending.isCompleted) pending.complete(http.Response('{}', 200));
        client.close();
      }
    }
  });
}
