import 'dart:async';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/friends_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/realtime_client.dart';

import 'support/live_gateway_harness.dart';

const _requestTimeout = Duration(seconds: 15);

const _runExistingAccountLive = String.fromEnvironment(
  'VOICE_RUN_EXISTING_ACCOUNT_FRIEND_REQUEST_WS',
  defaultValue: '',
);

/// One-shot staging check using a dedicated pair of existing test accounts.
/// The receiver declines the request during cleanup; Social retains a declined
/// outgoing record, so do not reuse this pair for another run.
///
/// Set VOICE_LIVE_REQUESTER_EMAIL/PASSWORD and
/// VOICE_LIVE_RECIPIENT_EMAIL/PASSWORD in the process environment, then run:
///
/// flutter test test/existing_accounts_friend_request_ws_live_test.dart \
///   --dart-define=VOICE_RUN_EXISTING_ACCOUNT_FRIEND_REQUEST_WS=true \
///   --dart-define=VOICE_API_BASE_URL=https://voice.comrade.click
void main() {
  test(
    'existing accounts deliver a friend request over the recipient WebSocket',
    () async {
      final requesterEmail = _requiredEnvironment('VOICE_LIVE_REQUESTER_EMAIL');
      final requesterPassword = _requiredEnvironment(
        'VOICE_LIVE_REQUESTER_PASSWORD',
      );
      final recipientEmail = _requiredEnvironment('VOICE_LIVE_RECIPIENT_EMAIL');
      final recipientPassword = _requiredEnvironment(
        'VOICE_LIVE_RECIPIENT_PASSWORD',
      );
      final requesterTotpCode = _optionalEnvironment(
        'VOICE_LIVE_REQUESTER_TOTP_CODE',
      );
      final recipientTotpCode = _optionalEnvironment(
        'VOICE_LIVE_RECIPIENT_TOTP_CODE',
      );

      final config = GatewayConfig(baseUrl: liveGatewayBaseUrl());
      final httpClient = http.Client();
      final gateway = GatewayHttpClient(httpClient: httpClient, config: config);
      final auth = VoiceAuthClient(gateway: gateway);
      final friends = VoiceFriendsClient(gateway: gateway);
      AuthSession? requesterSession;
      AuthSession? recipientSession;
      var invitationMayExist = false;

      addTearDown(() async {
        final cleanupErrors = <String>[];
        final requester = requesterSession;
        final recipient = recipientSession;
        if (invitationMayExist && requester != null && recipient != null) {
          final cleanup = await _declineIncomingRequest(
            friends,
            recipient,
            requester.activeProfileId,
          );
          if (cleanup != null) cleanupErrors.add(cleanup);
        }
        if (recipient != null) {
          final error = await _logoutTestSession(
            config,
            recipient,
            'recipient',
          );
          if (error != null) cleanupErrors.add(error);
        }
        if (requester != null) {
          final error = await _logoutTestSession(
            config,
            requester,
            'requester',
          );
          if (error != null) cleanupErrors.add(error);
        }
        httpClient.close();
        expect(cleanupErrors, isEmpty, reason: cleanupErrors.join('; '));
      });

      requesterSession = _expectLogin(
        await auth.login(
          email: requesterEmail,
          password: requesterPassword,
          totpCode: requesterTotpCode,
        ),
        'requester',
      );
      recipientSession = _expectLogin(
        await auth.login(
          email: recipientEmail,
          password: recipientPassword,
          totpCode: recipientTotpCode,
        ),
        'recipient',
      );

      await _requireCleanAccountPair(
        friends,
        requesterSession,
        recipientSession,
      );

      final realtime = VoiceRealtimeConnection(
        uri: gatewayWebSocketUri(config.baseUrl),
        headers: {'Authorization': recipientSession.authorizationHeader},
      );
      addTearDown(
        () => realtime.dispose().timeout(const Duration(seconds: 10)),
      );
      await realtime.connect();
      await waitForOp(realtime.events, 'hello');

      final notification = Completer<RealtimeFrame>();
      final frameSubscription = realtime.events.listen((frame) {
        final data = frame.data;
        if (frame.op == 'notification' &&
            data?['type'] == 'friend_request' &&
            data?['sender_profile_id'] == requesterSession!.activeProfileId &&
            !notification.isCompleted) {
          notification.complete(frame);
        }
      });
      addTearDown(frameSubscription.cancel);

      // Even a lost HTTP response may follow a successful server write; always
      // check for and decline the incoming request during teardown.
      invitationMayExist = true;
      final send = await friends
          .sendFriendInvitation(
            authorization: requesterSession.authorizationHeader,
            targetProfileId: recipientSession.activeProfileId,
          )
          .timeout(
            _requestTimeout,
            onTimeout: () =>
                throw TimeoutException('SendFriendInvitation timed out.'),
          );
      if (send case FriendsApiFailure(:final statusCode, :final errorCode)) {
        fail(
          'SendFriendInvitation failed: HTTP ${statusCode ?? 'unknown'} '
          '(error ${errorCode ?? 'unknown'}).',
        );
      }
      expect(send, isA<FriendsApiEmpty>(), reason: 'send one friend request');

      final frame = await notification.future.timeout(
        const Duration(seconds: 8),
        onTimeout: () => throw TestFailure(
          'timed out waiting for the matching friend_request notification',
        ),
      );
      expect(
        frame.data?['sender_profile_id'],
        requesterSession.activeProfileId,
      );
      expect(frame.data?['friend_request_id'], isNotEmpty);

      final requests = await _friendRequests(
        friends,
        recipientSession,
        'recipient post-send check',
      );
      expect(
        requests.incoming,
        contains(requesterSession.activeProfileId),
        reason: 'the request is visible in the REST snapshot without reload',
      );
    },
    timeout: const Timeout(Duration(minutes: 2)),
    skip: _runExistingAccountLive == 'true'
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_EXISTING_ACCOUNT_FRIEND_REQUEST_WS=true',
  );
}

String _requiredEnvironment(String name) {
  final value = _optionalEnvironment(name);
  if (value == null || value.isEmpty) {
    fail('Set $name in the test process environment.');
  }
  return value;
}

String? _optionalEnvironment(String name) {
  final value = Platform.environment[name]?.trim();
  return value == null || value.isEmpty ? null : value;
}

AuthSession _expectLogin(AuthSessionResult result, String role) {
  if (result is AuthSessionOk) return result.session;
  if (result is AuthSessionFailure) {
    fail(
      '$role login failed: HTTP ${result.statusCode ?? 'unknown'} '
      '(error ${result.errorCode ?? 'unknown'}).',
    );
  }
  fail('$role login returned an unexpected result.');
}

Future<void> _requireCleanAccountPair(
  VoiceFriendsClient friends,
  AuthSession requester,
  AuthSession recipient,
) async {
  if (requester.accountId == recipient.accountId) {
    fail('Use two different accounts; no friend request was sent.');
  }

  final requesterFriends = await _listFriendIds(
    friends,
    requester,
    'requester',
  );
  final recipientFriends = await _listFriendIds(
    friends,
    recipient,
    'recipient',
  );
  if (requesterFriends.contains(recipient.activeProfileId) ||
      recipientFriends.contains(requester.activeProfileId)) {
    fail('The accounts are already friends; no friend request was sent.');
  }

  final requesterRequests = await _friendRequests(
    friends,
    requester,
    'requester',
  );
  final recipientRequests = await _friendRequests(
    friends,
    recipient,
    'recipient',
  );
  final requestExists =
      requesterRequests.incoming.contains(recipient.activeProfileId) ||
      requesterRequests.outgoing.any(
        (item) => item.profileId == recipient.activeProfileId,
      ) ||
      recipientRequests.incoming.contains(requester.activeProfileId) ||
      recipientRequests.outgoing.any(
        (item) => item.profileId == requester.activeProfileId,
      );
  if (requestExists) {
    fail(
      'A request record already exists between the accounts; '
      'no friend request was sent. Use a fresh test-account pair.',
    );
  }
}

Future<Set<String>> _listFriendIds(
  VoiceFriendsClient friends,
  AuthSession session,
  String role,
) async {
  final ids = <String>{};
  final seenCursors = <String>{};
  String? cursor;
  do {
    final result = await friends
        .listFriends(
          authorization: session.authorizationHeader,
          pageSize: 100,
          cursor: cursor,
        )
        .timeout(
          _requestTimeout,
          onTimeout: () => throw TimeoutException(
            '$role friend-list preflight timed out; no request was sent.',
          ),
        );
    if (result is! FriendsApiOk<FriendsListData>) {
      fail('Could not verify friendship state; no friend request was sent.');
    }
    ids.addAll(result.data.friends);
    cursor = result.data.nextCursor;
    if (cursor != null && !seenCursors.add(cursor)) {
      fail('Friend list repeated a cursor; no friend request was sent.');
    }
  } while (cursor != null && cursor.isNotEmpty);
  return ids;
}

Future<FriendRequestsData> _friendRequests(
  VoiceFriendsClient friends,
  AuthSession session,
  String role,
) async {
  final result = await friends
      .listFriendRequests(authorization: session.authorizationHeader)
      .timeout(
        _requestTimeout,
        onTimeout: () =>
            throw TimeoutException('$role friend-request check timed out.'),
      );
  if (result is FriendsApiOk<FriendRequestsData>) return result.data;
  fail('Could not verify request state; no friend request was sent.');
}

Future<String?> _declineIncomingRequest(
  VoiceFriendsClient friends,
  AuthSession recipient,
  String requesterProfileId,
) async {
  try {
    final requests = await _friendRequests(
      friends,
      recipient,
      'recipient cleanup',
    );
    if (!requests.incoming.contains(requesterProfileId)) return null;
    final result = await friends
        .declineFriendInvitation(
          authorization: recipient.authorizationHeader,
          requesterProfileId: requesterProfileId,
        )
        .timeout(
          _requestTimeout,
          onTimeout: () => throw TimeoutException(
            'Declining the test friend request timed out.',
          ),
        );
    return result is FriendsApiEmpty
        ? null
        : 'recipient could not decline the test friend request';
  } on Object catch (error) {
    return 'friend request cleanup failed: ${error.runtimeType}';
  }
}

Future<String?> _logoutTestSession(
  GatewayConfig config,
  AuthSession session,
  String role,
) async {
  final cleanupClient = http.Client();
  try {
    final auth = VoiceAuthClient(
      gateway: GatewayHttpClient(httpClient: cleanupClient, config: config),
    );
    final error = await auth
        .logout(session: session)
        .timeout(
          _requestTimeout,
          onTimeout: () => throw TimeoutException('$role logout timed out.'),
        );
    return error == null ? null : '$role logout failed: $error';
  } on Object catch (error) {
    return '$role logout request failed: ${error.runtimeType}';
  } finally {
    cleanupClient.close();
  }
}
