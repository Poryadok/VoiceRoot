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
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/ui/space/join_space_invite_sheet.dart';
import 'package:voice_frontend/ui/space/space_invites_sheet.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  final sampleInvites = [
    SpaceInvite(
      id: 'inv-1',
      spaceId: 'space-1',
      code: 'abc123',
      creatorProfileId: 'owner',
      useCount: 1,
      maxUses: 5,
      createdAt: DateTime.utc(2026, 1, 1),
    ),
  ];

  testWidgets('SpaceInvitesSheet lists invites with copy and revoke actions', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spaceInvitesProvider(
            'space-1',
          ).overrideWith((ref) async => sampleInvites),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: SpaceInvitesSheet(spaceId: 'space-1')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('abc123'), findsOneWidget);
    expect(find.byKey(const Key('copy_invite_inv-1')), findsOneWidget);
    expect(find.byKey(const Key('revoke_invite_inv-1')), findsOneWidget);
    expect(find.byKey(SpaceInvitesSheet.createButtonKey), findsOneWidget);
  });

  for (final revoke in [false, true]) {
    testWidgets('${revoke ? 'revoke' : 'create'} invite hides API diagnostics', (
      tester,
    ) async {
      var mutation = 0;
      final gateway = GatewayHttpClient(
        httpClient: MockClient((request) async {
          if (request.method == 'POST' || request.method == 'DELETE') {
            mutation++;
            return http.Response(
              '{"error":"private_backend_detail","message":"private_backend_detail"}',
              500,
            );
          }
          return http.Response('{}', 200);
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            ...voiceThemeTestOverrides(),
            gatewayHttpClientProvider.overrideWithValue(gateway),
            authorizationHeaderProvider.overrideWithValue('Bearer test'),
            spaceInvitesProvider(
              'space-1',
            ).overrideWith((ref) async => sampleInvites),
          ],
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: SpaceInvitesSheet(spaceId: 'space-1')),
          ),
        ),
      );
      await tester.pumpAndSettle();
      final l10n = AppLocalizations.of(
        tester.element(find.byType(SpaceInvitesSheet)),
      )!;
      if (revoke) {
        await tester.tap(find.byKey(const Key('revoke_invite_inv-1')));
      } else {
        await tester.tap(find.byKey(SpaceInvitesSheet.createButtonKey));
      }
      await tester.pumpAndSettle();
      expect(mutation, 1);
      expect(find.textContaining('private_backend_detail'), findsNothing);
      expect(find.text(l10n.commonActionFailed), findsOneWidget);
    });
  }

  testWidgets('unauthenticated create retains its local action copy', (
    tester,
  ) async {
    var requestCount = 0;
    final gateway = GatewayHttpClient(
      httpClient: MockClient((request) async {
        requestCount++;
        return http.Response('{}', 200);
      }),
      config: const GatewayConfig(baseUrl: 'http://api.test'),
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          gatewayHttpClientProvider.overrideWithValue(gateway),
          authorizationHeaderProvider.overrideWithValue(null),
          spaceInvitesProvider(
            'space-1',
          ).overrideWith((ref) async => sampleInvites),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: SpaceInvitesSheet(spaceId: 'space-1')),
        ),
      ),
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizations.of(
      tester.element(find.byType(SpaceInvitesSheet)),
    )!;
    await tester.tap(find.byKey(SpaceInvitesSheet.createButtonKey));
    await tester.pumpAndSettle();
    expect(
      find.text(l10n.spaceInviteCreateError('not_authenticated')),
      findsOneWidget,
    );
    expect(requestCount, 0);
  });

  testWidgets('invalid max uses stays local and sends no request', (
    tester,
  ) async {
    var mutationCount = 0;
    final gateway = GatewayHttpClient(
      httpClient: MockClient((request) async {
        if (request.method == 'POST' || request.method == 'DELETE') {
          mutationCount++;
        }
        return http.Response('{}', 200);
      }),
      config: const GatewayConfig(baseUrl: 'http://api.test'),
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          gatewayHttpClientProvider.overrideWithValue(gateway),
          authorizationHeaderProvider.overrideWithValue('Bearer test'),
          spaceInvitesProvider(
            'space-1',
          ).overrideWith((ref) async => sampleInvites),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: SpaceInvitesSheet(spaceId: 'space-1')),
        ),
      ),
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizations.of(
      tester.element(find.byType(SpaceInvitesSheet)),
    )!;
    await tester.tap(find.text(l10n.spaceInviteAdvancedToggle));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(SpaceInvitesSheet.maxUsesFieldKey), '0');
    await tester.tap(find.byKey(SpaceInvitesSheet.createButtonKey));
    await tester.pumpAndSettle();
    expect(mutationCount, 0);
    expect(find.text(l10n.spaceInviteMaxUsesInvalid), findsOneWidget);
  });

  testWidgets('join invite hides API diagnostics and keeps the form open', (
    tester,
  ) async {
    var requestCount = 0;
    String? requestedCode;
    final gateway = GatewayHttpClient(
      httpClient: MockClient((request) async {
        requestCount++;
        requestedCode =
            request.url.pathSegments[request.url.pathSegments.length - 2];
        return http.Response(
          '{"error":"private_join_detail","message":"private_join_detail"}',
          500,
        );
      }),
      config: const GatewayConfig(baseUrl: 'http://api.test'),
    );
    final container = ProviderContainer(
      overrides: [
        ...voiceThemeTestOverrides(),
        gatewayHttpClientProvider.overrideWithValue(gateway),
        authorizationHeaderProvider.overrideWithValue('Bearer test'),
      ],
    );
    addTearDown(container.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: JoinSpaceInviteSheet()),
        ),
      ),
    );
    final l10n = AppLocalizations.of(
      tester.element(find.byType(JoinSpaceInviteSheet)),
    )!;
    await tester.enterText(
      find.byKey(JoinSpaceInviteSheet.codeFieldKey),
      'join-code-42',
    );
    await tester.tap(find.byKey(JoinSpaceInviteSheet.submitKey));
    await tester.pumpAndSettle();

    expect(requestCount, 1);
    expect(requestedCode, 'join-code-42');
    expect(find.byType(JoinSpaceInviteSheet), findsOneWidget);
    expect(container.read(selectedSpaceIdProvider), isNull);
    expect(find.textContaining('private_join_detail'), findsNothing);
    expect(
      find.text(l10n.spaceInviteJoinError(l10n.commonActionFailed)),
      findsOneWidget,
    );
  });

  testWidgets('join invite keeps the local unauthenticated message', (
    tester,
  ) async {
    var requestCount = 0;
    final gateway = GatewayHttpClient(
      httpClient: MockClient((request) async {
        requestCount++;
        return http.Response('{}', 200);
      }),
      config: const GatewayConfig(baseUrl: 'http://api.test'),
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          gatewayHttpClientProvider.overrideWithValue(gateway),
          authorizationHeaderProvider.overrideWithValue(null),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: JoinSpaceInviteSheet()),
        ),
      ),
    );
    final l10n = AppLocalizations.of(
      tester.element(find.byType(JoinSpaceInviteSheet)),
    )!;
    await tester.enterText(
      find.byKey(JoinSpaceInviteSheet.codeFieldKey),
      'join-code-42',
    );
    await tester.tap(find.byKey(JoinSpaceInviteSheet.submitKey));
    await tester.pumpAndSettle();

    expect(requestCount, 0);
    expect(find.byType(JoinSpaceInviteSheet), findsOneWidget);
    expect(
      find.text(l10n.spaceInviteJoinError('not_authenticated')),
      findsOneWidget,
    );
  });
}
