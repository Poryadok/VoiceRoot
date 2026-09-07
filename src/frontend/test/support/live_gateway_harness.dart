import 'dart:convert';
import 'dart:io';
import 'dart:math';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/files_client.dart';
import 'package:voice_frontend/backend/friends_client.dart';
import 'package:voice_frontend/backend/gateway_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/backend/roles_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/backend/user_privacy_client.dart';

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

Uri liveAuthMailStubBaseUrl() {
  final configured = Platform.environment['VOICE_AUTH_MAIL_STUB_URL']?.trim();
  if (configured != null && configured.isNotEmpty) {
    return Uri.parse(configured);
  }
  final port = Platform.environment['VERIFICATION_STUB_PORT']?.trim();
  return Uri.parse(
    'http://127.0.0.1:${port?.isNotEmpty == true ? port : '14180'}',
  );
}

Future<String> waitForLiveVerificationCode({
  required http.Client httpClient,
  required Uri stubBaseUrl,
  required String email,
  Duration pollInterval = const Duration(milliseconds: 100),
  int maxAttempts = 50,
}) async {
  final uri = stubBaseUrl
      .resolve('/emails/latest')
      .replace(queryParameters: {'to': email});
  for (var attempt = 0; attempt < maxAttempts; attempt += 1) {
    final response = await httpClient.get(uri);
    if (response.statusCode == 404) {
      if (attempt + 1 < maxAttempts) {
        await Future<void>.delayed(pollInterval);
      }
      continue;
    }
    if (response.statusCode != 200) {
      throw TestFailure(
        'verification mail lookup failed for $email: HTTP ${response.statusCode}',
      );
    }
    final body = jsonDecode(response.body);
    if (body is! Map<String, dynamic>) {
      throw TestFailure('verification mail lookup returned an invalid body');
    }
    final recipients = body['to'];
    if (recipients is! List ||
        !recipients.map((value) => '$value').contains(email)) {
      throw TestFailure(
        'verification mail lookup returned the wrong recipient',
      );
    }
    final match = RegExp(r'\b(\d{6})\b').firstMatch('${body['text'] ?? ''}');
    if (match == null) {
      throw TestFailure('verification mail did not contain a six-digit code');
    }
    return match.group(1)!;
  }
  throw TestFailure('verification mail was not delivered for $email');
}

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
    'ratelimit:FileUpload:*',
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

/// Reads `messages.content` from compose `messaging_db` (encryption (docs/features/encryption.md) ciphertext-at-rest).
///
/// Returns null when compose/postgres is unavailable; live tests should treat that
/// as a soft skip for the DB assertion only.
Future<String?> queryMessagingDbMessageContent(String messageId) async {
  if (!runLiveIntegration) {
    return null;
  }
  final root = liveRepoRoot();
  if (root == null) {
    return null;
  }
  try {
    final result = await Process.run(
      'docker',
      [
        'compose',
        'exec',
        '-T',
        'postgres',
        'psql',
        '-U',
        'voice',
        '-d',
        'messaging_db',
        '-t',
        '-A',
        '-c',
        "SELECT content FROM messages WHERE id = '$messageId'",
      ],
      workingDirectory: root,
      runInShell: Platform.isWindows,
    );
    if (result.exitCode != 0) {
      return null;
    }
    final content = (result.stdout as String).trim();
    return content.isEmpty ? null : content;
  } catch (_) {
    return null;
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

String _sanitizedGatewayFailure(GatewayHttpFailure failure) {
  final error = failure.error;
  return 'HTTP ${error.statusCode}';
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
    final email = qaUniqueEmail(prefix);
    final result = await authClient().register(
      email: email,
      password: qaPassword,
    );
    expect(result, isA<AuthSessionOk>(), reason: 'register $prefix');
    final pendingSession = (result as AuthSessionOk).session;
    expect(
      pendingSession.accountType,
      'guest',
      reason: 'pending email account',
    );
    final session = await completeEmailVerification(
      email: email,
      pendingSession: pendingSession,
    );
    // Default gaming preset blocks chat/space invites from non-friends
    // (docs/TESTING.md). Live E2E users are strangers unless a test tightens
    // privacy afterwards (e.g. privacy_actions).
    await allowOpenGamingPrivacy(session);
    return session;
  }

  Future<AuthSession> completeEmailVerification({
    required String email,
    required AuthSession pendingSession,
    Duration pendingRefreshInterval = const Duration(seconds: 5),
    int maxPendingRefreshAttempts = 24,
  }) async {
    final gateway = gatewayHttp();
    final sent = await gateway.postEmpty(
      uri: gateway.resolve('/api/v1/auth/otp/send'),
      authorization: pendingSession.authorizationHeader,
      jsonBody: {'email': email, 'otp_type': 'email_verify'},
    );
    expect(
      sent,
      isA<GatewayHttpOk<void>>(),
      reason: 'send email verification OTP',
    );

    final code = await waitForLiveVerificationCode(
      httpClient: httpClient,
      stubBaseUrl: liveAuthMailStubBaseUrl(),
      email: email,
    );
    final verified = await gateway.postJson(
      uri: gateway.resolve('/api/v1/auth/otp/verify'),
      authorization: pendingSession.authorizationHeader,
      body: {'email': email, 'code': code, 'otp_type': 'email_verify'},
    );

    AuthSession? regularSession;
    switch (verified) {
      case GatewayHttpOk(:final data):
        regularSession = AuthSession.fromAuthResponse(data);
      case GatewayHttpFailure(:final error)
          when error.statusCode == 503 &&
              error.errorCode == 'verification_pending':
        var current = pendingSession;
        for (
          var attempt = 0;
          attempt < maxPendingRefreshAttempts;
          attempt += 1
        ) {
          await Future<void>.delayed(pendingRefreshInterval);
          final refreshed = await authClient().refresh(
            refreshToken: current.refreshToken,
          );
          if (refreshed case AuthSessionOk(:final session)) {
            _expectSameLiveIdentity(pendingSession, session);
            current = session;
            if (session.accountType == 'regular') {
              regularSession = session;
              break;
            }
            continue;
          }
          if (refreshed case AuthSessionFailure(
            :final errorCode,
            :final statusCode,
          ) when statusCode == 503 && errorCode == 'verification_pending') {
            continue;
          }
          fail('refresh after verification_pending failed: $refreshed');
        }
      case GatewayHttpFailure(:final error):
        fail(
          'verify email OTP failed: ${error.errorCode} (HTTP ${error.statusCode})',
        );
    }

    if (regularSession == null) {
      fail('email verification remained pending after worker recovery window');
    }
    _expectSameLiveIdentity(pendingSession, regularSession);
    expect(
      regularSession.accountType,
      'regular',
      reason: 'verified email account',
    );
    await _waitForAccountVisibility(regularSession);
    return regularSession;
  }

  Future<void> _waitForAccountVisibility(AuthSession session) async {
    final deadline = DateTime.now().add(const Duration(seconds: 30));
    final gateway = gatewayHttp();
    while (true) {
      final result = await gateway.getJson(
        gateway.resolve('/api/v1/users/me'),
        authorization: session.authorizationHeader,
      );
      if (result is GatewayHttpOk) {
        return;
      }

      final failure = result as GatewayHttpFailure;
      final error = failure.error;
      final isVisibilityTransient =
          error.statusCode == HttpStatus.serviceUnavailable &&
          error.errorCode == 'unavailable' &&
          error.message == 'account visibility unavailable';
      if (!isVisibilityTransient || !DateTime.now().isBefore(deadline)) {
        fail(
          'verified account did not become visible: '
          '${_sanitizedGatewayFailure(failure)}',
        );
      }
      await Future<void>.delayed(const Duration(milliseconds: 250));
    }
  }

  void _expectSameLiveIdentity(AuthSession before, AuthSession after) {
    expect(
      after.accountId,
      before.accountId,
      reason: 'verified account identity',
    );
    expect(
      after.activeProfileId,
      before.activeProfileId,
      reason: 'verified primary profile identity',
    );
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

  GatewayHttpClient gatewayHttp() =>
      GatewayHttpClient(httpClient: httpClient, config: config);

  VoiceFriendsClient friendsClient() =>
      VoiceFriendsClient(gateway: gatewayHttp());

  VoiceChatsClient chatsClient() => VoiceChatsClient(gateway: gatewayHttp());

  VoiceSpacesClient spacesClient() => VoiceSpacesClient(gateway: gatewayHttp());

  VoiceRolesClient rolesClient() => VoiceRolesClient(gateway: gatewayHttp());

  VoiceMessagesClient messagesClient() =>
      VoiceMessagesClient(gateway: gatewayHttp());

  VoiceFilesClient filesClient() => VoiceFilesClient(gateway: gatewayHttp());

  VoiceUserPrivacyClient privacyClient() =>
      VoiceUserPrivacyClient(gateway: gatewayHttp());

  VoicePrivacySettings gamingOpenE2ePrivacy(String profileId) =>
      VoicePrivacySettings(
        profileId: profileId,
        preset: 'gaming',
        showOnline: VoicePrivacyAudience.everyoneWithGuests,
        showGameStatus: VoicePrivacyAudience.everyoneWithGuests,
        showMmRating: VoicePrivacyAudience.everyoneWithGuests,
        showPhone: VoicePrivacyAudience.nobody,
        showStories: VoicePrivacyAudience.everyoneWithGuests,
        allowDm: VoicePrivacyAudience.everyoneWithGuests,
        allowFriendRequests: VoicePrivacyAudience.everyoneWithGuests,
        allowGuestDm: true,
        allowPhoneSearch: VoicePrivacyAudience.friendsOnly,
        allowCalls: VoicePrivacyAudience.everyoneWithGuests,
        allowChatSpaceInvites: VoicePrivacyAudience.everyoneWithGuests,
        allowFiles: VoicePrivacyAudience.everyoneWithGuests,
        allowVoiceMessages: VoicePrivacyAudience.everyoneWithGuests,
      );

  Future<void> allowOpenGamingPrivacy(AuthSession session) async {
    final result = await privacyClient().updatePrivacy(
      authorization: session.authorizationHeader,
      settings: gamingOpenE2ePrivacy(session.activeProfileId),
    );
    expect(result, isA<UserPrivacyApiOk<VoicePrivacySettings>>());
  }

  Future<void> allowOpenGamingPrivacyMany(
    Iterable<AuthSession> sessions,
  ) async {
    for (final session in sessions) {
      await allowOpenGamingPrivacy(session);
    }
  }

  Future<VoiceChat> createDmBetween(
    AuthSession initiator,
    AuthSession peer,
  ) async {
    await allowOpenGamingPrivacyMany([initiator, peer]);
    final dm = await chatsClient().createDm(
      authorization: initiator.authorizationHeader,
      otherProfileId: peer.activeProfileId,
    );
    expect(dm, isA<ChatsApiOk<VoiceChat>>());
    return (dm as ChatsApiOk<VoiceChat>).data;
  }

  Future<bool> probeFileStorageAvailable(AuthSession session) async {
    final result = await filesClient().requestUpload(
      authorization: session.authorizationHeader,
      originalName: 'probe.txt',
      mimeType: 'text/plain',
      sizeBytes: 4,
    );
    return result is FilesApiOk<FileUploadTicket>;
  }

  Future<void> logoutSession(AuthSession session) async {
    final err = await authClient().logout(session: session);
    expect(err, isNull, reason: 'logout');
  }

  Future<int> protectedRouteStatus(String accessToken) async {
    final uri = gatewayHttp().resolve('/api/v1/chats');
    final resp = await httpClient.get(
      uri,
      headers: {'Authorization': 'Bearer $accessToken'},
    );
    return resp.statusCode;
  }

  Future<void> inviteAndAcceptFriends(
    AuthSession sessionA,
    AuthSession sessionB,
  ) async {
    final friends = friendsClient();
    final invite = await friends.sendFriendInvitation(
      authorization: sessionA.authorizationHeader,
      targetProfileId: sessionB.activeProfileId,
    );
    expect(invite, isA<FriendsApiEmpty>(), reason: 'friend invite');

    final accept = await friends.acceptFriendInvitation(
      authorization: sessionB.authorizationHeader,
      requesterProfileId: sessionA.activeProfileId,
    );
    expect(accept, isA<FriendsApiEmpty>(), reason: 'friend accept');
  }

  Future<VoiceRealtimeConnection> connectSubscribed(
    AuthSession session,
    String chatId,
  ) async {
    final conn = VoiceRealtimeConnection(
      uri: gatewayWebSocketUri(config.baseUrl),
      headers: {'Authorization': session.authorizationHeader},
    );
    await conn.connect();
    await waitForOp(conn.events, 'hello');
    conn.sendSubscribe(chatId);
    await waitForOp(
      conn.events,
      'subscribe_ack',
      where: (f) => f.data?['chat_id'] == chatId,
    );
    return conn;
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
