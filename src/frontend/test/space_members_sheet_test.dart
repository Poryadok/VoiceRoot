import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/ui/space/space_members_sheet.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  final sampleMembers = [
    SpaceMemberRosterEntry(
      profileId: 'owner',
      roleNames: const ['Owner'],
      joinedAt: DateTime.utc(2026, 1, 1),
    ),
    SpaceMemberRosterEntry(
      profileId: 'member-1',
      roleNames: const ['Member'],
      joinedAt: DateTime.utc(2026, 1, 2),
    ),
  ];

  testWidgets('SpaceMembersSheet lists members with role badges', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spaceMembersProvider(
            'space-1',
          ).overrideWith((ref) async => sampleMembers),
          spacePermissionProvider.overrideWith((ref, query) async => true),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: SpaceMembersSheet(spaceId: 'space-1')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Owner'), findsOneWidget);
    expect(find.text('Member'), findsWidgets);
    expect(find.byKey(const Key('kick_member_member-1')), findsOneWidget);
    expect(find.byKey(const Key('assign_role_member-1')), findsOneWidget);
    expect(find.byKey(const Key('ban_member_member-1')), findsOneWidget);
    expect(find.byKey(const Key('timeout_member_member-1')), findsOneWidget);
  });

  for (final action in [
    (
      key: SpaceMembersSheet.banMemberKey('member-1'),
      name: 'ban',
      suffix: '/bans',
    ),
    (
      key: SpaceMembersSheet.timeoutMemberKey('member-1'),
      name: 'timeout',
      suffix: '/members/member-1/timeout',
    ),
    (
      key: SpaceMembersSheet.kickMemberKey('member-1'),
      name: 'kick',
      suffix: '/members/member-1',
    ),
  ]) {
    testWidgets('${action.name} failure hides API diagnostics', (tester) async {
      final mutationPaths = <String>[];
      final gateway = GatewayHttpClient(
        httpClient: MockClient((request) async {
          if (request.method != 'GET') {
            mutationPaths.add(request.url.path);
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
            spaceMembersProvider(
              'space-1',
            ).overrideWith((ref) async => sampleMembers),
            spacePermissionProvider.overrideWith((ref, query) async => true),
            profileProvider('member-1').overrideWith(
              (ref) async => const VoiceProfile(
                id: 'member-1',
                accountId: 'account-1',
                username: 'member',
                discriminator: '0001',
                displayName: 'Member',
              ),
            ),
          ],
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: SpaceMembersSheet(spaceId: 'space-1')),
          ),
        ),
      );
      await tester.pumpAndSettle();
      final l10n = AppLocalizations.of(
        tester.element(find.byType(SpaceMembersSheet)),
      )!;
      await tester.tap(find.byKey(action.key));
      await tester.pumpAndSettle();
      await tester.tap(find.byType(FilledButton).last);
      await tester.pumpAndSettle();
      expect(mutationPaths, ['/api/v1/spaces/space-1${action.suffix}']);
      expect(find.textContaining('private_backend_detail'), findsNothing);
      expect(find.text(l10n.commonActionFailed), findsOneWidget);
    });
  }
}
