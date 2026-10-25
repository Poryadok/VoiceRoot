import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/livekit_room.dart';
import 'call_providers.dart';
import 'social_providers.dart';
import 'chat_providers.dart';
import 'shell_providers.dart';
import 'space_providers.dart';

/// Room-scoped expansion prevents a finished call from expanding its successor.
final StateProvider<String?> callExpandedRoomProvider = StateProvider<String?>((
  ref,
) {
  final identity = ref.watch(
    callControllerProvider.select(
      (call) => (active: call.isActive, roomId: call.session?.roomId),
    ),
  );
  void navigated(Object? previous, Object? next) {
    if (previous != next) {
      ref.read(callExpandedRoomProvider.notifier).state = null;
    }
  }

  ref.listen(
    selectedChatIdProvider,
    (previous, next) => navigated(previous, next),
  );
  ref.listen(
    selectedSpaceIdProvider,
    (previous, next) => navigated(previous, next),
  );
  ref.listen(
    navigationSectionProvider,
    (previous, next) => navigated(previous, next),
  );
  ref.listen(
    mobileShellOverlayDepthProvider,
    (previous, next) => navigated(previous, next),
  );
  return identity.active ? identity.roomId : null;
});

/// Explicit minimize restores call focus; ordinary navigation retains its focus.
final callMiniFocusRequestProvider = StateProvider<int>((ref) => 0);

/// Local connection duration for room admissions without a server timestamp.
/// Media and mute updates do not restart the clock, nor does shell navigation.
final callConnectedAtProvider = Provider<DateTime?>((ref) {
  final identity = ref.watch(
    callControllerProvider.select(
      (call) => (
        active: call.phase == CallPhase.active,
        roomId: call.session?.roomId,
        startedAt: call.session?.startedAt,
      ),
    ),
  );
  return identity.active ? identity.startedAt ?? DateTime.now() : null;
});

final callParticipantsProvider = Provider<List<VoiceCallParticipant>>((ref) {
  ref.watch(callControllerProvider.select((value) => value.mediaTracksVersion));
  final call = ref.watch(callControllerProvider);
  if (!call.isActive) return const [];
  final room = ref.read(callControllerProvider.notifier).liveKitRoom;
  if (room is! VoiceCallPresentationRoom) return const [];
  return [
    for (final participant in room.callParticipants)
      if (participant.profileId case final String profileId)
        VoiceCallParticipant(
          identity: participant.identity,
          name:
              ref.watch(profileProvider(profileId)).valueOrNull?.displayName ??
              participant.name,
          avatarUrl: ref
              .watch(profileProvider(profileId))
              .valueOrNull
              ?.avatarUrl,
          profileId: profileId,
          isSpeaking: participant.isSpeaking,
        )
      else
        participant,
  ];
});
