import 'dart:io';
import 'dart:math';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/gateway_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/realtime_client.dart';

/// Compile-time API base (`--dart-define=VOICE_API_BASE_URL=...`).
String liveGatewayBaseUrl() {
  const fromEnv = String.fromEnvironment(
    'VOICE_API_BASE_URL',
    defaultValue: '',
  );
  if (fromEnv.isNotEmpty) return fromEnv;
  return 'http://127.0.0.1:18080';
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

/// RFC 4122 v4 UUID for [client_message_id] (Messaging rejects non-UUID values).
String qaClientMessageId() {
  final r = Random.secure();
  final b = List<int>.generate(16, (_) => r.nextInt(256));
  b[6] = (b[6] & 0x0f) | 0x40;
  b[8] = (b[8] & 0x3f) | 0x80;
  String hex(int v) => v.toRadixString(16).padLeft(2, '0');
  final h = b.map(hex).join();
  return '${h.substring(0, 8)}-${h.substring(8, 12)}-${h.substring(12, 16)}-${h.substring(16, 20)}-${h.substring(20)}';
}

/// Repo root (directory containing [docker-compose.yml]), or null if not found.
String? liveRepoRoot() {
  var dir = Directory.current;
  while (true) {
    if (File(
      '${dir.path}${Platform.pathSeparator}docker-compose.yml',
    ).existsSync()) {
      return dir.path;
    }
    final parent = dir.parent;
    if (parent.path == dir.path) {
      return null;
    }
    dir = parent;
  }
}

/// Clears Gateway auth rate-limit keys in compose Redis (dev stack only).
Future<void> clearLiveAuthRateLimit() async {
  if (!runLiveIntegration) {
    return;
  }
  final root = liveRepoRoot();
  if (root == null) {
    return;
  }
  const patterns = [
    'ratelimit:AuthLogin:*',
    'ratelimit:AuthRegister:*',
    'ratelimit:Auth:*',
  ];
  for (final pattern in patterns) {
    try {
      final scan = await Process.run(
        'docker',
        [
          'compose',
          'exec',
          '-T',
          'redis',
          'redis-cli',
          '--scan',
          '--pattern',
          pattern,
        ],
        workingDirectory: root,
        runInShell: Platform.isWindows,
      );
      if (scan.exitCode != 0) {
        continue;
      }
      final keys = (scan.stdout as String)
          .split('\n')
          .map((k) => k.trim())
          .where((k) => k.isNotEmpty);
      for (final key in keys) {
        await Process.run(
          'docker',
          ['compose', 'exec', '-T', 'redis', 'redis-cli', 'DEL', key],
          workingDirectory: root,
          runInShell: Platform.isWindows,
        );
      }
    } catch (_) {
      // Redis/compose unavailable — live tests may still hit 429.
    }
  }
}

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
  await clearLiveAuthRateLimit();

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

  final auth = VoiceAuthClient(
    gateway: GatewayHttpClient(httpClient: httpClient, config: config),
  );
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

  return LiveGatewayReady(
    LiveGatewayContext(config: config, httpClient: httpClient),
  );
}

class LiveGatewayContext {
  LiveGatewayContext({required this.config, required this.httpClient});

  final GatewayConfig config;
  final http.Client httpClient;

  VoiceAuthClient authClient() => VoiceAuthClient(
    gateway: GatewayHttpClient(httpClient: httpClient, config: config),
  );

  Future<AuthSession> registerUser(String prefix) async {
    final result = await authClient().register(
      email: qaUniqueEmail(prefix),
      password: qaPassword,
    );
    expect(result, isA<AuthSessionOk>(), reason: 'register $prefix');
    return (result as AuthSessionOk).session;
  }

  Future<AuthSession> refreshSession(AuthSession session) async {
    final result = await authClient().refresh(
      refreshToken: session.refreshToken,
    );
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
  Duration timeout = const Duration(seconds: 8),
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
