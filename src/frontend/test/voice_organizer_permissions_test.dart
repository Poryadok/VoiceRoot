import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/roles_client.dart';
import 'package:voice_frontend/backend/space_permissions.dart';
import 'package:voice_frontend/backend/voice_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/call_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/ui/call/voice_organizer_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/gateway_test_client.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('Space moderator controls wait for current scoped permission', (
    tester,
  ) async {
    final permission = Completer<bool>();
    final roles = _RolesClient()..moderatorPermission = permission.future;
    final harness = await _mount(tester, roles: roles);

    final commander = tester.widget<FilterChip>(
      find.byKey(VoiceOrganizerPanel.commanderKey),
    );
    expect(commander.onSelected, isNull);
    expect(
      tester
          .widget<TextButton>(
            find.byKey(const Key('voice_organizer_grant_peer')),
          )
          .onPressed,
      isNull,
    );
    expect(
      tester
          .widget<FilterChip>(find.byKey(VoiceOrganizerPanel.raiseHandKey))
          .onSelected,
      isNotNull,
    );
    expect(find.text('Checking permissions…'), findsWidgets);
    expect(
      roles.calls,
      contains((
        spaceId: 'space-1',
        profileId: 'prof-test',
        permission: SpacePermissions.voiceMuteOthers,
        voiceRoomId: 'voice-room-1',
        chatId: null,
      )),
    );
    expect(harness.voice.actions, isEmpty);

    permission.complete(false);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(VoiceOrganizerPanel.commanderKey));
    expect(harness.voice.actions, isEmpty);
    expect(find.textContaining('Missing permission:'), findsWidgets);
  });

  testWidgets('failed permission lookup never enables Space floor control', (
    tester,
  ) async {
    final permission = Completer<bool>();
    final harness = await _mount(
      tester,
      roles: _RolesClient()..moderatorPermission = permission.future,
    );
    permission.completeError(StateError('private Role diagnostic'));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('voice_organizer_grant_peer')));
    expect(harness.voice.actions, isEmpty);
    expect(find.textContaining('private Role diagnostic'), findsNothing);
    expect(find.textContaining('Missing permission:'), findsWidgets);
  });

  testWidgets(
    'broadcast enable needs speaking as well as moderator permission',
    (tester) async {
      final voice = _VoiceClient()..commander = true;
      final harness = await _mount(
        tester,
        roles: _RolesClient()..speakingPermission = Future.value(false),
        voice: voice,
      );
      await tester.pumpAndSettle();

      expect(
        tester
            .widget<FilterChip>(find.byKey(VoiceOrganizerPanel.broadcastKey))
            .onSelected,
        isNull,
      );
      await tester.tap(find.byKey(const Key('voice_organizer_grant_peer')));
      await tester.pumpAndSettle();
      expect(harness.voice.actions, ['grant:peer']);
      expect(find.text('Missing permission: Voice Speak'), findsOneWidget);
    },
  );

  testWidgets(
    'permission loss still permits stopping own broadcast and commander',
    (tester) async {
      final voice = _VoiceClient()
        ..commander = true
        ..broadcasting = true;
      await _mount(
        tester,
        roles: _RolesClient()
          ..moderatorPermission = Future.value(false)
          ..speakingPermission = Future.value(false),
        voice: voice,
      );
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(VoiceOrganizerPanel.broadcastKey));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(VoiceOrganizerPanel.commanderKey));
      await tester.pumpAndSettle();
      expect(voice.actions, ['broadcast:false', 'commander:false']);
      expect(
        tester
            .widget<TextButton>(
              find.byKey(const Key('voice_organizer_revoke_speaker')),
            )
            .onPressed,
        isNull,
      );
    },
  );

  testWidgets(
    'ordinary group controls do not infer Space permission requirements',
    (tester) async {
      final roles = _RolesClient()..moderatorPermission = Future.value(false);
      final harness = await _mount(
        tester,
        roles: roles,
        session: const VoiceCallSession(
          roomId: 'group-call',
          livekitRoomName: 'group',
          chatId: 'group-chat',
          initiatorProfileId: 'prof-test',
          calleeProfileId: '',
          mediaKind: VoiceCallMediaKind.audio,
          status: VoiceCallStatus.active,
          sessionKind: VoiceSessionKind.groupVoice,
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(VoiceOrganizerPanel.commanderKey));
      await tester.pumpAndSettle();

      expect(harness.voice.actions, ['commander:true']);
      expect(roles.calls, isEmpty);
    },
  );

  testWidgets(
    'missing Space locator cannot fall back to ordinary group controls',
    (tester) async {
      final roles = _RolesClient();
      final harness = await _mount(
        tester,
        roles: roles,
        session: _spaceSession(spaceId: null, voiceRoomId: null),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(VoiceOrganizerPanel.commanderKey));

      expect(harness.voice.actions, isEmpty);
      expect(roles.calls, isEmpty);
      expect(find.textContaining('Missing permission:'), findsWidgets);
    },
  );

  testWidgets(
    'a server permission race is visible without upstream diagnostics',
    (tester) async {
      final voice = _VoiceClient()
        ..actionResult = const VoiceApiFailure(
          message: 'private upstream permission diagnostic',
          statusCode: 403,
        );
      await _mount(tester, roles: _RolesClient(), voice: voice);
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(VoiceOrganizerPanel.commanderKey));
      await tester.pumpAndSettle();

      expect(voice.actions, ['commander:true']);
      expect(find.textContaining('Missing permission:'), findsWidgets);
      expect(find.textContaining('private upstream'), findsNothing);
    },
  );

  testWidgets(
    'old action completion does not refresh or mark a replacement room',
    (tester) async {
      final action = Completer<VoiceApiResult<void>>();
      final voice = _VoiceClient()..actionFuture = action.future;
      final harness = await _mount(tester, roles: _RolesClient(), voice: voice);
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(VoiceOrganizerPanel.commanderKey));
      await tester.pump();
      (harness.container.read(callControllerProvider.notifier)
              as _CallController)
          .replaceSession(
            _spaceSession(roomId: 'call-2', voiceRoomId: 'voice-room-2'),
          );
      await tester.pumpAndSettle();
      final readsAfterSwitch = voice.stateRooms.length;

      action.complete(
        const VoiceApiFailure(message: 'old diagnostic', statusCode: 403),
      );
      await tester.pumpAndSettle();

      expect(voice.stateRooms.length, readsAfterSwitch);
      expect(find.textContaining('Missing permission:'), findsNothing);
    },
  );

  testWidgets(
    'a foreign voice binding disables actions and drops old results',
    (tester) async {
      final action = Completer<VoiceApiResult<void>>();
      final voice = _VoiceClient()..actionFuture = action.future;
      final harness = await _mount(tester, roles: _RolesClient(), voice: voice);
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(VoiceOrganizerPanel.commanderKey));
      await tester.pump();
      (harness.container.read(callControllerProvider.notifier)
              as _CallController)
          .replaceBindingOwner('another-profile');
      await tester.pumpAndSettle();
      final readsAfterSwitch = voice.stateRooms.length;
      expect(
        tester
            .widget<FilterChip>(find.byKey(VoiceOrganizerPanel.commanderKey))
            .onSelected,
        isNull,
      );
      expect(
        tester
            .widget<FilterChip>(find.byKey(VoiceOrganizerPanel.raiseHandKey))
            .onSelected,
        isNull,
      );
      action.complete(
        const VoiceApiFailure(message: 'old diagnostic', statusCode: 403),
      );
      await tester.pumpAndSettle();
      expect(voice.stateRooms.length, readsAfterSwitch);
      expect(find.textContaining('Missing permission:'), findsNothing);
    },
  );
}

typedef _PermissionCall = ({
  String spaceId,
  String profileId,
  String permission,
  String? voiceRoomId,
  String? chatId,
});

class _RolesClient extends VoiceRolesClient {
  _RolesClient() : super(gateway: gatewayHttpForTest(_unexpectedHttp()));

  Future<bool> moderatorPermission = Future.value(true);
  Future<bool> speakingPermission = Future.value(true);
  final calls = <_PermissionCall>[];

  @override
  Future<RolesApiResult<bool>> checkPermission({
    required String authorization,
    required String spaceId,
    required String profileId,
    required String permissionName,
    String? chatId,
    String? voiceRoomId,
  }) async {
    calls.add((
      spaceId: spaceId,
      profileId: profileId,
      permission: permissionName,
      voiceRoomId: voiceRoomId,
      chatId: chatId,
    ));
    return RolesApiOk(
      await (permissionName == SpacePermissions.voiceMuteOthers
          ? moderatorPermission
          : speakingPermission),
    );
  }
}

class _VoiceClient extends VoiceCallsClient {
  _VoiceClient() : super(gateway: gatewayHttpForTest(_unexpectedHttp()));

  final actions = <String>[];
  final stateRooms = <String>[];
  bool commander = false;
  bool broadcasting = false;
  VoiceApiResult<void> actionResult = const VoiceApiOk(null);
  Future<VoiceApiResult<void>>? actionFuture;

  @override
  Future<VoiceApiResult<List<VoiceRoomParticipantState>>> getCallVoiceStates({
    required String authorization,
    required String roomId,
  }) async {
    stateRooms.add(roomId);
    return VoiceApiOk([
      VoiceRoomParticipantState(
        profileId: 'prof-test',
        isCommander: commander,
        isBroadcasting: broadcasting,
      ),
      const VoiceRoomParticipantState(profileId: 'peer', handRaised: true),
      const VoiceRoomParticipantState(profileId: 'speaker', hasFloor: true),
    ]);
  }

  @override
  Future<VoiceApiResult<void>> setCommanderMode({
    required String authorization,
    required String roomId,
    required bool enabled,
  }) async {
    actions.add('commander:$enabled');
    return actionFuture ?? actionResult;
  }

  @override
  Future<VoiceApiResult<void>> setBroadcasting({
    required String authorization,
    required String roomId,
    required bool enabled,
  }) async {
    actions.add('broadcast:$enabled');
    return actionResult;
  }

  @override
  Future<VoiceApiResult<void>> grantFloor({
    required String authorization,
    required String roomId,
    required String profileId,
  }) async {
    actions.add('grant:$profileId');
    return actionResult;
  }
}

class _CallController extends CallController {
  _CallController(super.ref, VoiceCallSession session) {
    state = CallState(
      phase: CallPhase.active,
      session: session,
      voiceBindingProfileId: 'prof-test',
    );
  }

  void replaceSession(VoiceCallSession session) {
    state = CallState(
      phase: CallPhase.active,
      session: session,
      voiceBindingProfileId: 'prof-test',
    );
  }

  void replaceBindingOwner(String profileId) {
    state = CallState(
      phase: CallPhase.active,
      session: state.session,
      voiceBindingProfileId: profileId,
    );
  }
}

class _Harness {
  _Harness(this.container, this.voice);
  final ProviderContainer container;
  final _VoiceClient voice;
}

Future<_Harness> _mount(
  WidgetTester tester, {
  required _RolesClient roles,
  _VoiceClient? voice,
  VoiceCallSession? session,
}) async {
  final voiceClient = voice ?? _VoiceClient();
  final container = ProviderContainer(
    overrides: [
      ...voiceAppTestOverrides(client: _unexpectedHttp()),
      voiceCallsClientProvider.overrideWithValue(voiceClient),
      voiceRolesClientProvider.overrideWithValue(roles),
      spaceViewerProfileIdProvider.overrideWith(
        (ref) => ref.watch(authControllerProvider).activeProfileId,
      ),
      callControllerProvider.overrideWith(
        (ref) => _CallController(ref, session ?? _spaceSession()),
      ),
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
        home: const Scaffold(body: VoiceOrganizerPanel()),
      ),
    ),
  );
  await tester.pump();
  await tester.pump();
  return _Harness(container, voiceClient);
}

VoiceCallSession _spaceSession({
  String roomId = 'call-1',
  String? spaceId = 'space-1',
  String? voiceRoomId = 'voice-room-1',
}) => VoiceCallSession(
  roomId: roomId,
  livekitRoomName: 'livekit-room',
  chatId: '',
  initiatorProfileId: 'prof-test',
  calleeProfileId: '',
  mediaKind: VoiceCallMediaKind.audio,
  status: VoiceCallStatus.active,
  sessionKind: VoiceSessionKind.voiceRoom,
  spaceId: spaceId,
  voiceRoomId: voiceRoomId,
);

http.Client _unexpectedHttp() => MockClient(
  (_) async => http.Response('{"error":"unexpected_test_request"}', 500),
);
