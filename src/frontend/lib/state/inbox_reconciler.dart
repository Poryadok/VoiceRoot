import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/chats_client.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';

/// The three independent Chat `ListChats` inbox snapshots.
enum InboxScope { main, requests, archive }

/// The authoritative reconciliation state for one paginated inbox scope.
class InboxScopeSnapshot {
  const InboxScopeSnapshot({
    this.items = const [],
    this.nextCursor,
    this.failedCursor,
    this.errorMessage,
    this.errorStatusCode,
    this.isLoading = false,
    this.isComplete = false,
  });

  final List<ChatListItem> items;
  final String? nextCursor;

  /// The opaque cursor which failed. A null value means the first page failed.
  final String? failedCursor;
  final String? errorMessage;
  final int? errorStatusCode;
  final bool isLoading;
  final bool isComplete;

  bool get hasError => errorMessage != null;

  InboxScopeSnapshot copyWith({
    List<ChatListItem>? items,
    String? nextCursor,
    bool clearNextCursor = false,
    String? failedCursor,
    bool clearFailedCursor = false,
    String? errorMessage,
    int? errorStatusCode,
    bool clearError = false,
    bool? isLoading,
    bool? isComplete,
  }) {
    return InboxScopeSnapshot(
      items: items ?? this.items,
      nextCursor: clearNextCursor ? null : (nextCursor ?? this.nextCursor),
      failedCursor: clearFailedCursor
          ? null
          : (failedCursor ?? this.failedCursor),
      errorMessage: clearError ? null : (errorMessage ?? this.errorMessage),
      errorStatusCode: clearError
          ? null
          : (errorStatusCode ?? this.errorStatusCode),
      isLoading: isLoading ?? this.isLoading,
      isComplete: isComplete ?? this.isComplete,
    );
  }
}

/// All three snapshot scopes for one authenticated profile.
class ProfileInboxSnapshot {
  const ProfileInboxSnapshot({Map<InboxScope, InboxScopeSnapshot>? scopes})
    : scopes =
          scopes ??
          const {
            InboxScope.main: InboxScopeSnapshot(),
            InboxScope.requests: InboxScopeSnapshot(),
            InboxScope.archive: InboxScopeSnapshot(),
          };

  final Map<InboxScope, InboxScopeSnapshot> scopes;

  InboxScopeSnapshot operator [](InboxScope scope) => scopes[scope]!;

  ProfileInboxSnapshot withScope(InboxScope scope, InboxScopeSnapshot value) {
    return ProfileInboxSnapshot(scopes: {...scopes, scope: value});
  }
}

class InboxReconcilerState {
  const InboxReconcilerState({this.profileSnapshots = const {}});

  final Map<String, ProfileInboxSnapshot> profileSnapshots;

  ProfileInboxSnapshot? snapshotFor(String? profileId) {
    if (profileId == null) return null;
    return profileSnapshots[profileId];
  }

  InboxReconcilerState copyWith({
    Map<String, ProfileInboxSnapshot>? profileSnapshots,
  }) {
    return InboxReconcilerState(
      profileSnapshots: profileSnapshots ?? this.profileSnapshots,
    );
  }
}

/// Rebuilds Chat-owned inbox metadata after a Realtime reconnect.
///
/// This deliberately owns only Chat `ListChats` pagination. Message history is
/// still owned by the selected [ChatRoomController].
class InboxReconcilerController extends StateNotifier<InboxReconcilerState> {
  InboxReconcilerController(this._ref) : super(const InboxReconcilerState()) {
    _realtimeSub = _ref.listen<RealtimeLinkStatus>(realtimeLinkStatusProvider, (
      previous,
      next,
    ) {
      if (next != RealtimeLinkStatus.connected) return;
      final pending = _pendingSession;
      if (pending != null && _matchesSession(pending)) {
        _pendingSession = null;
        unawaited(reconcile());
        return;
      }
      if (previous == RealtimeLinkStatus.reconnecting) {
        unawaited(reconcile());
      }
    });
    _authSub = _ref.listen<AuthState>(authControllerProvider, (previous, next) {
      final previousSession = previous?.session;
      final nextSession = next.session;
      final profileChanged =
          previousSession?.activeProfileId != nextSession?.activeProfileId;
      final authorizationChanged =
          previousSession?.accessToken != nextSession?.accessToken;
      if (!profileChanged && !authorizationChanged) return;

      // Invalidate in-flight work synchronously. This also covers A -> B -> A,
      // where checking only the active profile id would otherwise accept the
      // first A response after returning to A.
      _generation++;
      _pendingItems.clear();
      _removedChatIds.clear();
      _pendingSession = nextSession == null
          ? null
          : _PendingInboxSession(
              profileId: nextSession.activeProfileId,
              authorization: nextSession.authorizationHeader,
            );
      if (profileChanged) {
        _ref.read(dmPeerProfileByChatIdProvider.notifier).state = const {};
      }
    });
  }

  final Ref _ref;
  ProviderSubscription<RealtimeLinkStatus>? _realtimeSub;
  ProviderSubscription<AuthState>? _authSub;
  int _generation = 0;
  _PendingInboxSession? _pendingSession;
  final Map<InboxScope, List<ChatListItem>> _pendingItems = {};
  final Map<String, Map<InboxScope, Set<String>>> _removedChatIds = {};
  final Map<String, Map<String, ChatListItem>> _archivedMutations = {};

  @override
  void dispose() {
    _realtimeSub?.close();
    _authSub?.close();
    super.dispose();
  }

  /// Starts a new full, independent snapshot of main, requests and archive.
  Future<void> reconcile() async {
    final session = _ref.read(authControllerProvider).session;
    if (session == null) return;
    final profileId = session.activeProfileId;
    final authorization = session.authorizationHeader;
    final generation = ++_generation;
    _pendingItems.clear();

    await Future.wait([
      for (final scope in InboxScope.values)
        _reconcileScope(
          generation: generation,
          profileId: profileId,
          authorization: authorization,
          scope: scope,
          cursor: null,
          replacesFirstPage: true,
        ),
    ]);
  }

  /// Retries precisely the page which last failed for [scope].
  Future<void> retry(InboxScope scope) async {
    final session = _ref.read(authControllerProvider).session;
    if (session == null) return;
    final profileId = session.activeProfileId;
    final current = state.profileSnapshots[profileId]?[scope];
    if (current == null || !current.hasError || current.isLoading) return;

    // A retry is scope-local: healthy scopes in the same snapshot generation
    // must continue loading while this exact opaque cursor is retried.
    final generation = _generation;
    await _reconcileScope(
      generation: generation,
      profileId: profileId,
      authorization: session.authorizationHeader,
      scope: scope,
      cursor: current.failedCursor ?? current.nextCursor,
      replacesFirstPage: current.failedCursor == null,
    );
  }

  /// Applies a successful Chat mutation to the visible authoritative snapshot.
  /// The mutation itself remains owned by the existing action controller.
  void removeChat(
    InboxScope scope,
    String chatId, {
    required String expectedProfileId,
    required String expectedAuthorization,
  }) {
    final session = _ref.read(authControllerProvider).session;
    if (session?.activeProfileId != expectedProfileId ||
        session?.authorizationHeader != expectedAuthorization) {
      return;
    }
    final profileId = expectedProfileId;
    _removedChatIds
        .putIfAbsent(profileId, () => <InboxScope, Set<String>>{})
        .putIfAbsent(scope, () => <String>{})
        .add(chatId);
    final profile = state.profileSnapshots[profileId];
    final current = profile?[scope];
    if (profile == null || current == null) return;
    final items = current.items
        .where((item) => item.chatId != chatId)
        .toList(growable: false);
    if (items.length == current.items.length) return;
    state = state.copyWith(
      profileSnapshots: {
        ...state.profileSnapshots,
        profileId: profile.withScope(scope, current.copyWith(items: items)),
      },
    );
    final pending = _pendingItems[scope];
    if (pending != null) {
      _pendingItems[scope] = pending
          .where((item) => item.chatId != chatId)
          .toList(growable: false);
    }
  }

  /// Applies a confirmed archive write to both authoritative inbox scopes.
  ///
  /// A successful server mutation belongs to the session that issued it. The
  /// same fence which protects pagination prevents a late A result from
  /// changing B after a profile switch.
  void archiveChat(
    ChatListItem item, {
    required String expectedProfileId,
    required String expectedAuthorization,
  }) {
    final session = _ref.read(authControllerProvider).session;
    if (session?.activeProfileId != expectedProfileId ||
        session?.authorizationHeader != expectedAuthorization) {
      return;
    }
    final profile = state.profileSnapshots[expectedProfileId];
    if (profile == null) return;

    // Retain a confirmed archive row while an older archive page is still in
    // flight. That page may complete, but cannot erase this newer mutation.
    _archivedMutations.putIfAbsent(
      expectedProfileId,
      () => <String, ChatListItem>{},
    )[item.chatId] = item;
    final main = profile[InboxScope.main];
    final archive = profile[InboxScope.archive];
    final updatedMain = main.copyWith(
      items: main.items.where((row) => row.chatId != item.chatId).toList(),
    );
    final updatedArchive = archive.copyWith(
      items: mergeInboxRows(archive.items, [item]),
      clearNextCursor: true,
      clearFailedCursor: true,
      clearError: true,
      isLoading: false,
      isComplete: true,
    );
    state = state.copyWith(
      profileSnapshots: {
        ...state.profileSnapshots,
        expectedProfileId: profile
            .withScope(InboxScope.main, updatedMain)
            .withScope(InboxScope.archive, updatedArchive),
      },
    );
  }

  Future<void> _reconcileScope({
    required int generation,
    required String profileId,
    required String authorization,
    required InboxScope scope,
    required String? cursor,
    required bool replacesFirstPage,
  }) async {
    var pageCursor = cursor;
    var replacesPage = replacesFirstPage;

    if (replacesFirstPage) _pendingItems[scope] = const [];
    _beginScope(
      generation: generation,
      profileId: profileId,
      scope: scope,
      resetCursor: replacesFirstPage,
    );
    if (!_isCurrent(generation, profileId)) return;

    while (true) {
      final requestedCursor = pageCursor;
      final result = await _ref
          .read(voiceChatsClientProvider)
          .listChats(
            authorization: authorization,
            cursor: requestedCursor,
            inbox: scope.name,
          );
      if (!_isCurrent(generation, profileId)) return;

      switch (result) {
        case ChatsApiOk(:final data):
          _commitPage(
            generation: generation,
            profileId: profileId,
            scope: scope,
            items: data.items,
            nextCursor: _nonEmptyCursor(data.nextCursor),
            replacesPage: replacesPage,
          );
          if (!_isCurrent(generation, profileId)) return;
          _syncDmPeers(
            generation: generation,
            profileId: profileId,
            scope: scope,
            items: data.items,
          );
          if (!_isCurrent(generation, profileId)) return;

          pageCursor = _nonEmptyCursor(data.nextCursor);
          if (pageCursor == null) {
            _completeScope(
              generation: generation,
              profileId: profileId,
              scope: scope,
            );
            _removedChatIds[profileId]?.remove(scope);
            if (_removedChatIds[profileId]?.isEmpty ?? false) {
              _removedChatIds.remove(profileId);
            }
            return;
          }
          replacesPage = false;
        case ChatsApiFailure(:final message, :final statusCode):
          _failScope(
            generation: generation,
            profileId: profileId,
            scope: scope,
            failedCursor: requestedCursor,
            message: message,
            statusCode: statusCode,
          );
          return;
      }
    }
  }

  String? _nonEmptyCursor(String? cursor) {
    return cursor == null || cursor.isEmpty ? null : cursor;
  }

  bool _isCurrent(int generation, String profileId) {
    return mounted &&
        generation == _generation &&
        _ref.read(authControllerProvider).activeProfileId == profileId;
  }

  bool _matchesSession(_PendingInboxSession pending) {
    final current = _ref.read(authControllerProvider).session;
    return current?.activeProfileId == pending.profileId &&
        current?.authorizationHeader == pending.authorization;
  }

  void _beginScope({
    required int generation,
    required String profileId,
    required InboxScope scope,
    required bool resetCursor,
  }) {
    if (!_isCurrent(generation, profileId)) return;
    final current =
        state.profileSnapshots[profileId]?[scope] ?? const InboxScopeSnapshot();
    _replaceScope(
      generation: generation,
      profileId: profileId,
      scope: scope,
      value: current.copyWith(
        clearNextCursor: resetCursor,
        clearFailedCursor: true,
        clearError: true,
        isLoading: true,
        isComplete: false,
      ),
    );
  }

  void _commitPage({
    required int generation,
    required String profileId,
    required InboxScope scope,
    required List<ChatListItem> items,
    required String? nextCursor,
    required bool replacesPage,
  }) {
    if (!_isCurrent(generation, profileId)) return;
    final current =
        state.profileSnapshots[profileId]?[scope] ?? const InboxScopeSnapshot();
    final removed =
        _removedChatIds[profileId]?[scope] ?? const <String>{};
    final acceptedItems = items
        .where((item) => !removed.contains(item.chatId))
        .toList(growable: false);
    final pending = replacesPage
        ? acceptedItems
        : mergeInboxRows(_pendingItems[scope] ?? const [], acceptedItems);
    final protectedPending = scope == InboxScope.archive
        ? mergeInboxRows(
            pending,
            _archivedMutations[profileId]?.values ?? const [],
          )
        : pending;
    _pendingItems[scope] = protectedPending;
    _replaceScope(
      generation: generation,
      profileId: profileId,
      scope: scope,
      value: current.copyWith(
        // Progressive pages render immediately, but old cached rows remain
        // until the scope reaches its terminal cursor.
        items: mergeInboxRows(current.items, protectedPending),
        nextCursor: nextCursor,
        clearNextCursor: nextCursor == null,
        clearFailedCursor: true,
        clearError: true,
        isLoading: true,
        isComplete: false,
      ),
    );
  }

  void _completeScope({
    required int generation,
    required String profileId,
    required InboxScope scope,
  }) {
    if (!_isCurrent(generation, profileId)) return;
    final current = state.profileSnapshots[profileId]![scope];
    final pendingItems = _pendingItems.remove(scope) ?? current.items;
    final authoritativeItems = scope == InboxScope.archive
        ? mergeInboxRows(
            pendingItems,
            _archivedMutations[profileId]?.values ?? const [],
          )
        : pendingItems;
    _replaceScope(
      generation: generation,
      profileId: profileId,
      scope: scope,
      value: current.copyWith(
        items: authoritativeItems,
        clearNextCursor: true,
        clearFailedCursor: true,
        clearError: true,
        isLoading: false,
        isComplete: true,
      ),
    );
  }

  void _failScope({
    required int generation,
    required String profileId,
    required InboxScope scope,
    required String? failedCursor,
    required String message,
    required int? statusCode,
  }) {
    if (!_isCurrent(generation, profileId)) return;
    final current =
        state.profileSnapshots[profileId]?[scope] ?? const InboxScopeSnapshot();
    _replaceScope(
      generation: generation,
      profileId: profileId,
      scope: scope,
      value: current.copyWith(
        nextCursor: failedCursor,
        clearNextCursor: failedCursor == null,
        failedCursor: failedCursor,
        clearFailedCursor: failedCursor == null,
        errorMessage: message,
        errorStatusCode: statusCode,
        isLoading: false,
        isComplete: false,
      ),
    );
  }

  void _replaceScope({
    required int generation,
    required String profileId,
    required InboxScope scope,
    required InboxScopeSnapshot value,
  }) {
    if (!_isCurrent(generation, profileId)) return;
    final profile =
        state.profileSnapshots[profileId] ?? const ProfileInboxSnapshot();
    // Check again immediately before the state side effect: a profile boundary
    // must not let an old async generation write another profile's snapshot.
    if (!_isCurrent(generation, profileId)) return;
    state = state.copyWith(
      profileSnapshots: {
        ...state.profileSnapshots,
        profileId: profile.withScope(scope, value),
      },
    );
  }

  void _syncDmPeers({
    required int generation,
    required String profileId,
    required InboxScope scope,
    required Iterable<ChatListItem> items,
  }) {
    if (!_isCurrent(generation, profileId)) return;
    final peers = Map<String, String>.from(
      _ref.read(dmPeerProfileByChatIdProvider),
    );
    var changed = false;
    for (final item in items) {
      if (_removedChatIds[profileId]?[scope]?.contains(item.chatId) ?? false) {
        continue;
      }
      final peerId = resolveDmPeerProfileId(
        item: item,
        // Current-profile server metadata is authoritative. A global cached
        // value may belong to the previous profile for the same chat id.
        knownPeerId: null,
        activeProfileId: profileId,
      );
      if (peerId == null || peers[item.chatId] == peerId) continue;
      peers[item.chatId] = peerId;
      changed = true;
    }
    if (!changed || !_isCurrent(generation, profileId)) return;
    _ref.read(dmPeerProfileByChatIdProvider.notifier).state = peers;
  }
}

class _PendingInboxSession {
  const _PendingInboxSession({
    required this.profileId,
    required this.authorization,
  });

  final String profileId;
  final String authorization;
}

/// Merges current rows with newer authoritative metadata while preserving order.
List<ChatListItem> mergeInboxRows(
  Iterable<ChatListItem> existing,
  Iterable<ChatListItem> incoming,
) {
  final byChatId = <String, ChatListItem>{
    for (final item in existing) item.chatId: item,
  };
  for (final item in incoming) {
    // Later pages are authoritative for row metadata but preserve its position.
    byChatId[item.chatId] = item;
  }
  return byChatId.values.toList(growable: false);
}

final inboxReconcilerProvider =
    StateNotifierProvider<InboxReconcilerController, InboxReconcilerState>(
      (ref) => InboxReconcilerController(ref),
    );
