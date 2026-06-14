import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test(
    'phase 13 profiles + verification: multi-profile switch and verified search boost',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final session = await ctx.registerUser('p13-profiles');
      expect(session.accountId, isNotEmpty);
      expect(session.activeProfileId, isNotEmpty);

      final createAlt = await ctx.httpClient.post(
        ctx.gatewayHttp().resolve('/api/v1/users/profiles'),
        headers: {
          'Authorization': session.authorizationHeader,
          'Content-Type': 'application/json',
        },
        body: '{"display_name":"Gaming Persona"}',
      );
      expect(createAlt.statusCode, 200, reason: createAlt.body);
      final altBody = jsonDecode(createAlt.body) as Map<String, dynamic>;
      final altProfile = altBody['profile'] as Map<String, dynamic>;
      final altProfileId = altProfile['id'] as String;
      expect(altProfileId, isNotEmpty);

      final switchResp = await ctx.httpClient.post(
        ctx.gatewayHttp().resolve('/api/v1/auth/switch-profile'),
        headers: {
          'Authorization': session.authorizationHeader,
          'Content-Type': 'application/json',
        },
        body: jsonEncode({'profile_id': altProfileId}),
      );
      expect(switchResp.statusCode, 200, reason: switchResp.body);
      final switchBody = jsonDecode(switchResp.body) as Map<String, dynamic>;
      expect(switchBody['profile_id'], altProfileId);

      final verificationResp = await ctx.httpClient.get(
        ctx.gatewayHttp().resolve('/api/v1/users/me/verification'),
        headers: {'Authorization': 'Bearer ${switchBody['access_token']}'},
      );
      expect(verificationResp.statusCode, 200, reason: verificationResp.body);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
