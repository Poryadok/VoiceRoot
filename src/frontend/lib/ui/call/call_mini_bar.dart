import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/livekit_room.dart';
import '../../l10n/app_localizations.dart';
import '../../state/call_presentation_providers.dart';
import '../../state/call_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/gateway_providers.dart';
import '../../state/space_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_avatar.dart';
import 'screen_share_panel.dart';

/// Occupies a normal shell slot; it never positions itself over content.
class CallMiniBar extends ConsumerStatefulWidget {
  const CallMiniBar({super.key, this.mobile = false});
  final bool mobile;
  static const panelKey = Key('call_mini_bar');
  static const returnKey = Key('call_mini_return');
  static const muteKey = Key('call_mini_mute');
  static const deafenKey = Key('call_mini_deafen');
  static const hangupKey = Key('call_mini_hangup');

  @override
  ConsumerState<CallMiniBar> createState() => _CallMiniBarState();
}

class _CallMiniBarState extends ConsumerState<CallMiniBar> {
  Timer? _timer;
  String? _roomId;
  final _returnFocus = FocusNode(debugLabel: 'call mini return');

  @override
  void initState() {
    super.initState();
    _timer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (mounted && _roomId != null) setState(() {});
    });
  }

  @override
  void dispose() {
    _timer?.cancel();
    _returnFocus.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final call = ref.watch(callControllerProvider);
    final session = call.session;
    ref.listen(callExpandedRoomProvider, (previous, next) {
      if (previous == _roomId && next == null) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (mounted) _returnFocus.requestFocus();
        });
      }
    });
    if (!call.isActive ||
        session == null ||
        !ref.watch(gatewayConfigProvider).canPlaceVoiceCalls) {
      _roomId = null;
      return const SizedBox.shrink();
    }
    if (_roomId != session.roomId) {
      _roomId = session.roomId;
    }
    // Watch before hiding so duration and roster keep updating while expanded.
    final participants = ref.watch(callParticipantsProvider);
    final connectedAt = ref.watch(callConnectedAtProvider);
    if (ref.watch(callExpandedRoomProvider) == session.roomId) {
      return const SizedBox.shrink();
    }
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    String? name;
    for (final item in ref.watch(chatListProvider).valueOrNull?.items ?? []) {
      if (item.chatId == session.chatId) {
        name = item.dmPeerDisplayName ?? item.chat.name;
        break;
      }
    }
    if (session.spaceId != null) {
      for (final room
          in ref
                  .watch(spaceTreeProvider(session.spaceId!))
                  .valueOrNull
                  ?.voiceRooms ??
              []) {
        if (room.id == session.voiceRoomId) name = room.name;
      }
    }
    final seconds = DateTime.now()
        .difference(connectedAt ?? DateTime.now())
        .inSeconds
        .clamp(0, 359999);
    final elapsed =
        '${(seconds ~/ 60).toString().padLeft(2, '0')}:${(seconds % 60).toString().padLeft(2, '0')}';
    final connecting = call.phase == CallPhase.connecting;
    final subtitle = connecting
        ? l10n.callConnecting
        : widget.mobile
        ? '$elapsed · ${l10n.callMiniParticipants(participants.length)}'
        : '${l10n.callMiniConnected} · $elapsed';
    void expand() =>
        ref.read(callExpandedRoomProvider.notifier).state = session.roomId;
    final copy = Expanded(
      child: InkWell(
        key: CallMiniBar.returnKey,
        focusNode: _returnFocus,
        onTap: expand,
        child: Semantics(
          button: true,
          label: l10n.callMiniReturn,
          child: Padding(
            padding: const EdgeInsets.symmetric(vertical: 4),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  name?.isNotEmpty == true ? name! : l10n.callActive,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                Text(
                  subtitle,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(fontSize: 12, color: voice.textSecondary),
                ),
              ],
            ),
          ),
        ),
      ),
    );
    final status = connecting
        ? const SizedBox(
            width: 8,
            height: 8,
            child: CircularProgressIndicator(strokeWidth: 2),
          )
        : Container(
            width: 8,
            height: 8,
            decoration: BoxDecoration(
              color: voice.success,
              shape: BoxShape.circle,
            ),
          );
    Widget action(
      Key key,
      String label,
      IconData icon,
      VoidCallback onPressed, {
      bool selected = false,
      bool danger = false,
    }) {
      return Expanded(
        child: Padding(
          padding: const EdgeInsets.only(left: 3, right: 3),
          child: IconButton(
            key: key,
            tooltip: label,
            isSelected: selected,
            style: IconButton.styleFrom(
              minimumSize: const Size(44, 44),
              tapTargetSize: MaterialTapTargetSize.shrinkWrap,
              backgroundColor: danger
                  ? voice.error
                  : selected
                  ? voice.profileAccent.withValues(alpha: .12)
                  : voice.elevated,
              foregroundColor: danger
                  ? Theme.of(context).colorScheme.onError
                  : voice.textPrimary,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(6),
                side: BorderSide(
                  color: selected ? voice.profileAccent : voice.borderDefault,
                ),
              ),
            ),
            onPressed: onPressed,
            icon: Icon(icon, size: 20),
          ),
        ),
      );
    }

    final controls = Row(
      children: [
        action(
          CallMiniBar.muteKey,
          call.isMuted ? l10n.callUnmute : l10n.callMute,
          call.isMuted ? Icons.mic_off_outlined : Icons.mic_none,
          () =>
              ref.read(callControllerProvider.notifier).setMuted(!call.isMuted),
          selected: !call.isMuted,
        ),
        if (!widget.mobile)
          action(
            CallMiniBar.deafenKey,
            call.isSpeakerMuted ? l10n.callSpeakerOn : l10n.callSpeakerOff,
            call.isSpeakerMuted
                ? Icons.headset_off_outlined
                : Icons.headphones_outlined,
            () => ref
                .read(callControllerProvider.notifier)
                .setSpeakerMuted(!call.isSpeakerMuted),
            selected: call.isSpeakerMuted,
          ),
        if (!widget.mobile)
          Expanded(
            child: ScreenShareCallButton(
              style: IconButton.styleFrom(
                minimumSize: const Size(44, 44),
                tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                backgroundColor: voice.elevated,
                foregroundColor: voice.textPrimary,
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(6),
                  side: BorderSide(color: voice.borderDefault),
                ),
              ),
            ),
          ),
        action(
          CallMiniBar.hangupKey,
          l10n.callHangup,
          Icons.call_end,
          () => ref.read(callControllerProvider.notifier).hangUp(),
          danger: true,
        ),
      ],
    );
    final avatars = _MiniParticipants(
      participants: participants.take(widget.mobile ? 2 : 3).toList(),
    );
    return Material(
      key: CallMiniBar.panelKey,
      color: voice.surface,
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: expand,
        child: Container(
          constraints: BoxConstraints(minHeight: widget.mobile ? 64 : 144),
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          decoration: BoxDecoration(
            border: Border(
              top: BorderSide(color: voice.borderDefault),
              bottom: widget.mobile
                  ? BorderSide(color: voice.borderDefault)
                  : BorderSide.none,
            ),
          ),
          child: FocusTraversalGroup(
            child: widget.mobile
                ? Row(
                    children: [
                      status,
                      const SizedBox(width: 8),
                      avatars,
                      const SizedBox(width: 8),
                      copy,
                      SizedBox(width: 96, child: controls),
                    ],
                  )
                : Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Row(
                        children: [
                          status,
                          const SizedBox(width: 8),
                          copy,
                          IconButton(
                            style: IconButton.styleFrom(
                              minimumSize: const Size(40, 40),
                              tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                            ),
                            tooltip: l10n.callMiniReturn,
                            onPressed: expand,
                            icon: const Icon(Icons.open_in_full, size: 18),
                          ),
                        ],
                      ),
                      SizedBox(
                        height: 28,
                        child: InkWell(
                          onTap: expand,
                          child: Row(
                            children: [
                              avatars,
                              const SizedBox(width: 8),
                              Expanded(
                                child: Text(
                                  participants.any((p) => p.isSpeaking)
                                      ? l10n.callMiniSpeaking(
                                          participants
                                                  .firstWhere(
                                                    (p) => p.isSpeaking,
                                                  )
                                                  .name
                                                  .isEmpty
                                              ? l10n.callMiniParticipant
                                              : participants
                                                    .firstWhere(
                                                      (p) => p.isSpeaking,
                                                    )
                                                    .name,
                                        )
                                      : l10n.callMiniParticipants(
                                          participants.length,
                                        ),
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: TextStyle(
                                    fontSize: 12,
                                    color: voice.textSecondary,
                                  ),
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                      const SizedBox(height: 4),
                      controls,
                    ],
                  ),
          ),
        ),
      ),
    );
  }
}

class _MiniParticipants extends StatelessWidget {
  const _MiniParticipants({required this.participants});
  final List<VoiceCallParticipant> participants;
  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final l10n = AppLocalizations.of(context)!;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final participant in participants)
          Semantics(
            label: participant.isSpeaking
                ? l10n.callMiniSpeaking(participant.name)
                : participant.name,
            child: Container(
              key: ValueKey('call_mini_participant_${participant.identity}'),
              margin: const EdgeInsets.only(right: 2),
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                border: Border.all(
                  width: 2,
                  color: participant.isSpeaking
                      ? voice.profileAccent
                      : voice.surface,
                ),
              ),
              child: VoiceAvatar(
                label: participant.name,
                imageUrl: participant.avatarUrl,
                radius: 10,
              ),
            ),
          ),
      ],
    );
  }
}
