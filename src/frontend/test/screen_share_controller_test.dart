import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/state/screen_share_providers.dart';

void main() {
  test('ScreenShareUiState tracks streams and selection', () {
    const a = ActiveScreenShare(
      roomId: 'room-1',
      profileId: 'profile-a',
      streamId: 'stream-1',
    );
    const b = ActiveScreenShare(
      roomId: 'room-1',
      profileId: 'profile-b',
      streamId: 'stream-2',
    );
    const state = ScreenShareUiState(
      streams: [a, b],
      selectedProfileId: 'profile-a',
    );
    expect(state.selectedStream?.streamId, 'stream-1');

    final afterStop = state.copyWith(
      streams: [b],
      selectedProfileId: 'profile-b',
    );
    expect(afterStop.streams, hasLength(1));
    expect(afterStop.selectedStream?.profileId, 'profile-b');
  });
}
