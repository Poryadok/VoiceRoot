import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/voice_client.dart';
import '../../state/auth_providers.dart';
import '../../state/call_providers.dart';
import '../../state/gateway_providers.dart';
import '../../theme/voice_colors.dart';

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

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _refresh());
  }

  Future<void> _refresh() async {
    final call = ref.read(callControllerProvider);
    final session = call.session;
    final auth = ref.read(authControllerProvider);
    final token = auth.accessToken;
    final selfId = auth.activeProfileId;
    if (session == null || token == null || token.isEmpty) return;

    setState(() => _loading = true);
    final result = await ref.read(voiceCallsClientProvider).getCallVoiceStates(
          authorization: 'Bearer $token',
          roomId: session.roomId,
        );
    if (!mounted) return;
    setState(() {
      _loading = false;
      if (result is VoiceApiOk<List<VoiceRoomParticipantState>>) {
        _participants = result.data;
        final self = _participants.cast<VoiceRoomParticipantState?>().firstWhere(
              (p) => p?.profileId == selfId,
              orElse: () => null,
            );
        _handRaised = self?.handRaised ?? false;
        _isCommander = self?.isCommander ?? false;
        _isBroadcasting = self?.isBroadcasting ?? false;
      }
    });
    await _applyDucking();
  }

  Future<void> _applyDucking() async {
    final room = ref.read(callControllerProvider.notifier).liveKitRoom;
    if (room == null) return;
    final broadcaster = _participants.cast<VoiceRoomParticipantState?>().firstWhere(
          (p) => p?.isBroadcasting == true,
          orElse: () => null,
        );
    await room.setCommanderDucking(
      enabled: broadcaster != null,
      commanderIdentity: broadcaster?.profileId,
      duckedVolume: 0.2,
    );
  }

  Future<void> _run(Future<VoiceApiResult<void>> Function(String token, String roomId) action) async {
    final call = ref.read(callControllerProvider);
    final session = call.session;
    final token = ref.read(authControllerProvider).accessToken;
    if (session == null || token == null || token.isEmpty) return;
    await action(token, session.roomId);
    await _refresh();
  }

  @override
  Widget build(BuildContext context) {
    final call = ref.watch(callControllerProvider);
    if (!call.isActive || call.session == null) {
      return const SizedBox.shrink();
    }
    final voice = VoiceColors.of(context);
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
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                FilterChip(
                  key: VoiceOrganizerPanel.raiseHandKey,
                  label: Text(_handRaised ? 'Lower hand' : 'Raise hand'),
                  selected: _handRaised,
                  onSelected: (_) => _run((token, roomId) {
                    final client = ref.read(voiceCallsClientProvider);
                    return _handRaised
                        ? client.lowerHand(
                            authorization: 'Bearer $token',
                            roomId: roomId,
                          )
                        : client.raiseHand(
                            authorization: 'Bearer $token',
                            roomId: roomId,
                          );
                  }),
                ),
                FilterChip(
                  key: VoiceOrganizerPanel.commanderKey,
                  label: Text(_isCommander ? 'Commander on' : 'Commander'),
                  selected: _isCommander,
                  onSelected: (v) => _run((token, roomId) {
                    return ref.read(voiceCallsClientProvider).setCommanderMode(
                          authorization: 'Bearer $token',
                          roomId: roomId,
                          enabled: v,
                        );
                  }),
                ),
                if (_isCommander)
                  FilterChip(
                    key: VoiceOrganizerPanel.broadcastKey,
                    label: Text(
                      _isBroadcasting ? 'Stop broadcast' : 'Start broadcast',
                    ),
                    selected: _isBroadcasting,
                    onSelected: (v) => _run((token, roomId) {
                      return ref.read(voiceCallsClientProvider).setBroadcasting(
                            authorization: 'Bearer $token',
                            roomId: roomId,
                            enabled: v,
                          );
                    }),
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
                    onPressed: () => _run((token, roomId) {
                      return ref.read(voiceCallsClientProvider).grantFloor(
                            authorization: 'Bearer $token',
                            roomId: roomId,
                            profileId: p.profileId,
                          );
                    }),
                    child: const Text('Grant floor'),
                  ),
                ),
            ],
            if (withFloor.isNotEmpty) ...[
              const SizedBox(height: 4),
              Text(
                'Has floor',
                style: Theme.of(context).textTheme.labelLarge,
              ),
              for (final p in withFloor)
                ListTile(
                  dense: true,
                  contentPadding: EdgeInsets.zero,
                  title: Text(p.profileId),
                  trailing: TextButton(
                    key: Key('voice_organizer_revoke_${p.profileId}'),
                    onPressed: () => _run((token, roomId) {
                      return ref.read(voiceCallsClientProvider).revokeFloor(
                            authorization: 'Bearer $token',
                            roomId: roomId,
                            profileId: p.profileId,
                          );
                    }),
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
