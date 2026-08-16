import 'dart:convert';
import 'dart:typed_data';

import 'package:crypto/crypto.dart' as crypto;
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/users_client.dart';

import 'support/live_gateway_harness.dart';

/// SUB-06: premium webhook → GIF avatar + 3rd profile (live entitlements).
void main() {
  test(
    'premium cosmetics: GIF avatar + third profile after webhook',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final session = await ctx.registerUser('sub06-cosmetics');
      expect(session.accountId, isNotEmpty);

      final users = VoiceUsersClient(gateway: ctx.gatewayHttp());

      final freeGif = await users.createAvatarPresignedUpload(
        authorization: session.authorizationHeader,
        contentType: 'image/gif',
        contentLength: 128,
      );
      expect(freeGif, isA<UsersApiFailure>());

      await _activatePremiumWebhook(ctx, session.accountId);
      await _waitForPremiumPlan(ctx, session.authorizationHeader);

      if (await ctx.probeFileStorageAvailable(session)) {
        final gifBytes = Uint8List.fromList([
          0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00,
        ]);
        final premiumGif = await users.createAvatarPresignedUpload(
          authorization: session.authorizationHeader,
          contentType: 'image/gif',
          contentLength: gifBytes.length,
        );
        expect(
          premiumGif,
          isA<UsersApiOk<AvatarPresignedUpload>>(),
          reason: '$premiumGif',
        );
      }

      // Primary exists from register; create 2nd then 3rd under premium.
      final alt = await users.createProfile(
        authorization: session.authorizationHeader,
        displayName: 'Cosmetics Alt',
      );
      expect(alt, isA<UsersApiOk<VoiceProfile>>(), reason: '$alt');
      final third = await users.createProfile(
        authorization: session.authorizationHeader,
        displayName: 'Cosmetics Three',
      );
      expect(third, isA<UsersApiOk<VoiceProfile>>(), reason: '$third');
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}

Future<void> _activatePremiumWebhook(
  LiveGatewayContext ctx,
  String accountId,
) async {
  final eventId = 'evt_cosm_${DateTime.now().microsecondsSinceEpoch}';
  final body = jsonEncode({
    'event_id': eventId,
    'event_type': 'subscription.activated',
    'data': {
      'custom_data': {
        'account_id': accountId,
        'plan': 'premium',
      },
      'status': 'active',
    },
  });
  final signature = _signPaddleWebhook(body);
  final uri = ctx.gatewayHttp().resolve('/api/v1/subscription/webhooks/paddle');
  final resp = await ctx.httpClient.post(
    uri,
    headers: {
      'Content-Type': 'application/json',
      'Paddle-Signature': signature,
    },
    body: body,
  );
  expect(resp.statusCode, 200, reason: resp.body);
}

Future<void> _waitForPremiumPlan(
  LiveGatewayContext ctx,
  String authorization,
) async {
  for (var attempt = 0; attempt < 8; attempt++) {
    if (attempt > 0) {
      await Future<void>.delayed(Duration(milliseconds: 150 * attempt));
    }
    final uri = ctx.gatewayHttp().resolve('/api/v1/subscription/me');
    final resp = await ctx.httpClient.get(
      uri,
      headers: {'Authorization': authorization},
    );
    if (resp.statusCode != 200) continue;
    final parsed = jsonDecode(resp.body);
    if (parsed is Map &&
        parsed['subscription'] is Map &&
        (parsed['subscription'] as Map)['plan'] == 'premium') {
      return;
    }
  }
  fail('subscription did not become premium after webhook');
}

String _signPaddleWebhook(String body) {
  const secret = String.fromEnvironment(
    'PADDLE_WEBHOOK_SECRET',
    defaultValue: 'test_paddle_webhook_secret',
  );
  final ts = (DateTime.now().millisecondsSinceEpoch ~/ 1000).toString();
  final payload = '$ts:$body';
  final digest = crypto.Hmac(
    crypto.sha256,
    utf8.encode(secret),
  ).convert(utf8.encode(payload));
  return 'ts=$ts;h1=${digest.toString()}';
}
