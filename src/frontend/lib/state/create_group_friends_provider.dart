import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/api_errors.dart';
import '../backend/friends_client.dart';
import 'auth_providers.dart';
import 'social_providers.dart';

/// Loads the complete friend list for Create Group without changing the
/// single-page semantics used by other friends-list consumers.
final createGroupFriendsProvider = FutureProvider.autoDispose<List<String>>((
  ref,
) async {
  final authorization = ref.watch(authorizationHeaderProvider);
  if (authorization == null) {
    throw StateError('not_authenticated');
  }

  final client = ref.watch(voiceFriendsClientProvider);
  final friends = <String>{};
  final seenCursors = <String>{};
  var disposed = false;
  var cursor = '';
  ref.onDispose(() => disposed = true);

  while (true) {
    final result = await client.listFriends(
      authorization: authorization,
      cursor: cursor.isEmpty ? null : cursor,
    );
    if (disposed) return List.unmodifiable(friends);

    switch (result) {
      case FriendsApiOk(:final data):
        friends.addAll(data.friends);
        final nextCursor = data.nextCursor;
        if (nextCursor == null || nextCursor.isEmpty) {
          return List.unmodifiable(friends);
        }
        if (!seenCursors.add(nextCursor)) {
          throw StateError('The friends list returned a repeated cursor.');
        }
        cursor = nextCursor;
      case FriendsApiFailure(:final statusCode)
          when isBackendUnavailable(statusCode):
        throw const BackendUnavailableException();
      case FriendsApiFailure(:final message):
        throw Exception(message);
      case FriendsApiEmpty():
        throw StateError('The friends list response was empty.');
    }
  }
});
