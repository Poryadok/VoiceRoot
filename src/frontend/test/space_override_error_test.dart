import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/roles_client.dart';
import 'package:voice_frontend/backend/space_permissions.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/ui/space/space_chat_override_sheet.dart';
import 'package:voice_frontend/ui/space/space_voice_room_override_sheet.dart';

import 'support/gateway_test_client.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const _diagnostic = 'sensitive role backend diagnostic';
const _role = SpaceRole(id: 'role-1', spaceId: 'space-1', name: 'Member');

void main() {
  testWidgets('chat override hides generic failure and rolls back', (
    tester,
  ) async {
    final roles = _OverrideRolesClient()
      ..chatResult = const RolesApiFailure(
        message: _diagnostic,
        errorCode: 'internal_error',
        statusCode: 500,
      );
    await _pumpOverrideSheet(tester, roles, _OverrideSurface.chat);
    final l10n = await AppLocalizations.delegate.load(const Locale('en'));

    final toggle = _toggle(
      _OverrideSurface.chat,
      l10n.spaceChatOverrideDenyView,
    );
    await tester.tap(toggle);
    await tester.pumpAndSettle();

    expect(find.text(l10n.commonActionFailed), findsOneWidget);
    expect(find.text(_diagnostic), findsNothing);
    expect(tester.widget<Switch>(toggle).value, isFalse);
    expect(roles.chatCalls, hasLength(1));
    expect(
      roles.chatCalls.single.denyMask,
      SpacePermissions.setPermission(0, SpacePermissions.textChatView, true),
    );
  });

  testWidgets('chat override maps unavailable failure and rolls back', (
    tester,
  ) async {
    final roles = _OverrideRolesClient()
      ..chatResult = const RolesApiFailure(
        message: _diagnostic,
        errorCode: 'unavailable',
        statusCode: 503,
      );
    await _pumpOverrideSheet(tester, roles, _OverrideSurface.chat);
    final l10n = await AppLocalizations.delegate.load(const Locale('en'));

    final toggle = _toggle(
      _OverrideSurface.chat,
      l10n.spaceChatOverrideDenySend,
    );
    await tester.tap(toggle);
    await tester.pumpAndSettle();

    expect(find.text(l10n.backendUnavailable), findsOneWidget);
    expect(find.text(_diagnostic), findsNothing);
    expect(tester.widget<Switch>(toggle).value, isFalse);
    expect(
      roles.chatCalls.single.denyMask,
      SpacePermissions.setPermission(
        0,
        SpacePermissions.textChatSendMessages,
        true,
      ),
    );
  });

  testWidgets('voice override hides generic failure and rolls back', (
    tester,
  ) async {
    final roles = _OverrideRolesClient()
      ..voiceResult = const RolesApiFailure(
        message: _diagnostic,
        errorCode: 'internal_error',
        statusCode: 500,
      );
    await _pumpOverrideSheet(tester, roles, _OverrideSurface.voice);
    final l10n = await AppLocalizations.delegate.load(const Locale('en'));

    final toggle = _toggle(
      _OverrideSurface.voice,
      l10n.spaceVoiceOverrideDenyJoin,
    );
    await tester.tap(toggle);
    await tester.pumpAndSettle();

    expect(find.text(l10n.commonActionFailed), findsOneWidget);
    expect(find.text(_diagnostic), findsNothing);
    expect(tester.widget<Switch>(toggle).value, isFalse);
    expect(
      roles.voiceCalls.single.denyMask,
      SpacePermissions.setPermission(0, SpacePermissions.voiceJoin, true),
    );
  });

  testWidgets('voice override maps unavailable failure and rolls back', (
    tester,
  ) async {
    final roles = _OverrideRolesClient()
      ..voiceResult = const RolesApiFailure(
        message: _diagnostic,
        errorCode: 'unavailable',
        statusCode: 503,
      );
    await _pumpOverrideSheet(tester, roles, _OverrideSurface.voice);
    final l10n = await AppLocalizations.delegate.load(const Locale('en'));

    final toggle = _toggle(
      _OverrideSurface.voice,
      l10n.spaceVoiceOverrideDenyJoin,
    );
    await tester.tap(toggle);
    await tester.pumpAndSettle();

    expect(find.text(l10n.backendUnavailable), findsOneWidget);
    expect(find.text(_diagnostic), findsNothing);
    expect(tester.widget<Switch>(toggle).value, isFalse);
  });

  testWidgets('chat override stays busy until success and sends the mask', (
    tester,
  ) async {
    final roles = _OverrideRolesClient()
      ..chatPending = Completer<RolesApiResult<void>>();
    await _pumpOverrideSheet(tester, roles, _OverrideSurface.chat);
    final l10n = await AppLocalizations.delegate.load(const Locale('en'));
    final toggle = _toggle(
      _OverrideSurface.chat,
      l10n.spaceChatOverrideDenyView,
    );

    await tester.tap(toggle);
    await tester.pump();
    expect(tester.widget<Switch>(toggle).onChanged, isNull);
    expect(roles.chatCalls.single, (
      authorization: 'Bearer test',
      spaceId: 'space-1',
      chatId: 'chat-1',
      roleId: 'role-1',
      allowMask: 0,
      denyMask: SpacePermissions.setPermission(
        0,
        SpacePermissions.textChatView,
        true,
      ),
    ));

    roles.chatPending!.complete(const RolesApiOk<void>(null));
    await tester.pumpAndSettle();
    expect(tester.widget<Switch>(toggle).value, isTrue);
    expect(find.byType(SnackBar), findsNothing);
  });

  testWidgets('voice override stays busy until success and sends the mask', (
    tester,
  ) async {
    final roles = _OverrideRolesClient()
      ..voicePending = Completer<RolesApiResult<void>>();
    await _pumpOverrideSheet(tester, roles, _OverrideSurface.voice);
    final l10n = await AppLocalizations.delegate.load(const Locale('en'));
    final toggle = _toggle(
      _OverrideSurface.voice,
      l10n.spaceVoiceOverrideDenyJoin,
    );

    await tester.tap(toggle);
    await tester.pump();
    expect(tester.widget<Switch>(toggle).onChanged, isNull);
    expect(roles.voiceCalls.single, (
      authorization: 'Bearer test',
      spaceId: 'space-1',
      voiceRoomId: 'room-1',
      roleId: 'role-1',
      allowMask: 0,
      denyMask: SpacePermissions.setPermission(
        0,
        SpacePermissions.voiceJoin,
        true,
      ),
    ));

    roles.voicePending!.complete(const RolesApiOk<void>(null));
    await tester.pumpAndSettle();
    expect(tester.widget<Switch>(toggle).value, isTrue);
    expect(find.byType(SnackBar), findsNothing);
  });
}

enum _OverrideSurface { chat, voice }

Future<void> _pumpOverrideSheet(
  WidgetTester tester,
  _OverrideRolesClient roles,
  _OverrideSurface surface,
) async {
  tester.view.physicalSize = const Size(800, 1200);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        ...voiceThemeTestOverrides(),
        authorizationHeaderProvider.overrideWithValue('Bearer test'),
        voiceRolesClientProvider.overrideWithValue(roles),
        spaceRolesProvider(
          'space-1',
        ).overrideWith((ref) async => const [_role]),
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: surface == _OverrideSurface.chat
              ? const SpaceChatOverrideSheet(
                  spaceId: 'space-1',
                  chatId: 'chat-1',
                )
              : const SpaceVoiceRoomOverrideSheet(
                  spaceId: 'space-1',
                  voiceRoomId: 'room-1',
                ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

Finder _toggle(_OverrideSurface surface, String label) {
  final tile = find.ancestor(
    of: find.text(label),
    matching: find.byType(SwitchListTile),
  );
  return find.descendant(of: tile, matching: find.byType(Switch));
}

class _OverrideRolesClient extends VoiceRolesClient {
  _OverrideRolesClient()
    : super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 500)),
        ),
      );

  RolesApiResult<void> chatResult = const RolesApiOk<void>(null);
  RolesApiResult<void> voiceResult = const RolesApiOk<void>(null);
  Completer<RolesApiResult<void>>? chatPending;
  Completer<RolesApiResult<void>>? voicePending;
  final chatCalls =
      <
        ({
          String authorization,
          String spaceId,
          String chatId,
          String roleId,
          int allowMask,
          int denyMask,
        })
      >[];
  final voiceCalls =
      <
        ({
          String authorization,
          String spaceId,
          String voiceRoomId,
          String roleId,
          int allowMask,
          int denyMask,
        })
      >[];

  @override
  Future<RolesApiResult<void>> setChatOverride({
    required String authorization,
    required String spaceId,
    required String chatId,
    required String roleId,
    int allowMask = 0,
    int denyMask = 0,
  }) {
    chatCalls.add((
      authorization: authorization,
      spaceId: spaceId,
      chatId: chatId,
      roleId: roleId,
      allowMask: allowMask,
      denyMask: denyMask,
    ));
    return chatPending?.future ?? Future.value(chatResult);
  }

  @override
  Future<RolesApiResult<void>> setVoiceRoomOverride({
    required String authorization,
    required String spaceId,
    required String voiceRoomId,
    required String roleId,
    int allowMask = 0,
    int denyMask = 0,
  }) {
    voiceCalls.add((
      authorization: authorization,
      spaceId: spaceId,
      voiceRoomId: voiceRoomId,
      roleId: roleId,
      allowMask: allowMask,
      denyMask: denyMask,
    ));
    return voicePending?.future ?? Future.value(voiceResult);
  }
}
