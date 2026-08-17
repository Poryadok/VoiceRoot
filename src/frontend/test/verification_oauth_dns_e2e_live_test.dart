import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';

import 'support/live_gateway_harness.dart';

/// VR-02/VR-03: OAuth start + org DNS start (compose-partial; no real Twitch/DNS stub).
void main() {
  test(
    'verification OAuth start + org DNS TXT start (VR-02/03 partial)',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;
      final session = await ctx.registerUser('vr-live');

      final twitch = await ctx.httpClient.post(
        ctx.gatewayHttp().resolve('/api/v1/auth/linked-accounts/twitch/link'),
        headers: {
          'Authorization': session.authorizationHeader,
          'Content-Type': 'application/json',
        },
        body: jsonEncode({
          'redirect_uri': 'https://app.voice.test/oauth/twitch',
        }),
      );
      expect(twitch.statusCode, 200, reason: twitch.body);
      final twitchBody = jsonDecode(twitch.body) as Map<String, dynamic>;
      final twitchUrl = twitchBody['authorization_url'] as String? ??
          twitchBody['authorizationUrl'] as String?;
      expect(twitchUrl, contains('twitch'));

      final youtube = await ctx.httpClient.post(
        ctx.gatewayHttp().resolve('/api/v1/auth/linked-accounts/youtube/link'),
        headers: {
          'Authorization': session.authorizationHeader,
          'Content-Type': 'application/json',
        },
        body: jsonEncode({
          'redirect_uri': 'https://app.voice.test/oauth/youtube',
        }),
      );
      expect(youtube.statusCode, 200, reason: youtube.body);
      final ytBody = jsonDecode(youtube.body) as Map<String, dynamic>;
      final ytUrl = ytBody['authorization_url'] as String? ??
          ytBody['authorizationUrl'] as String?;
      expect(ytUrl, contains('google'));

      final org = await ctx.httpClient.post(
        ctx.gatewayHttp().resolve('/api/v1/users/me/verification/organization'),
        headers: {
          'Authorization': session.authorizationHeader,
          'Content-Type': 'application/json',
        },
        body: jsonEncode({
          'profile_id': session.activeProfileId,
          'domain': 'example.com',
        }),
      );
      expect(org.statusCode, 200, reason: org.body);
      final orgBody = jsonDecode(org.body) as Map<String, dynamic>;
      expect(orgBody['domain'], 'example.com');
      final txt = orgBody['txt_record'] as String? ??
          orgBody['txtRecord'] as String? ??
          '';
      expect(txt, contains('voice-verify='));
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
