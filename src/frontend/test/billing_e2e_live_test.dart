import 'dart:convert';

import 'package:crypto/crypto.dart' as crypto;
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/files_client.dart';
import 'package:voice_frontend/backend/subscription_client.dart';

import 'support/live_gateway_harness.dart';

const _upload100MiB = 100 << 20;
const _upload250MiB = 250 << 20;

void main() {
  test(
    'phase 12 billing: webhook premium, subscription me, upload boundaries',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final session = await ctx.registerUser('p12-billing');
      expect(session.accountId, isNotEmpty);

      await _activatePremiumWebhook(ctx, session.accountId);

      final subscription = VoiceSubscriptionClient(gateway: ctx.gatewayHttp());
      final me = await subscription.getSubscription(
        authorization: session.authorizationHeader,
      );
      expect(me, isA<SubscriptionApiOk<VoiceSubscription>>());
      final sub = (me as SubscriptionApiOk<VoiceSubscription>).data;
      expect(sub.plan, 'premium');
      expect(sub.isPremium, isTrue);

      if (!await ctx.probeFileStorageAvailable(session)) {
        markTestSkipped(
          'object storage not configured (MinIO/R2); set FILE_R2_* in .env',
        );
      }

      final files = ctx.filesClient();
      final ok = await files.requestUpload(
        authorization: session.authorizationHeader,
        originalName: 'phase12-100mb.bin',
        mimeType: 'application/octet-stream',
        sizeBytes: _upload100MiB,
      );
      expect(ok, isA<FilesApiOk<FileUploadTicket>>());

      final rejected = await files.requestUpload(
        authorization: session.authorizationHeader,
        originalName: 'phase12-250mb.bin',
        mimeType: 'application/octet-stream',
        sizeBytes: _upload250MiB,
      );
      expect(rejected, isA<FilesApiFailure>());
      expect(
        (rejected as FilesApiFailure).statusCode,
        400,
      );
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
  final eventId = 'evt_paddle_${DateTime.now().microsecondsSinceEpoch}';
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

String _signPaddleWebhook(String body) {
  const secret = String.fromEnvironment(
    'PADDLE_WEBHOOK_SECRET',
    defaultValue: 'test-webhook-secret',
  );
  const ts = '1700000000';
  final digest = crypto.Hmac(crypto.sha256, utf8.encode(secret))
      .convert(utf8.encode('$ts:$body'))
      .toString();
  return 'ts=$ts,h1=$digest';
}
