import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/deep_links_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';

import 'support/live_gateway_harness.dart';

/// Phase 18 deep link invite E2E (API-level): universal link resolve after compose up.
void main() {
  test('phase 18 deeplink invite: HTML redirect and resolve kind=invite', () async {
    final probe = await probeLiveGateway();
    expect(
      probe,
      isA<LiveGatewayReady>(),
      reason: probe is LiveGatewayUnavailable ? probe.reason : null,
    );
    final ctx = (probe as LiveGatewayReady).context;

    final owner = await ctx.registerUser('p18-deeplink-owner');
    final spaces = ctx.spacesClient();
    final created = await spaces.createSpace(
      authorization: owner.authorizationHeader,
      name: 'Deep Link E2E',
    );
    expect(created, isA<SpacesApiOk<VoiceSpace>>());
    final spaceId = (created as SpacesApiOk<VoiceSpace>).data.id;

    final invite = await spaces.createInvite(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
    );
    expect(invite, isA<SpacesApiOk<SpaceInvite>>());
    final code = (invite as SpacesApiOk<SpaceInvite>).data.code;

    final links = VoiceDeepLinksClient(gateway: ctx.gatewayHttp());
    final html = await links.fetchInviteLanding(code: code);
    expect(html, isA<DeepLinksApiOk<String>>());
    expect(
      (html as DeepLinksApiOk<String>).data,
      contains('voice://invite/$code'),
    );

    final resolved = await links.resolve(
      authorization: owner.authorizationHeader,
      url: 'https://voice.gg/invite/$code',
    );
    expect(resolved, isA<DeepLinksApiOk<ResolvedDeepLink>>());
    final target = (resolved as DeepLinksApiOk<ResolvedDeepLink>).data;
    expect(target.kind, 'invite');
    expect(target.inviteCode, code);
    expect(target.spaceId, spaceId);
  }, skip: runLiveIntegration
      ? null
      : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true');
}
