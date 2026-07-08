import 'dart:convert';

import 'package:crypto/crypto.dart' as crypto;
import 'package:flutter_test/flutter_test.dart';

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
      await _waitForPremiumPlan(ctx, session.authorizationHeader);

      if (!await _fileUploadAvailable(ctx, session.authorizationHeader)) {
        markTestSkipped(
          'object storage not configured (MinIO/R2); set FILE_R2_* in .env',
        );
      }

      expect(
        await _requestUploadStatus(
          ctx,
          session.authorizationHeader,
          _upload100MiB,
        ),
        200,
      );
      expect(
        await _requestUploadStatus(
          ctx,
          session.authorizationHeader,
          _upload250MiB,
        ),
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

Future<void> _waitForPremiumPlan(
  LiveGatewayContext ctx,
  String authorization,
) async {
  Object? lastFailure;
  for (var attempt = 0; attempt < 8; attempt++) {
    if (attempt > 0) {
      await Future<void>.delayed(Duration(milliseconds: 150 * attempt));
    }
    final plan = await _getSubscriptionPlan(ctx, authorization);
    if (plan == 'premium') {
      return;
    }
    lastFailure = 'plan=$plan';
  }
  fail('subscription did not become premium after webhook: $lastFailure');
}

Future<String> _getSubscriptionPlan(
  LiveGatewayContext ctx,
  String authorization,
) async {
  final uri = ctx.gatewayHttp().resolve('/api/v1/subscription/me');
  final resp = await ctx.httpClient.get(
    uri,
    headers: {'Authorization': authorization},
  );
  if (resp.statusCode != 200) {
    return 'http_${resp.statusCode}';
  }
  final parsed = jsonDecode(resp.body);
  if (parsed is! Map<String, dynamic>) {
    return '';
  }
  final subscription = parsed['subscription'];
  if (subscription is! Map<String, dynamic>) {
    return '';
  }
  final plan = subscription['plan'];
  return plan is String ? plan : '';
}

Future<bool> _fileUploadAvailable(
  LiveGatewayContext ctx,
  String authorization,
) async {
  return await _requestUploadStatus(ctx, authorization, 4) == 200;
}

Future<int> _requestUploadStatus(
  LiveGatewayContext ctx,
  String authorization,
  int sizeBytes,
) async {
  final uri = ctx.gatewayHttp().resolve('/api/v1/files/upload');
  final resp = await ctx.httpClient.post(
    uri,
    headers: {
      'Authorization': authorization,
      'Content-Type': 'application/json',
    },
    body: jsonEncode({
      'original_name': 'phase12-boundary.bin',
      'mime_type': 'application/octet-stream',
      'size_bytes': sizeBytes,
    }),
  );
  return resp.statusCode;
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
