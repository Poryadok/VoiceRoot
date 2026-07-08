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
      final sub = await _waitForPremiumSubscription(
        subscription,
        session.authorizationHeader,
      );
      expect(sub.plan, 'premium');
      expect(sub.status, anyOf('active', 'grace_period'));
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
      expect(
        ok,
        isA<FilesApiOk<FileUploadTicket>>(),
        reason: ok is FilesApiFailure
            ? '${(ok as FilesApiFailure).message} (HTTP ${ok.statusCode})'
            : null,
      );

      final rejected = await files.requestUpload(
        authorization: session.authorizationHeader,
        originalName: 'phase12-250mb.bin',
        mimeType: 'application/octet-stream',
        sizeBytes: _upload250MiB,
      );
      expect(
        rejected,
        isA<FilesApiFailure>(),
        reason: rejected is FilesApiOk<FileUploadTicket>
            ? '250MiB upload should be rejected for premium tier'
            : null,
      );
      final failure = rejected as FilesApiFailure;
      expect(
        failure.statusCode,
        400,
        reason: '${failure.message} (HTTP ${failure.statusCode})',
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

Future<VoiceSubscription> _waitForPremiumSubscription(
  VoiceSubscriptionClient client,
  String authorization,
) async {
  Object? lastFailure;
  for (var attempt = 0; attempt < 8; attempt++) {
    if (attempt > 0) {
      await Future<void>.delayed(Duration(milliseconds: 150 * attempt));
    }
    final me = await client.getSubscription(authorization: authorization);
    if (me is SubscriptionApiOk<VoiceSubscription>) {
      final sub = me.data;
      if (sub.plan == 'premium' && sub.isPremium) {
        return sub;
      }
      lastFailure =
          'plan=${sub.plan} status=${sub.status} isPremium=${sub.isPremium}';
      continue;
    }
    if (me is SubscriptionApiFailure) {
      lastFailure = '${me.message} (HTTP ${me.statusCode})';
    }
  }
  fail('subscription did not become premium after webhook: $lastFailure');
}

String _paddleWebhookSecret() {
  const secret = String.fromEnvironment('PADDLE_WEBHOOK_SECRET', defaultValue: '');
  return secret.isEmpty ? 'test-webhook-secret' : secret;
}

String _signPaddleWebhook(String body) {
  const ts = '1700000000';
  final digest = crypto.Hmac(crypto.sha256, utf8.encode(_paddleWebhookSecret()))
      .convert(utf8.encode('$ts:$body'))
      .toString();
  return 'ts=$ts,h1=$digest';
}
