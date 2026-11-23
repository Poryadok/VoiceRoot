// v3 Space Members production captures (TT UIV3-PANEL-SPACE-MEMBERS): wide
// desktop shows the side panel with the Members tab roster; narrow phone shows
// the sheet with the Bans tab and unban buttons. Mirrors the channel
// settings capture model (side panel + backdrop for wide, sheet body for
// narrow, real Voice theme/fonts for capture, settle + overflow guard).

import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/shell/side_panel.dart';
import 'package:voice_frontend/ui/space/space_members_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const _captureEnv = 'SPACE_MEMBERS_CAPTURE_DIR';
const _captureRootKey = ValueKey('space_members_capture_root');
const _spaceId = 'space-1';

const membersJson =
    '{"space_member_list":{"members":['
    '{"space_id":"space-1","profile_id":"member-1","role_names":["Owner"],'
    '"joined_at":{"secondsSinceEpoch":1730000000}},'
    '{"space_id":"space-1","profile_id":"member-2","role_names":["Admin"],'
    '"nickname":"Nick","joined_at":{"secondsSinceEpoch":1730000000}},'
    '{"space_id":"space-1","profile_id":"member-3","role_names":["Member"],'
    '"joined_at":{"secondsSinceEpoch":1730000000}}]}}';

const bansJson =
    '{"ban_list":{"bans":['
    '{"space_id":"space-1","account_id":"account-9","banned_by_profile_id":"member-2",'
    '"reason":"repeat offender","banned_at":{"secondsSinceEpoch":1730246400}},'
    '{"space_id":"space-1","account_id":"account-5","banned_at":{"secondsSinceEpoch":1730160000}'
    '}]}}';

var _captureFontsLoaded = false;

Future<void> _loadCaptureFonts(WidgetTester tester) async {
  if (_captureFontsLoaded) return;
  final loaded = await tester.runAsync(() async {
    final noto = FontLoader(VoiceTheme.fontFamily)
      ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
      ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
      ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
      ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf'));
    await noto.load();

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
    throw StateError('Capture font loading did not finish');
  }
  _captureFontsLoaded = true;
}

Future<ThemeData> _themeForCapture(WidgetTester tester, bool capture) async {
  if (!capture) return voiceTestTheme();
  await _loadCaptureFonts(tester);
  final theme = await tester.runAsync(() async {
    final catalog = await VoiceTokenCatalog.load();
    return VoiceTheme.build(
      catalog: catalog,
      mode: VoiceThemeMode.dark,
      profileAccent: catalog.profileAccentAt(0),
    );
  });
  if (theme == null) {
    throw StateError('Voice theme asset loading did not finish');
  }
  return theme;
}

MockClient _captureClient() => MockClient((request) async {
  if (request.method == 'GET' &&
      request.url.path == '/api/v1/spaces/$_spaceId/members') {
    return http.Response(membersJson, 200);
  }
  if (request.method == 'GET' &&
      request.url.path == '/api/v1/spaces/$_spaceId/bans') {
    return http.Response(bansJson, 200);
  }
  return http.Response('{}', 404);
});

ProviderContainer _container(MockClient client, {bool wide = false}) {
  final container = ProviderContainer(
    overrides: [
      ...voiceThemeTestOverrides(),
      profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      authControllerProvider.overrideWith(authenticatedAuthController),
      spacePermissionProvider.overrideWith((ref, query) async => true),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(client),
    ],
  );
  container.read(spaceMembersTabProvider(_spaceId).notifier).state = wide
      ? SpaceMembersTabKind.members
      : SpaceMembersTabKind.bans;
  container.read(spaceMembersSearchQueryProvider(_spaceId).notifier).state = '';
  if (wide) {
    container.read(selectedSpaceIdProvider.notifier).state = _spaceId;
    container.read(shellSidePanelProvider.notifier).state =
        ShellSidePanel.members;
  }
  return container;
}

Widget _app({
  required ProviderContainer container,
  required ThemeData theme,
  required Widget body,
}) => RepaintBoundary(
  key: _captureRootKey,
  child: UncontrolledProviderScope(
    container: container,
    child: MaterialApp(
      debugShowCheckedModeBanner: false,
      theme: theme,
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: body,
    ),
  ),
);

Future<void> _capture(
  WidgetTester tester,
  String? directory,
  String filename,
) async {
  if (directory == null || directory.isEmpty) {
    return;
  }
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_captureRootKey),
  );
  final bytes = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      final data = await image.toByteData(format: ui.ImageByteFormat.png);
      if (data == null) {
        throw StateError('PNG encoding returned no bytes');
      }
      final output = File('$directory${Platform.pathSeparator}$filename');
      await output.parent.create(recursive: true);
      final png = data.buffer.asUint8List();
      await output.writeAsBytes(png, flush: true);
      return png;
    } finally {
      image.dispose();
    }
  });
  if (bytes == null || bytes.isEmpty) {
    throw StateError('Capture did not finish');
  }
}

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  testWidgets('space members sheet v3 wide and narrow production captures', (
    tester,
  ) async {
    final captureDir = Platform.environment[_captureEnv];
    final capturing = captureDir != null && captureDir.isNotEmpty;
    final theme = await _themeForCapture(tester, capturing);
    final scenarios = [
      (size: const Size(1280, 800), wide: true, file: 'space-members-wide.png'),
      (
        size: const Size(390, 844),
        wide: false,
        file: 'space-members-narrow.png',
      ),
    ];

    for (final scenario in scenarios) {
      tester.view.physicalSize = scenario.size;
      tester.view.devicePixelRatio = 1;
      final container = _container(_captureClient(), wide: scenario.wide);
      addTearDown(container.dispose);
      final body = scenario.wide
          ? Scaffold(
              body: Row(
                children: [
                  const Expanded(child: ColoredBox(color: Color(0xFF17191F))),
                  SizedBox(
                    width: 380,
                    child: SidePanelHost(
                      key: const Key('uiv3_space_members_side_host'),
                    ),
                  ),
                ],
              ),
            )
          : Scaffold(body: SpaceMembersSheet(spaceId: _spaceId));
      await tester.pumpWidget(
        _app(container: container, theme: theme, body: body),
      );
      await tester.pumpAndSettle();

      if (scenario.wide) {
        expect(
          find.byKey(const Key('uiv3_space_members_side_host')),
          findsOneWidget,
        );
        expect(
          find.byKey(const Key('space_members_sheet_tabs')),
          findsOneWidget,
        );
        expect(
          find.byKey(const Key('space_members_sheet_members_tab')),
          findsOneWidget,
        );
        expect(
          find.byKey(const Key('member_profile_open_member-1')),
          findsOneWidget,
        );
        expect(
          find.byKey(SpaceMembersSheet.banMemberKey('member-2')),
          findsOneWidget,
        );
      } else {
        expect(find.byKey(SpaceMembersSheet.sheetKey), findsOneWidget);
        expect(
          find.byKey(const Key('space_members_sheet_bans_tab')),
          findsOneWidget,
        );
        expect(
          find.byKey(const Key('space_members_sheet_bans_body')),
          findsOneWidget,
        );
        expect(
          find.byKey(SpaceMembersSheet.unbanMemberKey('account-9')),
          findsOneWidget,
        );
        expect(find.byKey(SpaceMembersSheet.searchKey), findsOneWidget);
        expect(
          find.byKey(const Key('member_profile_open_member-1')),
          findsNothing,
        );
      }
      expect(find.byType(BottomSheet), findsNothing);
      expect(tester.takeException(), isNull);

      await _capture(tester, captureDir, scenario.file);
      await tester.pumpWidget(const SizedBox.shrink());
    }
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  });
}
