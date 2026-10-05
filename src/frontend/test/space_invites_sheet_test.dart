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
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/space_permissions.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/ui/space/join_space_invite_sheet.dart';
import 'package:voice_frontend/ui/space/space_invites_sheet.dart';
import 'package:voice_frontend/ui/api_error_messages.dart';
import 'package:qr_flutter/qr_flutter.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const _spaceInviteCaptureRootKey = Key('space_invite_capture_root');
var _spaceInviteCaptureFontsLoaded = false;

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
  final expiringInvites = [
    SpaceInvite(
      id: 'inv-expiring',
      spaceId: 'space-1',
      code: 'expiry-code',
      creatorProfileId: 'owner',
      useCount: 0,
      createdAt: DateTime.utc(2026, 1, 1),
      expiresAt: DateTime.utc(2030, 1, 2, 3, 4),
    ),
  ];

  testWidgets('SpaceInvitesSheet lists invites with copy and revoke actions', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spacePermissionProvider((
            spaceId: 'space-1',
            permission: SpacePermissions.spaceManageInvites,
            chatId: null,
            voiceRoomId: null,
          )).overrideWith((ref) async => true),
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

  testWidgets('direct invite sheet access hides controls without permission', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spacePermissionProvider((
            spaceId: 'space-1',
            permission: SpacePermissions.spaceManageInvites,
            chatId: null,
            voiceRoomId: null,
          )).overrideWith((ref) async => false),
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

    expect(find.byKey(SpaceInvitesSheet.createButtonKey), findsNothing);
    expect(find.byKey(const Key('revoke_invite_inv-1')), findsNothing);
    expect(
      find.text(l10n.spacePermissionDeniedGeneric('Space Manage Invites')),
      findsOneWidget,
    );
  });

  testWidgets(
    'expiry picker exposes the documented options and sends UTC expiry',
    (tester) async {
      String? requestBody;
      final gateway = GatewayHttpClient(
        httpClient: MockClient((request) async {
          if (request.method == 'POST') requestBody = request.body;
          return http.Response('{}', 200);
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            ...voiceThemeTestOverrides(),
            authorizationHeaderProvider.overrideWithValue('Bearer test'),
            gatewayHttpClientProvider.overrideWithValue(gateway),
            spacePermissionProvider((
              spaceId: 'space-1',
              permission: SpacePermissions.spaceManageInvites,
              chatId: null,
              voiceRoomId: null,
            )).overrideWith((ref) async => true),
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

      await tester.tap(find.byKey(const Key('space_invite_expiry_field')));
      await tester.pumpAndSettle();
      for (final choice in ['30m', '1h', '6h', '12h', '1d', '7d', 'never']) {
        expect(
          find.byKey(Key('space_invite_expiry_choice_$choice')),
          findsAtLeastNWidgets(1),
        );
      }
      await tester.tap(find.byKey(const Key('space_invite_expiry_choice_6h')));
      await tester.pumpAndSettle();
      expect(find.text('6 hours'), findsOneWidget);

      final beforeSubmit = DateTime.now().toUtc();
      await tester.tap(find.byKey(SpaceInvitesSheet.createButtonKey));
      await tester.pumpAndSettle();
      final afterSubmit = DateTime.now().toUtc();

      expect(requestBody, isNotNull);
      final body = jsonDecode(requestBody!) as Map<String, dynamic>;
      final timestamp = body['expires_at'] as Map<String, dynamic>;
      final seconds = int.parse(timestamp['seconds'] as String);
      final nanos = timestamp['nanos'] as int;
      final expiresAt = DateTime.fromMillisecondsSinceEpoch(
        seconds * Duration.millisecondsPerSecond + nanos ~/ 1000000,
        isUtc: true,
      );
      expect(
        expiresAt.isAfter(beforeSubmit.add(const Duration(hours: 6))),
        isTrue,
      );
      expect(
        expiresAt.isBefore(afterSubmit.add(const Duration(hours: 6))),
        isTrue,
      );
    },
  );

  testWidgets('invite row displays its expiry in the current locale', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spacePermissionProvider((
            spaceId: 'space-1',
            permission: SpacePermissions.spaceManageInvites,
            chatId: null,
            voiceRoomId: null,
          )).overrideWith((ref) async => true),
          spaceInvitesProvider(
            'space-1',
          ).overrideWith((ref) async => expiringInvites),
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
    final context = tester.element(find.byType(SpaceInvitesSheet));
    final material = MaterialLocalizations.of(context);
    final localExpiry = expiringInvites.single.expiresAt!.toLocal();
    final formatted =
        '${material.formatMediumDate(localExpiry)} '
        '${material.formatTimeOfDay(TimeOfDay.fromDateTime(localExpiry))}';

    expect(
      find.textContaining(
        AppLocalizations.of(context)!.spaceInviteExpiresAt(formatted),
      ),
      findsOneWidget,
    );
  });

  testWidgets('QR shows the usable invite URL and row copy copies that URL', (
    tester,
  ) async {
    String? copiedText;
    final messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    messenger.setMockMethodCallHandler(SystemChannels.platform, (call) async {
      if (call.method == 'Clipboard.setData') {
        copiedText =
            (call.arguments as Map<Object?, Object?>)['text'] as String?;
      }
      return null;
    });
    addTearDown(
      () => messenger.setMockMethodCallHandler(SystemChannels.platform, null),
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spacePermissionProvider((
            spaceId: 'space-1',
            permission: SpacePermissions.spaceManageInvites,
            chatId: null,
            voiceRoomId: null,
          )).overrideWith((ref) async => true),
          spaceInvitesProvider(
            'space-1',
          ).overrideWith((ref) async => expiringInvites),
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
    final expectedLink = expiringInvites.single.inviteLink;
    final parsedLink = Uri.parse(expectedLink);
    expect(parsedLink.hasScheme, isTrue);
    expect(parsedLink.host, isNotEmpty);
    expect(parsedLink.pathSegments, contains('invite'));
    expect(parsedLink.pathSegments, contains('expiry-code'));

    await tester.tap(find.byKey(const Key('qr_invite_inv-expiring')));
    await tester.pumpAndSettle();
    expect(find.byType(QrImageView), findsOneWidget);
    expect(find.text(expectedLink), findsOneWidget);
    final semantics = tester.widget<Semantics>(
      find
          .ancestor(
            of: find.byKey(const Key('space_invite_qr_image')),
            matching: find.byType(Semantics),
          )
          .first,
    );
    expect(
      semantics.properties.label,
      AppLocalizations.of(
        tester.element(find.byType(SpaceInvitesSheet)),
      )!.spaceInviteQrSemanticLabel,
    );
    await tester.tap(find.byKey(const Key('space_invite_qr_close')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('copy_invite_inv-expiring')));
    await tester.pumpAndSettle();

    expect(copiedText, expectedLink);
  });

  testWidgets('revoke requires confirmation; cancel leaves the invite active', (
    tester,
  ) async {
    var deletes = 0;
    final gateway = GatewayHttpClient(
      httpClient: MockClient((request) async {
        if (request.method == 'DELETE') deletes++;
        return http.Response('', 204);
      }),
      config: const GatewayConfig(baseUrl: 'http://api.test'),
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          authorizationHeaderProvider.overrideWithValue('Bearer test'),
          gatewayHttpClientProvider.overrideWithValue(gateway),
          spacePermissionProvider((
            spaceId: 'space-1',
            permission: SpacePermissions.spaceManageInvites,
            chatId: null,
            voiceRoomId: null,
          )).overrideWith((ref) async => true),
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

    await tester.tap(find.byKey(const Key('revoke_invite_inv-1')));
    await tester.pumpAndSettle();
    expect(find.text(l10n.spaceInviteRevokeConfirmTitle), findsOneWidget);
    expect(find.text(l10n.spaceInviteRevokeConfirmBody), findsOneWidget);
    await tester.tap(find.byKey(const Key('space_invite_revoke_cancel')));
    await tester.pumpAndSettle();
    expect(deletes, 0);
    expect(find.byKey(const Key('space_invite_inv-1')), findsOneWidget);

    await tester.tap(find.byKey(const Key('revoke_invite_inv-1')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('space_invite_revoke_confirm')));
    await tester.pumpAndSettle();
    expect(deletes, 1);
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
            spacePermissionProvider((
              spaceId: 'space-1',
              permission: SpacePermissions.spaceManageInvites,
              chatId: null,
              voiceRoomId: null,
            )).overrideWith((ref) async => true),
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
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('space_invite_revoke_confirm')));
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
          spacePermissionProvider((
            spaceId: 'space-1',
            permission: SpacePermissions.spaceManageInvites,
            chatId: null,
            voiceRoomId: null,
          )).overrideWith((ref) async => true),
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
          spacePermissionProvider((
            spaceId: 'space-1',
            permission: SpacePermissions.spaceManageInvites,
            chatId: null,
            voiceRoomId: null,
          )).overrideWith((ref) async => true),
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

  testWidgets('create rechecks invite permission before sending the request', (
    tester,
  ) async {
    var permissionChecks = 0;
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
          authorizationHeaderProvider.overrideWithValue('Bearer test'),
          gatewayHttpClientProvider.overrideWithValue(gateway),
          spacePermissionProvider((
            spaceId: 'space-1',
            permission: SpacePermissions.spaceManageInvites,
            chatId: null,
            voiceRoomId: null,
          )).overrideWith((ref) async => ++permissionChecks == 1),
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
    expect(find.byKey(SpaceInvitesSheet.createButtonKey), findsOneWidget);

    await tester.tap(find.byKey(SpaceInvitesSheet.createButtonKey));
    await tester.pumpAndSettle();

    expect(permissionChecks, 2);
    expect(mutationCount, 0);
  });

  testWidgets('permission recheck errors fail closed with neutral copy', (
    tester,
  ) async {
    var permissionChecks = 0;
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
          authorizationHeaderProvider.overrideWithValue('Bearer test'),
          gatewayHttpClientProvider.overrideWithValue(gateway),
          spacePermissionProvider((
            spaceId: 'space-1',
            permission: SpacePermissions.spaceManageInvites,
            chatId: null,
            voiceRoomId: null,
          )).overrideWith((ref) async {
            if (++permissionChecks == 1) return true;
            throw StateError('permission transport detail');
          }),
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

    expect(permissionChecks, 2);
    expect(mutationCount, 0);
    expect(find.text(commonActionErrorMessage(l10n)), findsAtLeastNWidgets(1));
    expect(find.textContaining('permission transport detail'), findsNothing);
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

  testWidgets('invite expiry, QR, and revoke journey is keyboard accessible', (
    tester,
  ) async {
    var deletes = 0;
    final gateway = GatewayHttpClient(
      httpClient: MockClient((request) async {
        if (request.method == 'DELETE') deletes++;
        return http.Response('', 204);
      }),
      config: const GatewayConfig(baseUrl: 'http://api.test'),
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          authorizationHeaderProvider.overrideWithValue('Bearer test'),
          gatewayHttpClientProvider.overrideWithValue(gateway),
          spacePermissionProvider((
            spaceId: 'space-1',
            permission: SpacePermissions.spaceManageInvites,
            chatId: null,
            voiceRoomId: null,
          )).overrideWith((ref) async => true),
          spaceInvitesProvider(
            'space-1',
          ).overrideWith((ref) async => expiringInvites),
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

    final expiryFinder = find.byKey(const Key('space_invite_expiry_field'));
    expect(find.byTooltip(l10n.spaceInviteExpiryLabel), findsOneWidget);
    final expiryInvokerFocus = FocusManager.instance.primaryFocus;
    await tester.tap(expiryFinder);
    await tester.pumpAndSettle();
    expect(find.text(l10n.spaceInviteExpiry30Minutes), findsOneWidget);
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pumpAndSettle();
    expect(find.text(l10n.spaceInviteExpiry30Minutes), findsNothing);
    expect(FocusManager.instance.primaryFocus, same(expiryInvokerFocus));
    await tester.tap(expiryFinder);
    await tester.pumpAndSettle();
    expect(
      find.byKey(const Key('space_invite_expiry_choice_30m')),
      findsOneWidget,
    );
    await tester.sendKeyEvent(LogicalKeyboardKey.arrowDown);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();
    expect(find.text(l10n.spaceInviteExpiry30Minutes), findsOneWidget);

    final qrFinder = find.byKey(const Key('qr_invite_inv-expiring'));
    await tester.tap(qrFinder);
    await tester.pumpAndSettle();
    expect(find.byType(QrImageView), findsOneWidget);
    final qrDialogFocus = FocusManager.instance.primaryFocus;
    expect(qrDialogFocus, isNotNull);
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pumpAndSettle();
    expect(find.byType(QrImageView), findsNothing);
    expect(FocusManager.instance.primaryFocus, isNot(same(qrDialogFocus)));
    expect(FocusManager.instance.primaryFocus?.context, isNotNull);

    final revokeFinder = find.byKey(const Key('revoke_invite_inv-expiring'));
    await tester.tap(revokeFinder);
    await tester.pumpAndSettle();
    expect(find.text(l10n.spaceInviteRevokeConfirmTitle), findsOneWidget);
    final revokeDialogFocus = FocusManager.instance.primaryFocus;
    expect(revokeDialogFocus, isNotNull);
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pumpAndSettle();
    expect(find.text(l10n.spaceInviteRevokeConfirmTitle), findsNothing);
    expect(deletes, 0);
    expect(FocusManager.instance.primaryFocus, isNot(same(revokeDialogFocus)));
    expect(FocusManager.instance.primaryFocus?.context, isNotNull);

    await tester.tap(revokeFinder);
    await tester.pumpAndSettle();
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();
    expect(deletes, 1);
  });

  testWidgets('captures invite controls at vertical and horizontal sizes', (
    tester,
  ) async {
    final captureDirectory =
        Platform.environment['VOICE_SPACE_INVITES_CAPTURE_DIR'];
    final captureEnabled =
        captureDirectory != null && captureDirectory.isNotEmpty;
    final theme = captureEnabled
        ? await _spaceInviteCaptureTheme(tester)
        : voiceTestTheme();
    final gateway = GatewayHttpClient(
      httpClient: MockClient((request) async => http.Response('', 204)),
      config: const GatewayConfig(baseUrl: 'http://api.test'),
    );
    final launcherFocus = FocusNode(debugLabel: 'space invite launcher');
    addTearDown(launcherFocus.dispose);

    Future<void> captureAt(Size size, String filename) async {
      tester.view.physicalSize = size;
      tester.view.devicePixelRatio = 1;
      tester.binding.handleMetricsChanged();
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            ...voiceThemeTestOverrides(),
            authorizationHeaderProvider.overrideWithValue('Bearer test'),
            gatewayHttpClientProvider.overrideWithValue(gateway),
            spacePermissionProvider((
              spaceId: 'space-1',
              permission: SpacePermissions.spaceManageInvites,
              chatId: null,
              voiceRoomId: null,
            )).overrideWith((ref) async => true),
            spaceInvitesProvider(
              'space-1',
            ).overrideWith((ref) async => expiringInvites),
          ],
          child: RepaintBoundary(
            key: _spaceInviteCaptureRootKey,
            child: MaterialApp(
              debugShowCheckedModeBanner: false,
              theme: theme,
              locale: const Locale('en'),
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: Scaffold(
                body: Center(
                  child: Builder(
                    builder: (context) => TextButton(
                      key: const Key('open_space_invites'),
                      focusNode: launcherFocus,
                      onPressed: () =>
                          SpaceInvitesSheet.show(context, spaceId: 'space-1'),
                      child: const Text('Manage invites'),
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('open_space_invites')));
      await tester.pumpAndSettle();

      expect(find.text('Never'), findsOneWidget);
      expect(find.byKey(const Key('qr_invite_inv-expiring')), findsOneWidget);
      expect(
        find.byKey(const Key('revoke_invite_inv-expiring')),
        findsOneWidget,
      );
      expect(tester.takeException(), isNull);
      if (captureEnabled) {
        await _writeSpaceInviteCapture(tester, captureDirectory, filename);
      }
    }

    await captureAt(const Size(390, 844), 'space-invites-v-390x844.png');
    await tester.tap(find.byKey(const Key('qr_invite_inv-expiring')));
    await tester.pumpAndSettle();
    expect(find.byType(QrImageView), findsOneWidget);
    if (captureEnabled) {
      await _writeSpaceInviteCapture(
        tester,
        captureDirectory,
        'space-invites-qr-v-390x844.png',
      );
    }

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pumpAndSettle();
    await captureAt(const Size(844, 390), 'space-invites-h-844x390.png');
    expect(tester.takeException(), isNull);
  });
}

Future<ThemeData> _spaceInviteCaptureTheme(WidgetTester tester) async {
  if (!_spaceInviteCaptureFontsLoaded) {
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
      final iconData = ByteData.sublistView(
        Uint8List.fromList(await iconFont.readAsBytes()),
      );
      await (FontLoader(
        'MaterialIcons',
      )..addFont(Future<ByteData>.value(iconData))).load();
      return true;
    });
    if (loaded != true) {
      throw StateError('Production font loading did not complete');
    }
    _spaceInviteCaptureFontsLoaded = true;
  }

  final theme = await tester.runAsync(() async {
    final catalog = await VoiceTokenCatalog.load();
    return VoiceTheme.build(
      catalog: catalog,
      mode: VoiceThemeMode.dark,
      profileAccent: catalog.profileAccentAt(0),
    );
  });
  if (theme == null) {
    throw StateError('Voice design tokens did not load for capture');
  }
  return theme;
}

Future<void> _writeSpaceInviteCapture(
  WidgetTester tester,
  String directory,
  String filename,
) async {
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_spaceInviteCaptureRootKey),
  );
  final captured = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      final data = await image.toByteData(format: ui.ImageByteFormat.png);
      if (data == null) throw StateError('PNG encoding returned null');
      final output = File('$directory${Platform.pathSeparator}$filename');
      await output.parent.create(recursive: true);
      await output.writeAsBytes(data.buffer.asUint8List(), flush: true);
      return data.buffer.asUint8List();
    } finally {
      image.dispose();
    }
  });
  if (captured == null || captured.isEmpty) {
    throw StateError('Space invite screenshot capture did not complete');
  }
}
