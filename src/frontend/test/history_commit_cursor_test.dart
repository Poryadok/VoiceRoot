import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/message_cache/in_memory_message_cache_store.dart';
import 'package:voice_frontend/backend/message_cache/message_cache_limits.dart';
import 'package:voice_frontend/gen/voice/messaging/v1/messaging.pb.dart'
    as wire;
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/message_cache_providers.dart';

import 'support/auth_test_overrides.dart';

const _chat = '20000000-0000-4000-8000-000000000001';
const _b = '019affff-ffff-7000-8000-000000000001';

void main() {
  for (final count in [5, 101]) {
    test(
      'deleted explicit anchor resets latest progress for $count rows',
      () async {
        final server = _CommitOrderedHistory();
        final cache = InMemoryMessageCacheStore();
        final container = _historyContainer(server, cache, () => false);
        final sub = container.listen(
          chatRoomControllerProvider(_chat),
          (_, _) {},
        );
        addTearDown(() {
          sub.close();
          container.dispose();
        });
        await pumpEventQueue(times: 100);
        server.commitLate(count);
        server.ids.remove(_b);
        final requests = server.requests.length;
        await _reconnect(container);
        expect(
          container
              .read(chatRoomControllerProvider(_chat))
              .historyCursor
              ?.messageId,
          server.ids.last,
          reason: 'latest fallback uses its first server-ordered row',
        );
        expect(
          server.requests.skip(requests).where((q) => q['cursor'] != null),
          isEmpty,
          reason: 'catch-up must not walk the fallback BEFORE chain',
        );
        final delivered = Map.of(server.deliveries);
        await _reconnect(container);
        expect(server.requests.last['last_message_id'], server.ids.last);
        expect(
          server.deliveries,
          delivered,
          reason: 'reset must not replay an old latest page',
        );
        server.commitLate(1);
        await _reconnect(container);
        expect(
          container
              .read(chatRoomControllerProvider(_chat))
              .historyCursor
              ?.messageId,
          server.ids.last,
        );
      },
    );
  }
  test(
    'deleted failed-continuation anchor resets to latest instead of walking backward',
    () async {
      final server = _CommitOrderedHistory()..failNextContinuation = true;
      final cache = InMemoryMessageCacheStore();
      final container = _historyContainer(server, cache, () => false);
      final sub = container.listen(
        chatRoomControllerProvider(_chat),
        (_, _) {},
      );
      addTearDown(() {
        sub.close();
        container.dispose();
      });
      await pumpEventQueue(times: 100);
      server.commitLate(101);
      await _reconnect(container);
      final saved = (await cache.getHistoryCursor(
        profileId: 'prof-test',
        chatId: _chat,
      ))!;
      expect(saved.pageCursor, isNotNull);
      server.ids.remove(saved.messageId);
      final requests = server.requests.length;
      await _reconnect(container);
      expect(server.requests[requests]['cursor'], saved.pageCursor);
      expect(
        container
            .read(chatRoomControllerProvider(_chat))
            .historyCursor
            ?.messageId,
        server.ids.last,
      );
      expect(
        server.requests.skip(requests).where((q) => q['cursor'] != null),
        hasLength(1),
      );
      final delivered = Map.of(server.deliveries);
      await _reconnect(container);
      expect(server.deliveries, delivered);
    },
  );
  test(
    'online restart resumes a failed accepted page before adopting latest progress',
    () async {
      final server = _CommitOrderedHistory()..failNextContinuation = true;
      final cache = InMemoryMessageCacheStore();
      final first = _historyContainer(server, cache, () => false);
      final firstSub = first.listen(
        chatRoomControllerProvider(_chat),
        (_, _) {},
      );
      await pumpEventQueue(times: 100);
      server.commitLate(101);
      await _reconnect(first);
      expect(server.deliveries, hasLength(50));
      final failedCursor = server.requests.last['cursor'];
      expect(failedCursor, isNotNull);
      firstSub.close();
      first.dispose();
      final requestCount = server.requests.length;
      final restored = _historyContainer(server, cache, () => false);
      final sub = restored.listen(chatRoomControllerProvider(_chat), (_, _) {});
      addTearDown(() {
        sub.close();
        restored.dispose();
      });
      await pumpEventQueue(times: 100);
      expect(
        server.deliveries,
        hasLength(101),
        reason: 'online startup must not skip the saved continuation',
      );
      expect(server.deliveries.values, everyElement(1));
      final delta = server.requests
          .skip(requestCount)
          .where(
            (q) =>
                q['cursor'] != null ||
                q['last_message_id'] != null ||
                q['after_message_id'] != null,
          );
      expect(delta, isNotEmpty);
      expect(delta.first['cursor'], failedCursor);
    },
  );
  test(
    'token round-trip cannot accept an earlier catch-up completion',
    () async {
      final server = _CommitOrderedHistory();
      final cache = InMemoryMessageCacheStore();
      final container = _historyContainer(server, cache, () => false);
      final sub = container.listen(
        chatRoomControllerProvider(_chat),
        (_, _) {},
      );
      addTearDown(() {
        sub.close();
        container.dispose();
      });
      await pumpEventQueue(times: 100);
      final gate = Completer<void>();
      server.deltaGate = gate;
      server.commitLate(1);
      await _reconnect(container);
      final original = container.read(authControllerProvider);
      final session = original.session!;
      container.read(authControllerProvider.notifier).state = AuthState(
        session: AuthSession(
          accessToken: 'different-session-token',
          refreshToken: session.refreshToken,
          accountId: session.accountId,
          activeProfileId: session.activeProfileId,
          expiresInSeconds: session.expiresInSeconds,
        ),
      );
      await pumpEventQueue();
      container.read(authControllerProvider.notifier).state = original;
      await pumpEventQueue();
      gate.complete();
      await pumpEventQueue(times: 100);
      expect(
        container.read(chatRoomControllerProvider(_chat)).messages.single.id,
        _b,
        reason: 'same token bytes do not restore ownership of an older request',
      );
      await _reconnect(container);
      expect(
        container.read(chatRoomControllerProvider(_chat)).messages,
        hasLength(2),
      );
    },
  );
  test(
    'full late-commit pages drain once and persisted cursor ignores display order',
    () async {
      final server = _CommitOrderedHistory();
      final cache = InMemoryMessageCacheStore();
      var offline = false;
      ProviderContainer createContainer() => ProviderContainer(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(MockClient(server.respond)),
          realtimeHubProvider.overrideWith(_NoopHub.new),
          messageCacheStoreProvider.overrideWithValue(cache),
          isDeviceOfflineProvider.overrideWith((ref) => offline),
          selectedChatIdProvider.overrideWith((ref) => _chat),
        ],
      );
      final container = createContainer();
      final sub = container.listen(
        chatRoomControllerProvider(_chat),
        (_, _) {},
      );
      await pumpEventQueue(times: 100);
      expect(
        container.read(chatRoomControllerProvider(_chat)).messages.single.id,
        _b,
      );

      // All 101 messages committed after B but sort before B by BOTH timestamps
      // and UUID. Two complete server pages cannot be skipped or replayed forever.
      server.commitLate(101);
      await _reconnect(container);
      final state = container.read(chatRoomControllerProvider(_chat));
      expect(
        state.messages,
        hasLength(102),
        reason: 'drain every returned next_cursor',
      );
      expect(
        state.messages.last.id,
        _b,
        reason: 'display order remains independent',
      );
      expect(server.deliveries.values, everyElement(1));
      expect(
        server.requests.map((r) => r['cursor']).whereType<String>(),
        hasLength(2),
      );
      sub.close();
      container.dispose();

      // A new controller restores its cursor from the same per-profile/chat cache,
      // rather than derive it from display-last B. No initial online latest fetch.
      offline = true;
      final restored = createContainer();
      final restoredSub = restored.listen(
        chatRoomControllerProvider(_chat),
        (_, _) {},
      );
      addTearDown(() {
        restoredSub.close();
        restored.dispose();
      });
      await pumpEventQueue(times: 100);
      expect(
        restored.read(chatRoomControllerProvider(_chat)).isOfflineCache,
        isTrue,
      );
      expect(
        restored.read(chatRoomControllerProvider(_chat)).messages,
        hasLength(kOfflineCacheMessageLimit),
      );
      expect(
        restored
            .read(chatRoomControllerProvider(_chat))
            .messages
            .map((m) => m.id),
        isNot(contains(server.ids.last)),
        reason:
            'server anchor survives eviction of its body from the display-sorted cache',
      );
      offline = false;
      restored.invalidate(isDeviceOfflineProvider);
      await _reconnect(restored);
      expect(server.requests.last['last_message_id'], server.ids.last);
      expect(
        server.deliveries.values,
        everyElement(1),
        reason: 'reconnect must not replay old pages',
      );
      server.commitLate(1);
      await _reconnect(restored);
      expect(
        restored.read(chatRoomControllerProvider(_chat)).messages,
        hasLength(kOfflineCacheMessageLimit + 1),
      );
      expect(server.deliveries.values, everyElement(1));
    },
  );
  test(
    'failed continuation resumes accepted full-page progress after cache restore',
    () async {
      final server = _CommitOrderedHistory()..failNextContinuation = true;
      final cache = InMemoryMessageCacheStore();
      var offline = false;
      ProviderContainer create() =>
          _historyContainer(server, cache, () => offline);
      final first = create();
      final firstSub = first.listen(
        chatRoomControllerProvider(_chat),
        (_, _) {},
      );
      await pumpEventQueue(times: 100);
      server.commitLate(101);
      await _reconnect(first);
      expect(server.deliveries, hasLength(50));
      final failedCursor = server.requests.last['cursor'];
      expect(failedCursor, isNotNull);
      expect(
        (await cache.getHistoryCursor(
          profileId: 'prof-test',
          chatId: _chat,
        ))?.pageCursor,
        failedCursor,
      );
      firstSub.close();
      first.dispose();
      offline = true;
      final restored = create();
      final sub = restored.listen(chatRoomControllerProvider(_chat), (_, _) {});
      addTearDown(() {
        sub.close();
        restored.dispose();
      });
      await pumpEventQueue(times: 100);
      final requestsBeforeRetry = server.requests.length;
      offline = false;
      restored.invalidate(isDeviceOfflineProvider);
      await _reconnect(restored);
      expect(server.requests[requestsBeforeRetry]['cursor'], failedCursor);
      expect(server.deliveries, hasLength(101));
      expect(server.deliveries.values, everyElement(1));
      expect(
        restored
            .read(chatRoomControllerProvider(_chat))
            .messages
            .map((m) => m.id),
        contains(server.ids.last),
      );
    },
  );
}

ProviderContainer _historyContainer(
  _CommitOrderedHistory server,
  InMemoryMessageCacheStore cache,
  bool Function() offline,
) => ProviderContainer(
  overrides: [
    authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
    authControllerProvider.overrideWith(authenticatedAuthController),
    gatewayConfigProvider.overrideWithValue(
      const GatewayConfig(baseUrl: 'http://api.test'),
    ),
    httpClientProvider.overrideWithValue(MockClient(server.respond)),
    realtimeHubProvider.overrideWith(_NoopHub.new),
    messageCacheStoreProvider.overrideWithValue(cache),
    isDeviceOfflineProvider.overrideWith((ref) => offline()),
    selectedChatIdProvider.overrideWith((ref) => _chat),
  ],
);

Future<void> _reconnect(ProviderContainer container) async {
  container.read(realtimeLinkStatusProvider.notifier).state =
      RealtimeLinkStatus.reconnecting;
  await pumpEventQueue();
  container.read(realtimeLinkStatusProvider.notifier).state =
      RealtimeLinkStatus.connected;
  await pumpEventQueue(times: 100);
}

class _CommitOrderedHistory {
  var _lateSerial = 0;
  Completer<void>? deltaGate;
  bool failNextContinuation = false;
  final ids = <String>[_b];
  final deliveries = <String, int>{};
  final requests = <Map<String, String>>[];

  void commitLate(int count) {
    for (var i = 0; i < count; i++) {
      ids.add(
        '019a0000-0000-7000-8000-${(999998 - _lateSerial++).toString().padLeft(12, '0')}',
      );
    }
  }

  Future<http.Response> respond(http.Request request) async {
    if (request.url.path != '/api/v1/messages' || request.method != 'GET') {
      return http.Response(
        '{}',
        request.method == 'GET' ? 404 : 200,
        headers: {'content-type': 'application/json'},
      );
    }
    final query = request.url.queryParameters;
    requests.add(Map.of(query));
    final anchor = query['after_message_id'] ?? query['last_message_id'];
    final cursor = query['cursor'];
    if (cursor != null && failNextContinuation) {
      failNextContinuation = false;
      return http.Response(
        '{"error_code":"unavailable","message":"retry"}',
        503,
        headers: {'content-type': 'application/json'},
      );
    }
    final isDelta = anchor != null || cursor != null;
    if (isDelta) {
      final gate = deltaGate;
      deltaGate = null;
      if (gate != null) await gate.future;
    }
    String? after = anchor;
    String? before;
    if (cursor != null) {
      final decoded =
          jsonDecode(utf8.decode(base64Url.decode(base64Url.normalize(cursor))))
              as Map<String, dynamic>;
      expect(decoded['c'], _chat);
      after = decoded['a'] as String?;
      before = decoded['b'] as String?;
    }
    final ref = after ?? before;
    final refIndex = ref == null ? -1 : ids.indexOf(ref);
    final reset = ref != null && refIndex < 0;
    final forward = after != null && !reset;
    final backward = before != null && !reset;
    final candidates = forward
        ? ids.skip(refIndex + 1).toList()
        : (backward ? ids.take(refIndex).toList() : ids).reversed.toList();
    final page = candidates.take(50).toList();
    final more = candidates.length > page.length;
    for (final id in page) {
      if (forward) deliveries.update(id, (n) => n + 1, ifAbsent: () => 1);
    }
    final result = wire.GetMessagesResponse(
      historyReset: reset,
      messageList: wire.MessageList(
        messages: page.map((id) {
          final message = wire.Message(
            id: id,
            senderProfileId: 'peer-1',
            content: id,
            createdAt: Timestamp.fromDateTime(
              DateTime.utc(id == _b ? 2026 : 2024),
            ),
          );
          message.ensureChat().id = _chat;
          return message;
        }),
        hasMore: more,
        nextCursor: more
            ? base64Url
                  .encode(
                    utf8.encode(
                      jsonEncode({'c': _chat, forward ? 'a' : 'b': page.last}),
                    ),
                  )
                  .replaceAll('=', '')
            : '',
      ),
    );
    return http.Response(
      jsonEncode(result.toProto3Json()),
      200,
      headers: {'content-type': 'application/json'},
    );
  }
}

class _NoopHub extends RealtimeHub {
  _NoopHub(super.ref);
  @override
  Future<void> ensureConnected() async {}
  @override
  void ensureSubscribed(String chatId) {}
  @override
  void markRead(String chatId, String messageId) {}
}
