import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/space_leave.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/ui/space/leave_space_sheet.dart';
import 'package:voice_frontend/ui/space/space_tree_column.dart';
import 'package:voice_frontend/backend/space_permissions.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets(
    'narrow Space header opens real confirmation and completes leave',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(390, 844));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      var requests = 0;
      await _mount(
        tester,
        column: true,
        respond: (request) async {
          if (request.method == 'POST') requests++;
          return http.Response('', 204);
        },
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('space_lifecycle_menu')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Leave Space'));
      await tester.pumpAndSettle();
      expect(find.byKey(LeaveSpaceSheet.confirmKey), findsOneWidget);
      expect(requests, 0);
      await tester.tap(find.byKey(LeaveSpaceSheet.confirmKey));
      await tester.pumpAndSettle();
      expect(requests, 1);
      expect(find.byKey(LeaveSpaceSheet.confirmKey), findsNothing);
      expect(tester.takeException(), isNull);
    },
  );
  for (final locale in ['en', 'ru']) {
    testWidgets('confirmation and owner explanation in $locale', (
      tester,
    ) async {
      var requests = 0;
      await _mount(
        tester,
        locale: locale,
        owner: true,
        respond: (_) async {
          requests++;
          return http.Response('', 204);
        },
      );
      await tester.pumpAndSettle();
      final button = tester.widget<FilledButton>(
        find.byKey(LeaveSpaceSheet.confirmKey),
      );
      expect(button.onPressed, isNull);
      expect(find.byKey(LeaveSpaceSheet.ownerHintKey), findsOneWidget);
      expect(requests, 0);
    });
  }

  testWidgets(
    'confirmation disables duplicate submit and retains rejection without raw diagnostics',
    (tester) async {
      final response = Completer<http.Response>();
      var requests = 0;
      await _mount(
        tester,
        respond: (_) {
          requests++;
          return response.future;
        },
      );
      await tester.pumpAndSettle();
      expect(requests, 0);
      await tester.tap(find.byKey(LeaveSpaceSheet.confirmKey));
      await tester.pump();
      expect(
        tester
            .widget<FilledButton>(find.byKey(LeaveSpaceSheet.confirmKey))
            .onPressed,
        isNull,
      );
      response.complete(
        http.Response(
          '{"error":"permission_denied","message":"private backend details"}',
          403,
        ),
      );
      await tester.pumpAndSettle();
      expect(requests, 1);
      expect(find.byKey(LeaveSpaceSheet.errorKey), findsOneWidget);
      expect(find.textContaining('private backend details'), findsNothing);
      expect(
        tester
            .widget<FilledButton>(find.byKey(LeaveSpaceSheet.confirmKey))
            .onPressed,
        isNotNull,
      );
    },
  );
}

Future<void> _mount(
  WidgetTester tester, {
  String locale = 'en',
  bool owner = false,
  bool column = false,
  required Future<http.Response> Function(http.Request) respond,
}) => tester.pumpWidget(
  ProviderScope(
    overrides: [
      ...voiceThemeTestOverrides(),
      for (final permission in [
        SpacePermissions.spaceManageBots,
        SpacePermissions.spaceManageInvites,
      ])
        spacePermissionProvider((
          spaceId: 'space-1',
          permission: permission,
          chatId: null,
          voiceRoomId: null,
        )).overrideWith((ref) async => false),
      spaceTreeProvider('space-1').overrideWith(
        (ref) async =>
            const SpaceTreeData(categories: [], nodes: [], voiceRooms: []),
      ),
      authorizationHeaderProvider.overrideWith((ref) => 'Bearer a'),
      spaceViewerProfileIdProvider.overrideWith(
        (ref) => owner ? 'owner' : 'member',
      ),
      spaceLeaveIdentityProvider.overrideWith(
        (ref) =>
            () => const GatewayRequestIdentity(
              generation: 0,
              accountId: 'a',
              profileId: 'member',
            ),
      ),
      spaceLeaveMediaCleanupProvider.overrideWith((ref) => (_, _) async {}),
      spaceProvider('space-1').overrideWith(
        (ref) async => const VoiceSpace(
          id: 'space-1',
          name: 'Raid HQ',
          visibility: 'private',
          ownerProfileId: 'owner',
        ),
      ),
      voiceSpacesClientProvider.overrideWith(
        (ref) => VoiceSpacesClient(
          gateway: GatewayHttpClient(
            config: const GatewayConfig(baseUrl: 'https://api.test'),
            httpClient: MockClient(respond),
          ),
        ),
      ),
    ],
    child: MaterialApp(
      theme: voiceTestTheme(),
      locale: Locale(locale),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: Scaffold(
        body: column
            ? SpaceTreeColumn(spaceId: 'space-1', onTextChatSelected: (_) {})
            : const LeaveSpaceSheet(spaceId: 'space-1'),
      ),
    ),
  ),
);
