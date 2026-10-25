import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/voice_client.dart';
import '../../backend/livekit_room.dart';
import '../../backend/space_permissions.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/call_providers.dart';
import '../../state/space_providers.dart';
import '../../theme/voice_colors.dart';
import '../api_error_messages.dart';
import '../core/voice_compact_banner.dart';
import '../core/voice_disabled_action.dart';

/// Organizer controls: raised hands, grant/revoke floor, commander broadcast.
class VoiceOrganizerPanel extends ConsumerStatefulWidget {
  const VoiceOrganizerPanel({super.key});

  static const Key panelKey = Key('voice_organizer_panel');
  static const Key raiseHandKey = Key('voice_organizer_raise_hand');
  static const Key broadcastKey = Key('voice_organizer_broadcast');
  static const Key commanderKey = Key('voice_organizer_commander');

  @override
  ConsumerState<VoiceOrganizerPanel> createState() =>
      _VoiceOrganizerPanelState();
}

class _VoiceOrganizerPanelState extends ConsumerState<VoiceOrganizerPanel> {
  List<VoiceRoomParticipantState> _participants = const [];
  var _loading = false;
  var _handRaised = false;
  var _isCommander = false;
  var _isBroadcasting = false;
  VoiceApiFailure? _actionFailure;
  String? _failedPermission;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _refresh());
  }

  Future<void> _refresh() async {
    final call = ref.read(callControllerProvider);
    final session = call.session;
    final authHeader = ref.read(authorizationHeaderProvider);
    final selfId = ref.read(authControllerProvider).activeProfileId;
    if (session == null ||
        authHeader == null ||
        authHeader.isEmpty ||
        !_callerOwnsBinding(call, selfId)) {
      return;
    }
    final room = ref.read(callControllerProvider.notifier).liveKitRoom;

    setState(() => _loading = true);
    final result = await ref
        .read(voiceCallsClientProvider)
        .getCallVoiceStates(authorization: authHeader, roomId: session.roomId);
    if (!mounted || !_isCurrentBinding(session, authHeader, selfId, room)) {
      return;
    }
    setState(() {
      _loading = false;
      if (result is VoiceApiOk<List<VoiceRoomParticipantState>>) {
        _participants = result.data;
        final self = _participants
            .cast<VoiceRoomParticipantState?>()
            .firstWhere((p) => p?.profileId == selfId, orElse: () => null);
        _handRaised = self?.handRaised ?? false;
        _isCommander = self?.isCommander ?? false;
        _isBroadcasting = self?.isBroadcasting ?? false;
      }
    });
    if (_isCurrentBinding(session, authHeader, selfId, room)) {
      await _applyDucking(session: session, room: room);
    }
  }

  bool _isCurrentBinding(
    VoiceCallSession session,
    String authorization,
    String? profileId,
    VoiceLiveKitRoom? room,
  ) {
    final currentCall = ref.read(callControllerProvider);
    final current = currentCall.session;
    return mounted &&
        _callerOwnsBinding(currentCall, profileId) &&
        identical(
          ref.read(callControllerProvider.notifier).liveKitRoom,
          room,
        ) &&
        ref.read(authorizationHeaderProvider) == authorization &&
        ref.read(authControllerProvider).activeProfileId == profileId &&
        current?.roomId == session.roomId &&
        current?.sessionKind == session.sessionKind &&
        current?.spaceId == session.spaceId &&
        current?.voiceRoomId == session.voiceRoomId &&
        current?.matchId == session.matchId &&
        current?.mediaEpoch == session.mediaEpoch;
  }

  bool _callerOwnsBinding(CallState call, String? profileId) =>
      profileId != null &&
      (call.voiceBindingProfileId == null ||
          call.voiceBindingProfileId == profileId);

  void _resetBinding() {
    setState(() {
      _participants = const [];
      _loading = false;
      _handRaised = false;
      _isCommander = false;
      _isBroadcasting = false;
      _actionFailure = null;
      _failedPermission = null;
    });
    unawaited(_refresh());
  }

  Future<void> _applyDucking({
    required VoiceCallSession? session,
    required VoiceLiveKitRoom? room,
  }) async {
    if (room == null) return;
    final broadcaster = _participants
        .cast<VoiceRoomParticipantState?>()
        .firstWhere((p) => p?.isBroadcasting == true, orElse: () => null);
    final isMatchSquad =
        session?.matchId != null && session?.mediaEpoch != null;
    final identity = isMatchSquad && broadcaster != null
        ? room is MatchSquadVoiceLiveKitRoom
              ? room.resolveRemoteParticipantIdentity(broadcaster.profileId)
              : null
        : broadcaster?.profileId;
    await room.setCommanderDucking(
      enabled: broadcaster != null && (!isMatchSquad || identity != null),
      commanderIdentity: identity,
      duckedVolume: 0.2,
    );
  }

  Future<void> _run(
    Future<VoiceApiResult<void>> Function(String auth, String roomId) action, {
    String? permission,
  }) async {
    final call = ref.read(callControllerProvider);
    final session = call.session;
    final authHeader = ref.read(authorizationHeaderProvider);
    final selfId = ref.read(authControllerProvider).activeProfileId;
    final room = ref.read(callControllerProvider.notifier).liveKitRoom;
    if (session == null ||
        authHeader == null ||
        authHeader.isEmpty ||
        !_callerOwnsBinding(call, selfId)) {
      return;
    }
    setState(() {
      _actionFailure = null;
      _failedPermission = null;
    });
    final result = await action(authHeader, session.roomId);
    if (!_isCurrentBinding(session, authHeader, selfId, room)) return;
    if (result is VoiceApiFailure) {
      setState(() {
        _actionFailure = result;
        _failedPermission = permission;
      });
    }
    await _refresh();
  }

  ({bool allowed, String? reason}) _permission(
    VoiceCallSession session,
    String permission,
    AppLocalizations l10n,
  ) {
    if (session.sessionKind != VoiceSessionKind.voiceRoom) {
      return (allowed: true, reason: null);
    }
    final spaceId = session.spaceId;
    final voiceRoomId = session.voiceRoomId;
    if (spaceId == null ||
        spaceId.isEmpty ||
        voiceRoomId == null ||
        voiceRoomId.isEmpty) {
      return (
        allowed: false,
        reason: spacePermissionDeniedReason(l10n, permission),
      );
    }
    final result = ref.watch(
      spacePermissionProvider((
        spaceId: spaceId,
        permission: permission,
        chatId: null,
        voiceRoomId: voiceRoomId,
      )),
    );
    if (result.isLoading) {
      return (allowed: false, reason: l10n.spacePermissionChecking);
    }
    final allowed = !result.hasError && result.valueOrNull == true;
    return (
      allowed: allowed,
      reason: allowed ? null : spacePermissionDeniedReason(l10n, permission),
    );
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(
      callControllerProvider.select(
        (call) => (call.session, call.voiceBindingProfileId),
      ),
      (_, _) => _resetBinding(),
    );
    ref.listen(
      authControllerProvider.select(
        (auth) => (auth.session?.accessToken, auth.activeProfileId),
      ),
      (_, _) => _resetBinding(),
    );
    ref.listen<int>(
      callControllerProvider.select((call) => call.mediaTracksVersion),
      (_, _) {
        final session = ref.read(callControllerProvider).session;
        final room = ref.read(callControllerProvider.notifier).liveKitRoom;
        unawaited(_applyDucking(session: session, room: room));
      },
    );
    final call = ref.watch(callControllerProvider);
    if (!call.isActive || call.session == null) {
      return const SizedBox.shrink();
    }
    final voice = VoiceColors.of(context);
    final l10n = AppLocalizations.of(context)!;
    final callerOwnsBinding = _callerOwnsBinding(
      call,
      ref.watch(authControllerProvider).activeProfileId,
    );
    final moderator = _permission(
      call.session!,
      SpacePermissions.voiceMuteOthers,
      l10n,
    );
    final speaker = _permission(
      call.session!,
      SpacePermissions.voiceSpeak,
      l10n,
    );
    final canModerate = callerOwnsBinding && moderator.allowed;
    final canStartBroadcast = canModerate && speaker.allowed;
    final failure = _actionFailure;
    final reasons = <String>{
      if (!callerOwnsBinding) l10n.commonActionFailed,
      if (moderator.reason != null) moderator.reason!,
      if (_isCommander && !_isBroadcasting && speaker.reason != null)
        speaker.reason!,
      if (failure != null)
        if (failure.statusCode == 403 && _failedPermission != null)
          spacePermissionDeniedReason(l10n, _failedPermission!)
        else
          commonActionErrorMessage(l10n, statusCode: failure.statusCode),
    };
    final raised = _participants.where((p) => p.handRaised).toList();
    final withFloor = _participants.where((p) => p.hasFloor).toList();

    return Material(
      key: VoiceOrganizerPanel.panelKey,
      color: voice.elevated,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            for (final reason in reasons)
              VoiceCompactBanner(message: reason, icon: Icons.lock_outline),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                FilterChip(
                  key: VoiceOrganizerPanel.raiseHandKey,
                  label: Text(_handRaised ? 'Lower hand' : 'Raise hand'),
                  selected: _handRaised,
                  onSelected: !callerOwnsBinding
                      ? null
                      : (_) => _run((auth, roomId) {
                          final client = ref.read(voiceCallsClientProvider);
                          return _handRaised
                              ? client.lowerHand(
                                  authorization: auth,
                                  roomId: roomId,
                                )
                              : client.raiseHand(
                                  authorization: auth,
                                  roomId: roomId,
                                );
                        }),
                ),
                FilterChip(
                  key: VoiceOrganizerPanel.commanderKey,
                  label: Text(_isCommander ? 'Commander on' : 'Commander'),
                  selected: _isCommander,
                  onSelected:
                      !callerOwnsBinding || (!_isCommander && !canModerate)
                      ? null
                      : (v) => _run((auth, roomId) {
                          return ref
                              .read(voiceCallsClientProvider)
                              .setCommanderMode(
                                authorization: auth,
                                roomId: roomId,
                                enabled: v,
                              );
                        }, permission: SpacePermissions.voiceMuteOthers),
                ),
                if (_isCommander)
                  FilterChip(
                    key: VoiceOrganizerPanel.broadcastKey,
                    label: Text(
                      _isBroadcasting ? 'Stop broadcast' : 'Start broadcast',
                    ),
                    selected: _isBroadcasting,
                    onSelected:
                        !callerOwnsBinding ||
                            (!_isBroadcasting && !canStartBroadcast)
                        ? null
                        : (v) => _run(
                            (auth, roomId) {
                              return ref
                                  .read(voiceCallsClientProvider)
                                  .setBroadcasting(
                                    authorization: auth,
                                    roomId: roomId,
                                    enabled: v,
                                  );
                            },
                            permission: moderator.allowed
                                ? SpacePermissions.voiceSpeak
                                : SpacePermissions.voiceMuteOthers,
                          ),
                  ),
                IconButton(
                  tooltip: 'Refresh',
                  onPressed: _loading ? null : _refresh,
                  icon: _loading
                      ? const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Icon(Icons.refresh),
                ),
              ],
            ),
            if (raised.isNotEmpty) ...[
              const SizedBox(height: 8),
              Text(
                'Raised hands',
                style: Theme.of(context).textTheme.labelLarge,
              ),
              for (final p in raised)
                ListTile(
                  dense: true,
                  contentPadding: EdgeInsets.zero,
                  title: Text(p.profileId),
                  trailing: TextButton(
                    key: Key('voice_organizer_grant_${p.profileId}'),
                    onPressed: !canModerate
                        ? null
                        : () => _run((auth, roomId) {
                            return ref
                                .read(voiceCallsClientProvider)
                                .grantFloor(
                                  authorization: auth,
                                  roomId: roomId,
                                  profileId: p.profileId,
                                );
                          }, permission: SpacePermissions.voiceMuteOthers),
                    child: const Text('Grant floor'),
                  ),
                ),
            ],
            if (withFloor.isNotEmpty) ...[
              const SizedBox(height: 4),
              Text('Has floor', style: Theme.of(context).textTheme.labelLarge),
              for (final p in withFloor)
                ListTile(
                  dense: true,
                  contentPadding: EdgeInsets.zero,
                  title: Text(p.profileId),
                  trailing: TextButton(
                    key: Key('voice_organizer_revoke_${p.profileId}'),
                    onPressed: !canModerate
                        ? null
                        : () => _run((auth, roomId) {
                            return ref
                                .read(voiceCallsClientProvider)
                                .revokeFloor(
                                  authorization: auth,
                                  roomId: roomId,
                                  profileId: p.profileId,
                                );
                          }, permission: SpacePermissions.voiceMuteOthers),
                    child: const Text('Revoke'),
                  ),
                ),
            ],
          ],
        ),
      ),
    );
  }
}
