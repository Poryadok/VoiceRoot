import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/spaces_client.dart';

import 'support/live_gateway_harness.dart';

/// Phase-5 space creation E2E (API-level): create, list, get, icon + description.
///
/// ```text
/// flutter test test/phase5_space_creation_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'space: create, list, get, icon and description',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final owner = await ctx.registerUser('space-owner');
      final spaces = ctx.spacesClient();

      final created = await spaces.createSpace(
        authorization: owner.authorizationHeader,
        name: 'Friday squad',
        description: 'We raid on Fridays',
      );
      expect(created, isA<SpacesApiOk<VoiceSpace>>());
      final space = (created as SpacesApiOk<VoiceSpace>).data;
      expect(space.name, 'Friday squad');
      expect(space.description, 'We raid on Fridays');
      expect(space.visibility, 'private');
      expect(space.ownerProfileId, owner.activeProfileId);

      final list = await spaces.listMySpaces(
        authorization: owner.authorizationHeader,
      );
      expect(list, isA<SpacesApiOk<SpaceListData>>());
      final items = (list as SpacesApiOk<SpaceListData>).data.spaces;
      expect(items.any((s) => s.id == space.id), isTrue);

      final got = await spaces.getSpace(
        authorization: owner.authorizationHeader,
        spaceId: space.id,
      );
      expect(got, isA<SpacesApiOk<VoiceSpace>>());
      expect((got as SpacesApiOk<VoiceSpace>).data.description, 'We raid on Fridays');

      const icon = 'https://cdn.voice.gg/spaces/flutter-e2e.webp';
      const desc = 'Updated about us';
      final updated = await spaces.updateSpace(
        authorization: owner.authorizationHeader,
        spaceId: space.id,
        iconUrl: icon,
        description: desc,
      );
      expect(updated, isA<SpacesApiOk<VoiceSpace>>());
      final after = (updated as SpacesApiOk<VoiceSpace>).data;
      expect(after.iconUrl, icon);
      expect(after.description, desc);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
