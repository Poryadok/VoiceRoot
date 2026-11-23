// v3 Space Members controls (TT UIV3-PANEL-SPACE-MEMBERS): Members/Bans tab
// pair with search field, member and ban filters, ban audit rows with unban,
// member row tap to ProfileDetail, header counts, hidden-tab gating, optional
// ban reason. Kick/timeout/role dialog regressions live in
// space_members_sheet_test.dart.

import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/presence_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/ui/social/profile_detail_sheet.dart';
import 'package:voice_frontend/ui/space/space_members_sheet.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  const membersJson =
      '{"space_member_list":{"members":['
      '{"space_id":"space-1","profile_id":"member-1","role_names":["Owner"],'
      '"joined_at":{"secondsSinceEpoch":1730000000}},'
      '{"space_id":"space-1","profile_id":"member-2","role_names":["Admin"],'
      '"joined_at":{"secondsSinceEpoch":1730000000}},'
      '{"space_id":"space-1","profile_id":"member-3","role_names":["Member"],'
      '"nickname":"Nick","joined_at":{"secondsSinceEpoch":1730000000}}]}}';

  const bansJson =
      '{"ban_list":{"bans":['
      '{"space_id":"space-1","account_id":"account-9","banned_by_profile_id":"member-2",'
      '"reason":"repeat offender","banned_at":{"secondsSinceEpoch":1730246400}},'
      '{"space_id":"space-1","account_id":"account-5","banned_at":{"secondsSinceEpoch":1730160000}'
      '}]}}';

  final sampleMembers = [
    SpaceMemberRosterEntry(
      profileId: 'member-1',
      roleNames: const ['Owner'],
      joinedAt: DateTime.utc(2026, 1, 1),
    ),
    SpaceMemberRosterEntry(
      profileId: 'member-2',
      roleNames: const ['Admin'],
      joinedAt: DateTime.utc(2026, 1, 2),
    ),
    SpaceMemberRosterEntry(
      profileId: 'member-3',
      roleNames: const ['Member'],
      nickname: 'Nick',
      joinedAt: DateTime.utc(2026, 1, 3),
    ),
  ];

  final sampleBans = [
    SpaceBanData(
      accountId: 'account-9',
      bannedAt: DateTime.utc(2026, 2, 1),
      bannedByProfileId: 'member-2',
      reason: 'repeat offender',
    ),
    SpaceBanData(accountId: 'account-5', bannedAt: DateTime.utc(2026, 1, 20)),
  ];

  final profileMemberOne = const VoiceProfile(
    id: 'member-1',
    accountId: 'account-1',
    username: 'alpha',
    discriminator: '0001',
    displayName: 'Alpha One',
  );
  final profileMemberTwo = const VoiceProfile(
    id: 'member-2',
    accountId: 'account-2',
    username: 'bob',
    discriminator: '0002',
    displayName: 'Bob Two',
  );
  final profileMemberThree = const VoiceProfile(
    id: 'member-3',
    accountId: 'account-3',
    username: 'nickster',
    discriminator: '0003',
    displayName: 'Nick Name',
  );

  MockClient fetchClient(
    List<String> banGetPaths, {
    List<String> presencePosts = const [],
  }) => MockClient((request) async {
    if (request.method == 'GET' &&
        request.url.path == '/api/v1/spaces/space-1/bans') {
      banGetPaths.add(request.url.path);
      return http.Response(bansJson, 200);
    }
    if (request.method == 'GET' &&
        request.url.path == '/api/v1/spaces/space-1/members') {
      return http.Response(membersJson, 200);
    }
    if (request.method == 'POST' && request.url.path.contains('presence')) {
      presencePosts.add(request.url.path);
      return http.Response('{}', 200);
    }
    return http.Response('{}', 404);
  });

  MockClient mutateClient(
    List<String> banGetPaths,
    List<String> banPostBodies,
    List<String> banDeletePaths, {
    List<String> presencePosts = const [],
  }) => MockClient((request) async {
    if (request.method == 'GET' &&
        request.url.path == '/api/v1/spaces/space-1/bans') {
      banGetPaths.add(request.url.path);
      const bans = [
        {
          'space_id': 'space-1',
          'account_id': 'account-9',
          'banned_by_profile_id': 'member-2',
          'reason': 'repeat offender',
          'banned_at': {'secondsSinceEpoch': 1730246400},
        },
        {
          'space_id': 'space-1',
          'account_id': 'account-5',
          'banned_at': {'secondsSinceEpoch': 1730160000},
        },
      ];
      final live = bans.where((ban) {
        final path = '/api/v1/spaces/space-1/bans/${ban['account_id']}';
        return !banDeletePaths.contains(path);
      }).toList();
      return http.Response(
        jsonEncode({
          'ban_list': {'bans': live},
        }),
        200,
      );
    }
    if (request.method == 'GET' &&
        request.url.path == '/api/v1/spaces/space-1/members') {
      return http.Response(membersJson, 200);
    }
    if (request.method == 'POST' &&
        request.url.path == '/api/v1/spaces/space-1/bans') {
      banPostBodies.add(request.body);
      return http.Response('{}', 200);
    }
    if (request.method == 'DELETE' &&
        request.url.path.startsWith('/api/v1/spaces/space-1/bans/')) {
      banDeletePaths.add(request.url.path);
      return http.Response('{}', 200);
    }
    if (request.method == 'POST' && request.url.path.contains('presence')) {
      presencePosts.add(request.url.path);
      return http.Response('{}', 200);
    }
    return http.Response('{}', 404);
  });

  MockClient failingBanClient() {
    var fetches = 0;
    return MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/spaces/space-1/bans') {
        fetches++;
        if (fetches == 1) {
          return http.Response(
            '{"error":"private_backend_detail","message":"private_backend_detail"}',
            500,
          );
        }
        return http.Response(
          '{"ban_list":{"bans":['
          '{"space_id":"space-1","account_id":"account-9","banned_by_profile_id":"member-2",'
          '"reason":"repeat offender","banned_at":{"secondsSinceEpoch":1730246400}}]}}',
          200,
        );
      }
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/spaces/space-1/members') {
        return http.Response(membersJson, 200);
      }
      return http.Response('{}', 404);
    });
  }

  MockClient emptyBanClient() => MockClient((request) async {
    if (request.method == 'GET' &&
        request.url.path == '/api/v1/spaces/space-1/bans') {
      return http.Response('{"ban_list":{"bans":[]}}', 200);
    }
    if (request.method == 'GET' &&
        request.url.path == '/api/v1/spaces/space-1/members') {
      return http.Response(membersJson, 200);
    }
    return http.Response('{}', 404);
  });

  GatewayHttpClient gateway(http.Client client) => GatewayHttpClient(
    httpClient: client,
    config: const GatewayConfig(baseUrl: 'http://api.test'),
  );

  const sheetHome = Scaffold(body: SpaceMembersSheet(spaceId: 'space-1'));

  ProviderScope scope(List<Override> overrides) => ProviderScope(
    overrides: overrides,
    child: MaterialApp(
      theme: voiceTestTheme(),
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: sheetHome,
    ),
  );

  ProviderScope scopeRu(List<Override> overrides) => ProviderScope(
    overrides: overrides,
    child: MaterialApp(
      theme: voiceTestTheme(),
      locale: const Locale('ru'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: sheetHome,
    ),
  );

  List<Override> profileOverrides() => [
    profileProvider('member-1').overrideWith((ref) async => profileMemberOne),
    profileProvider('member-2').overrideWith((ref) async => profileMemberTwo),
    profileProvider('member-3').overrideWith((ref) async => profileMemberThree),
  ];

  testWidgets('tabs, search field, role badges and count render together', (
    tester,
  ) async {
    final banGetPaths = <String>[];
    final presencePosts = <String>[];
    await tester.pumpWidget(
      scope([
        ...voiceThemeTestOverrides(),
        gatewayHttpClientProvider.overrideWithValue(
          gateway(fetchClient(banGetPaths, presencePosts: presencePosts)),
        ),
        authorizationHeaderProvider.overrideWithValue('Bearer test'),
        spacePermissionProvider.overrideWith((ref, query) async => true),
        ...profileOverrides(),
      ]),
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizations.of(
      tester.element(find.byType(SpaceMembersSheet)),
    )!;
    expect(find.byKey(const Key('space_members_sheet_tabs')), findsOneWidget);
    expect(
      find.byKey(const Key('space_members_sheet_members_tab')),
      findsOneWidget,
    );
    expect(
      find.byKey(const Key('space_members_sheet_bans_tab')),
      findsOneWidget,
    );
    expect(find.text(l10n.spaceMembersBansTab), findsOneWidget);
    expect(find.byKey(SpaceMembersSheet.searchKey), findsOneWidget);
    expect(find.text('Alpha One'), findsOneWidget);
    expect(find.text('Nick Name'), findsOneWidget);
    expect(find.text('Owner'), findsWidgets);
    expect(find.text('Admin'), findsWidgets);
    expect(find.text('3'), findsOneWidget);
    expect(presencePosts, isEmpty);
  });

  testWidgets('tab tap switches to the bans audit list with unban', (
    tester,
  ) async {
    final banGetPaths = <String>[];
    final presencePosts = <String>[];
    await tester.pumpWidget(
      scope([
        ...voiceThemeTestOverrides(),
        gatewayHttpClientProvider.overrideWithValue(
          gateway(fetchClient(banGetPaths, presencePosts: presencePosts)),
        ),
        authorizationHeaderProvider.overrideWithValue('Bearer test'),
        spacePermissionProvider.overrideWith((ref, query) async => true),
        ...profileOverrides(),
      ]),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('space_members_sheet_bans_tab')));
    await tester.pumpAndSettle();
    final l10n = AppLocalizations.of(
      tester.element(find.byType(SpaceMembersSheet)),
    )!;
    expect(
      find.byKey(const Key('space_members_sheet_bans_body')),
      findsOneWidget,
    );
    expect(
      find.byKey(SpaceMembersSheet.unbanMemberKey('account-9')),
      findsOneWidget,
    );
    expect(find.text('account-9'), findsOneWidget);
    expect(find.textContaining('repeat offender'), findsOneWidget);
    expect(find.text(l10n.spaceMembersUnbanNotice), findsOneWidget);
    expect(find.text(l10n.spaceMembersBanCount(2)), findsOneWidget);
    expect(banGetPaths, ['/api/v1/spaces/space-1/bans']);
    expect(presencePosts, isEmpty);

    await tester.tap(find.byKey(const Key('space_members_sheet_members_tab')));
    await tester.pumpAndSettle();
    expect(
      find.byKey(const Key('space_members_sheet_bans_body')),
      findsNothing,
    );
    expect(find.text('Alpha One'), findsOneWidget);
    expect(banGetPaths, ['/api/v1/spaces/space-1/bans']);
  });

  testWidgets('hidden bans tab without MEMBER_BAN never fetches', (
    tester,
  ) async {
    final banGetPaths = <String>[];
    await tester.pumpWidget(
      scope([
        ...voiceThemeTestOverrides(),
        gatewayHttpClientProvider.overrideWithValue(
          gateway(fetchClient(banGetPaths)),
        ),
        authorizationHeaderProvider.overrideWithValue('Bearer test'),
        spacePermissionProvider.overrideWith(
          (ref, query) async => query.permission != 'MEMBER_BAN',
        ),
        ...profileOverrides(),
      ]),
    );
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('space_members_sheet_bans_tab')), findsNothing);
    expect(
      find.byKey(const Key('space_members_sheet_bans_body')),
      findsNothing,
    );
    expect(
      find.byKey(SpaceMembersSheet.banMemberKey('member-2')),
      findsNothing,
    );
    expect(
      find.byKey(SpaceMembersSheet.kickMemberKey('member-2')),
      findsOneWidget,
    );
    expect(banGetPaths, isEmpty);
  });

  testWidgets('member search keeps only matching roster rows', (tester) async {
    await tester.pumpWidget(
      scope([
        ...voiceThemeTestOverrides(),
        gatewayHttpClientProvider.overrideWithValue(
          gateway(fetchClient(<String>[])),
        ),
        authorizationHeaderProvider.overrideWithValue('Bearer test'),
        spacePermissionProvider.overrideWith((ref, query) async => true),
        ...profileOverrides(),
      ]),
    );
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(SpaceMembersSheet.searchKey), 'Nick');
    await tester.pumpAndSettle();
    expect(find.text('Nick Name'), findsOneWidget);
    expect(find.text('Alpha One'), findsNothing);
    expect(find.text('Bob Two'), findsNothing);
  });

  testWidgets('member search without matches shows the recovery panel', (
    tester,
  ) async {
    await tester.pumpWidget(
      scope([
        ...voiceThemeTestOverrides(),
        gatewayHttpClientProvider.overrideWithValue(
          gateway(fetchClient(<String>[])),
        ),
        authorizationHeaderProvider.overrideWithValue('Bearer test'),
        spacePermissionProvider.overrideWith((ref, query) async => true),
        ...profileOverrides(),
      ]),
    );
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(SpaceMembersSheet.searchKey), 'zzz');
    await tester.pumpAndSettle();
    final l10n = AppLocalizations.of(
      tester.element(find.byType(SpaceMembersSheet)),
    )!;
    expect(
      find.byKey(const Key('space_members_sheet_search_no_matches')),
      findsOneWidget,
    );
    expect(find.text(l10n.spaceMembersSearchNoMatches), findsOneWidget);
  });

  testWidgets('bans search filters audit rows', (tester) async {
    await tester.pumpWidget(
      scope([
        ...voiceThemeTestOverrides(),
        gatewayHttpClientProvider.overrideWithValue(
          gateway(fetchClient(<String>[])),
        ),
        authorizationHeaderProvider.overrideWithValue('Bearer test'),
        spacePermissionProvider.overrideWith((ref, query) async => true),
        ...profileOverrides(),
      ]),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('space_members_sheet_bans_tab')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(SpaceMembersSheet.searchKey), 'repeat');
    await tester.pumpAndSettle();
    expect(find.text('account-9'), findsOneWidget);
    expect(find.text('account-5'), findsNothing);
    expect(find.textContaining('repeat offender'), findsOneWidget);
  });

  testWidgets('bans search without matches shows the audit panel', (
    tester,
  ) async {
    await tester.pumpWidget(
      scope([
        ...voiceThemeTestOverrides(),
        gatewayHttpClientProvider.overrideWithValue(
          gateway(fetchClient(<String>[])),
        ),
        authorizationHeaderProvider.overrideWithValue('Bearer test'),
        spacePermissionProvider.overrideWith((ref, query) async => true),
        ...profileOverrides(),
      ]),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('space_members_sheet_bans_tab')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(SpaceMembersSheet.searchKey), 'zzz');
    await tester.pumpAndSettle();
    final l10n = AppLocalizations.of(
      tester.element(find.byType(SpaceMembersSheet)),
    )!;
    expect(
      find.byKey(const Key('space_members_sheet_bans_no_matches')),
      findsOneWidget,
    );
    expect(find.text(l10n.spaceMembersBansSearchNoMatches), findsOneWidget);
  });

  testWidgets('empty bans audit shows the clean panel', (tester) async {
    await tester.pumpWidget(
      scope([
        ...voiceThemeTestOverrides(),
        gatewayHttpClientProvider.overrideWithValue(gateway(emptyBanClient())),
        authorizationHeaderProvider.overrideWithValue('Bearer test'),
        spacePermissionProvider.overrideWith((ref, query) async => true),
        ...profileOverrides(),
      ]),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('space_members_sheet_bans_tab')));
    await tester.pumpAndSettle();
    final l10n = AppLocalizations.of(
      tester.element(find.byType(SpaceMembersSheet)),
    )!;
    expect(
      find.byKey(const Key('space_members_sheet_bans_empty')),
      findsOneWidget,
    );
    // The zero ban-count header and the empty panel show the same copy, so pin
    // the empty copy inside the panel rather than counting global occurrences.
    expect(
      find.descendant(
        of: find.byKey(const Key('space_members_sheet_bans_empty')),
        matching: find.text(l10n.spaceMembersBansEmpty),
      ),
      findsOneWidget,
    );
  });

  testWidgets('bans load failure hides details and retry refetches', (
    tester,
  ) async {
    await tester.pumpWidget(
      scope([
        ...voiceThemeTestOverrides(),
        gatewayHttpClientProvider.overrideWithValue(
          gateway(failingBanClient()),
        ),
        authorizationHeaderProvider.overrideWithValue('Bearer test'),
        spacePermissionProvider.overrideWith((ref, query) async => true),
        ...profileOverrides(),
      ]),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('space_members_sheet_bans_tab')));
    await tester.pumpAndSettle();
    final l10n = AppLocalizations.of(
      tester.element(find.byType(SpaceMembersSheet)),
    )!;
    expect(
      find.byKey(const Key('space_members_sheet_bans_error')),
      findsOneWidget,
    );
    expect(find.text(l10n.spaceMembersBansLoadError), findsAtLeastNWidgets(2));
    expect(find.textContaining('private_backend_detail'), findsNothing);
    await tester.tap(find.byType(OutlinedButton));
    await tester.pumpAndSettle();
    expect(find.text('account-9'), findsOneWidget);
  });

  testWidgets('ban confirm can attach an optional reason', (tester) async {
    final banPostBodies = <String>[];
    final banGetPaths = <String>[];
    await tester.pumpWidget(
      scope([
        ...voiceThemeTestOverrides(),
        gatewayHttpClientProvider.overrideWithValue(
          gateway(mutateClient(banGetPaths, banPostBodies, <String>[])),
        ),
        authorizationHeaderProvider.overrideWithValue('Bearer test'),
        spacePermissionProvider.overrideWith((ref, query) async => true),
        ...profileOverrides(),
      ]),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(SpaceMembersSheet.banMemberKey('member-2')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('space_ban_dialog_reason')), findsOneWidget);
    await tester.enterText(
      find.byKey(const Key('space_ban_dialog_reason')),
      '  repeat offender ',
    );
    await tester.tap(find.byType(FilledButton).last);
    await tester.pumpAndSettle();
    expect(banPostBodies, [
      '{"account_id":"account-2","profile_id":"member-2","reason":"repeat offender"}',
    ]);
    expect(banGetPaths, hasLength(2));
  });

  testWidgets('ban confirm without reason posts account only', (tester) async {
    final banPostBodies = <String>[];
    await tester.pumpWidget(
      scope([
        ...voiceThemeTestOverrides(),
        gatewayHttpClientProvider.overrideWithValue(
          gateway(mutateClient(<String>[], banPostBodies, <String>[])),
        ),
        authorizationHeaderProvider.overrideWithValue('Bearer test'),
        spacePermissionProvider.overrideWith((ref, query) async => true),
        ...profileOverrides(),
      ]),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(SpaceMembersSheet.banMemberKey('member-2')));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(FilledButton).last);
    await tester.pumpAndSettle();
    expect(banPostBodies, [
      '{"account_id":"account-2","profile_id":"member-2"}',
    ]);
  });

  testWidgets('ban confirm cancel posts nothing', (tester) async {
    final banPostBodies = <String>[];
    final banGetPaths = <String>[];
    await tester.pumpWidget(
      scope([
        ...voiceThemeTestOverrides(),
        gatewayHttpClientProvider.overrideWithValue(
          gateway(mutateClient(banGetPaths, banPostBodies, <String>[])),
        ),
        authorizationHeaderProvider.overrideWithValue('Bearer test'),
        spacePermissionProvider.overrideWith((ref, query) async => true),
        ...profileOverrides(),
      ]),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(SpaceMembersSheet.banMemberKey('member-2')));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(TextButton).last);
    await tester.pumpAndSettle();
    expect(banPostBodies, isEmpty);
    expect(banGetPaths, hasLength(1));
  });

  testWidgets('member row tap opens the ProfileDetail destination', (
    tester,
  ) async {
    await tester.pumpWidget(
      scope([
        ...voiceThemeTestOverrides(),
        gatewayHttpClientProvider.overrideWithValue(
          gateway(fetchClient(<String>[])),
        ),
        authorizationHeaderProvider.overrideWithValue('Bearer test'),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        guestCredentialsStorageProvider.overrideWithValue(
          InMemoryGuestCredentialsStorage(),
        ),
        spacePermissionProvider.overrideWith((ref, query) async => true),
        spaceMembersProvider(
          'space-1',
        ).overrideWith((ref) async => sampleMembers),
        profileProvider(
          'member-1',
        ).overrideWith((ref) async => profileMemberOne),
        presenceProvider('member-1').overrideWithValue(null),
      ]),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('member_profile_open_member-1')));
    await tester.pumpAndSettle();
    expect(find.byType(BottomSheet), findsOneWidget);
    expect(find.byType(ProfileDetailSheet), findsOneWidget);
  });

  testWidgets('unban removes the ban row and refreshes the audit list', (
    tester,
  ) async {
    final banGetPaths = <String>[];
    final banDeletePaths = <String>[];
    await tester.pumpWidget(
      scope([
        ...voiceThemeTestOverrides(),
        gatewayHttpClientProvider.overrideWithValue(
          gateway(mutateClient(banGetPaths, <String>[], banDeletePaths)),
        ),
        authorizationHeaderProvider.overrideWithValue('Bearer test'),
        spacePermissionProvider.overrideWith((ref, query) async => true),
        ...profileOverrides(),
      ]),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('space_members_sheet_bans_tab')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(SpaceMembersSheet.unbanMemberKey('account-9')));
    await tester.pumpAndSettle();
    expect(banDeletePaths, ['/api/v1/spaces/space-1/bans/account-9']);
    expect(banGetPaths, hasLength(2));
    expect(find.text('account-9'), findsNothing);
    expect(find.text('account-5'), findsOneWidget);
  });

  testWidgets('unban without auth keeps the list and uses safe copy', (
    tester,
  ) async {
    await tester.pumpWidget(
      scope([
        ...voiceThemeTestOverrides(),
        gatewayHttpClientProvider.overrideWithValue(
          gateway(fetchClient(<String>[])),
        ),
        authorizationHeaderProvider.overrideWithValue(null),
        spacePermissionProvider.overrideWith((ref, query) async => true),
        spaceBansProvider('space-1').overrideWith((ref) async => sampleBans),
        ...profileOverrides(),
      ]),
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizations.of(
      tester.element(find.byType(SpaceMembersSheet)),
    )!;
    await tester.tap(find.byKey(const Key('space_members_sheet_bans_tab')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(SpaceMembersSheet.unbanMemberKey('account-9')));
    await tester.pumpAndSettle();
    expect(
      find.text(l10n.spaceMembersUnbanError(l10n.commonActionFailed)),
      findsOneWidget,
    );
    expect(find.textContaining('not_authenticated'), findsNothing);
    expect(find.text('account-9'), findsOneWidget);
  });

  for (final language in ['en', 'ru']) {
    testWidgets('$language ban confirm dialog and reason hint are localized', (
      tester,
    ) async {
      await tester.pumpWidget(
        (language == 'en' ? scope : scopeRu)([
          ...voiceThemeTestOverrides(),
          gatewayHttpClientProvider.overrideWithValue(
            gateway(fetchClient(<String>[])),
          ),
          authorizationHeaderProvider.overrideWithValue('Bearer test'),
          spacePermissionProvider.overrideWith((ref, query) async => true),
          ...profileOverrides(),
        ]),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(SpaceMembersSheet.banMemberKey('member-2')));
      await tester.pumpAndSettle();
      final l10n = AppLocalizations.of(
        tester.element(find.byType(SpaceMembersSheet)),
      )!;
      expect(find.text(l10n.spaceBanConfirmTitle), findsOneWidget);
      expect(find.text(l10n.spaceBanReasonHint), findsOneWidget);
    });
  }
}
