import 'dart:math';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/gateway_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/realtime_client.dart';

/// Compile-time API base (`--dart-define=VOICE_API_BASE_URL=...`).
String liveGatewayBaseUrl() {
  const fromEnv = String.fromEnvironment('VOICE_API_BASE_URL', defaultValue: '');
  if (fromEnv.isNotEmpty) return fromEnv;
  return 'http://127.0.0.1:8080';
}

/// Opt-in: `--dart-define=VOICE_RUN_LIVE_INTEGRATION=true`.
bool get runLiveIntegration {
  const flag = String.fromEnvironment(
    'VOICE_RUN_LIVE_INTEGRATION',
    defaultValue: '',
  );
  return flag == 'true' || flag == '1';
}

String qaUniqueEmail(String prefix) {
  final n = DateTime.now().microsecondsSinceEpoch;
  final r = Random().nextInt(0xFFFFFF);
  return '$prefix-$n-$r@voice-qa.test';
}

const qaPassword = 'VoiceQaTest1!';

sealed class LiveGatewayProbe {
  const LiveGatewayProbe();
}

final class LiveGatewayReady extends LiveGatewayProbe {
  const LiveGatewayReady(this.context);
  final LiveGatewayContext context;
}

final class LiveGatewayUnavailable extends LiveGatewayProbe {
  const LiveGatewayUnavailable(this.reason);
  final String reason;
}

/// Probes Gateway + Auth upstream (call only when [runLiveIntegration] is true).
Future<LiveGatewayProbe> probeLiveGateway() async {
  final config = GatewayConfig(baseUrl: liveGatewayBaseUrl());
  final httpClient = http.Client();
  final gateway = VoiceGatewayClient(httpClient: httpClient, config: config);
  final health = await gateway.fetchHealth();
  if (health is! GatewayHealthOk) {
    return LiveGatewayUnavailable(
      'Gateway health failed at ${config.baseUrl}: '
      '${(health as GatewayHealthFailure).message}',
    );
  }

  final auth = VoiceAuthClient(httpClient: httpClient, config: config);
  final probe = await auth.register(
    email: qaUniqueEmail('probe'),
    password: qaPassword,
  );
  if (probe is AuthSessionFailure) {
    final code = probe.statusCode;
    if (code == 404 || code == 502 || code == 503) {
      return LiveGatewayUnavailable(
        'Auth upstream not reachable via Gateway (${config.baseUrl}): '
        '${probe.message} (HTTP $code)',
      );
    }
    fail('Auth register probe failed: ${probe.message} (HTTP $code)');
  }

  return LiveGatewayReady(LiveGatewayContext(config: config, httpClient: httpClient));
}

class LiveGatewayContext {
  LiveGatewayContext({required this.config, required this.httpClient});

  final GatewayConfig config;
  final http.Client httpClient;

  VoiceAuthClient authClient() =>
      VoiceAuthClient(httpClient: httpClient, config: config);

  Future<AuthSession> registerUser(String prefix) async {
    final result = await authClient().register(
      email: qaUniqueEmail(prefix),
      password: qaPassword,
    );
    expect(result, isA<AuthSessionOk>(), reason: 'register $prefix');
    return (result as AuthSessionOk).session;
  }

  Future<AuthSession> refreshSession(AuthSession session) async {
    final result = await authClient().refresh(refreshToken: session.refreshToken);
    expect(result, isA<AuthSessionOk>(), reason: 'refresh session');
    final refreshed = (result as AuthSessionOk).session;
    expect(refreshed.accessToken, isNot(session.accessToken));
    expect(refreshed.activeProfileId, session.activeProfileId);
    return refreshed;
  }
}

Future<RealtimeFrame> waitForOp(
  Stream<RealtimeFrame> events,
  String op, {
  Duration timeout = const Duration(seconds: 20),
  bool Function(RealtimeFrame frame)? where,
}) {
  return events
      .where((f) => f.op == op && (where == null || where(f)))
      .first
      .timeout(
        timeout,
        onTimeout: () => throw TestFailure('timeout waiting for op=$op'),
      );
}
