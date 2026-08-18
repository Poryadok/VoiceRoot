import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

import 'support/live_gateway_harness.dart';

String liveVerificationStubUrl() {
  final fromEnv = Platform.environment['VOICE_VERIFICATION_STUB_URL'];
  if (fromEnv != null && fromEnv.trim().isNotEmpty) {
    return fromEnv.trim().replaceAll(RegExp(r'/$'), '');
  }
  return 'http://127.0.0.1:14180';
}

Map<String, dynamic> verificationStatusOf(Map<String, dynamic> body) {
  final nested =
      body['verification_status'] ?? body['verificationStatus'] ?? body;
  if (nested is Map<String, dynamic>) {
    return nested;
  }
  if (nested is Map) {
    return Map<String, dynamic>.from(nested);
  }
  return body;
}

/// VR-02/VR-03: OAuth callback via Helix/YPP stub + org DNS TXT grant.
void main() {
  test(
    'verification OAuth badge + org DNS TXT grant (VR-02/03)',
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
      final twitchUrl =
          twitchBody['authorization_url'] as String? ??
          twitchBody['authorizationUrl'] as String?;
      expect(twitchUrl, contains('twitch'));

      final twitchCb = await ctx.httpClient.post(
        ctx.gatewayHttp().resolve(
          '/api/v1/auth/linked-accounts/twitch/callback',
        ),
        headers: {
          'Authorization': session.authorizationHeader,
          'Content-Type': 'application/json',
        },
        body: jsonEncode({
          'code': 'compose-code',
          'redirect_uri': 'https://app.voice.test/oauth/twitch',
        }),
      );
      expect(twitchCb.statusCode, 200, reason: twitchCb.body);
      final twitchCbBody = jsonDecode(twitchCb.body) as Map<String, dynamic>;
      expect(
        twitchCbBody['verification_type'] ?? twitchCbBody['verificationType'],
        'personal',
      );
      expect(twitchCbBody['badge'], 'twitch');

      final twitchStatus = await ctx.httpClient.get(
        ctx.gatewayHttp().resolve('/api/v1/users/me/verification'),
        headers: {'Authorization': session.authorizationHeader},
      );
      expect(twitchStatus.statusCode, 200, reason: twitchStatus.body);
      final twitchGranted = verificationStatusOf(
        jsonDecode(twitchStatus.body) as Map<String, dynamic>,
      );
      expect(
        twitchGranted['verification_type'] ?? twitchGranted['verificationType'],
        'personal',
      );
      expect(twitchGranted['badge'], 'twitch');

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
      final ytUrl =
          ytBody['authorization_url'] as String? ??
          ytBody['authorizationUrl'] as String?;
      expect(ytUrl, contains('google'));

      final ytCb = await ctx.httpClient.post(
        ctx.gatewayHttp().resolve(
          '/api/v1/auth/linked-accounts/youtube/callback',
        ),
        headers: {
          'Authorization': session.authorizationHeader,
          'Content-Type': 'application/json',
        },
        body: jsonEncode({
          'code': 'compose-code',
          'redirect_uri': 'https://app.voice.test/oauth/youtube',
        }),
      );
      expect(ytCb.statusCode, 200, reason: ytCb.body);
      final ytCbBody = jsonDecode(ytCb.body) as Map<String, dynamic>;
      expect(
        ytCbBody['verification_type'] ?? ytCbBody['verificationType'],
        'personal',
      );
      expect(ytCbBody['badge'], 'youtube');

      final orgUser = await ctx.registerUser('vr-dns');
      final domain =
          'vr03-${DateTime.now().microsecondsSinceEpoch}.example.test';
      final org = await ctx.httpClient.post(
        ctx.gatewayHttp().resolve('/api/v1/users/me/verification/organization'),
        headers: {
          'Authorization': orgUser.authorizationHeader,
          'Content-Type': 'application/json',
        },
        body: jsonEncode({
          'profile_id': orgUser.activeProfileId,
          'domain': domain,
        }),
      );
      expect(org.statusCode, 200, reason: org.body);
      final orgBody = jsonDecode(org.body) as Map<String, dynamic>;
      expect(orgBody['domain'], domain);
      final txt =
          orgBody['txt_record'] as String? ??
          orgBody['txtRecord'] as String? ??
          '';
      expect(txt, contains('voice-verify='));

      final unpublished = await ctx.httpClient.post(
        ctx.gatewayHttp().resolve(
          '/api/v1/users/me/verification/organization/check',
        ),
        headers: {
          'Authorization': orgUser.authorizationHeader,
          'Content-Type': 'application/json',
        },
        body: jsonEncode({'profile_id': orgUser.activeProfileId}),
      );
      expect(unpublished.statusCode, 200, reason: unpublished.body);
      final unpublishedStatus = verificationStatusOf(
        jsonDecode(unpublished.body) as Map<String, dynamic>,
      );
      expect(
        unpublishedStatus['verification_type'] ??
            unpublishedStatus['verificationType'],
        isNot('organization'),
      );

      final putTxt = await http.put(
        Uri.parse('${liveVerificationStubUrl()}/dns-txt'),
        headers: {'Content-Type': 'application/json'},
        body: jsonEncode({'domain': domain, 'txt': txt}),
      );
      expect(putTxt.statusCode, 200, reason: putTxt.body);

      final published = await ctx.httpClient.post(
        ctx.gatewayHttp().resolve(
          '/api/v1/users/me/verification/organization/check',
        ),
        headers: {
          'Authorization': orgUser.authorizationHeader,
          'Content-Type': 'application/json',
        },
        body: jsonEncode({'profile_id': orgUser.activeProfileId}),
      );
      expect(published.statusCode, 200, reason: published.body);
      final granted = verificationStatusOf(
        jsonDecode(published.body) as Map<String, dynamic>,
      );
      expect(
        granted['verification_type'] ?? granted['verificationType'],
        'organization',
      );
      expect(granted['badge'], 'dns');
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
    timeout: const Timeout(Duration(minutes: 2)),
  );
}
