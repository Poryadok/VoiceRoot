import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/voice_client.dart';
import 'auth_providers.dart';
import 'call_providers.dart';

typedef JoinVoiceRoomCallback =
    Future<void> Function({
      required String voiceRoomId,
      required String spaceId,
    });

/// Overridable hook for tree UI join; defaults to [CallController.joinVoiceRoom].
/// No-op by default; production wires LiveKit join in [main.dart].
final joinVoiceRoomActionProvider = Provider<JoinVoiceRoomCallback>(
  (ref) => ({required String voiceRoomId, required String spaceId}) async {},
);

/// Currently joined or selected space voice room in the tree UI.
final selectedVoiceRoomIdProvider = StateProvider<String?>((ref) => null);

class VoiceRoomParticipant {
  const VoiceRoomParticipant({
    required this.profileId,
    required this.displayName,
  });

  final String profileId;
  final String displayName;
}

final voiceRoomParticipantsProvider =
    FutureProvider.family<List<VoiceRoomParticipant>, String>((
  ref,
  voiceRoomId,
) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) {
    throw StateError('not_authenticated');
  }
  final result = await ref
      .read(voiceCallsClientProvider)
      .getVoiceRoomStates(authorization: auth, voiceRoomId: voiceRoomId);
  return switch (result) {
    VoiceApiOk(:final data) => data
        .map(
          (state) => VoiceRoomParticipant(
            profileId: state.profileId,
            displayName: state.profileId,
          ),
        )
        .toList(growable: false),
    VoiceApiFailure(:final message) => throw Exception(message),
  };
});
