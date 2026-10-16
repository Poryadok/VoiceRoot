import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/api_errors.dart';
import '../../backend/friends_client.dart';
import '../../backend/users_client.dart';
import '../../state/auth_providers.dart';
import '../../state/social_providers.dart';

/// Accepted friend profile IDs for the Forward Message recipient picker.
///
/// This sheet-specific provider reads every page so local search covers the
/// complete accepted-friends list without changing the global friends-list
/// provider's single-page behavior.
final forwardMessageAcceptedFriendIdsProvider =
    FutureProvider.autoDispose<List<String>>((ref) async {
      var disposed = false;
      ref.onDispose(() => disposed = true);
      final session = ref.watch(
        authControllerProvider.select((state) => state.session),
      );
      if (session == null) throw StateError('not_authenticated');

      final client = ref.watch(voiceFriendsClientProvider);
      final profileIds = <String>{};
      final visitedCursors = <String>{};
      String? cursor;

      do {
        final page = await client.listFriends(
          authorization: session.authorizationHeader,
          cursor: cursor,
        );

        if (disposed) return const [];
        final currentSession = ref.read(authControllerProvider).session;
        if (currentSession?.activeProfileId != session.activeProfileId ||
            currentSession?.authorizationHeader !=
                session.authorizationHeader) {
          throw const ForwardContactsStaleSessionException();
        }

        switch (page) {
          case FriendsApiOk(:final data):
            profileIds.addAll(data.friends.where((id) => id.isNotEmpty));
            final nextCursor = data.nextCursor;
            if (nextCursor == null || nextCursor.isEmpty) {
              cursor = null;
            } else {
              if (!visitedCursors.add(nextCursor)) {
                throw StateError('friends_cursor_did_not_advance');
              }
              cursor = nextCursor;
            }
          case FriendsApiFailure(:final message):
            throw StateError(message);
          case FriendsApiEmpty():
            throw StateError('unexpected_empty_friends_response');
        }
      } while (cursor != null);

      return profileIds.toList(growable: false);
    });

class ForwardContactsStaleSessionException implements Exception {
  const ForwardContactsStaleSessionException();
}

class ForwardMessageContact {
  const ForwardMessageContact({required this.profileId, this.profile});

  final String profileId;
  final VoiceProfile? profile;

  String get displayName {
    final displayName = profile?.displayName.trim();
    if (displayName != null && displayName.isNotEmpty) return displayName;
    final username = profile?.username.trim();
    if (username != null && username.isNotEmpty) return '@$username';
    return profileId;
  }

  String get searchText =>
      '$displayName ${profile?.username ?? ''} ${profile?.discriminator ?? ''} $profileId'
          .toLowerCase();
}

class ForwardMessageContacts {
  const ForwardMessageContacts({
    required this.ownerProfileId,
    required this.contacts,
  });

  final String ownerProfileId;
  final List<ForwardMessageContact> contacts;
}

/// Resolves accepted friend names for complete local search, with bounded
/// concurrency over the existing single-profile User API.
final forwardMessageContactsProvider =
    FutureProvider.autoDispose<ForwardMessageContacts>((ref) async {
      var disposed = false;
      ref.onDispose(() => disposed = true);
      final session = ref.watch(
        authControllerProvider.select((state) => state.session),
      );
      if (session == null) throw StateError('not_authenticated');

      final profileIds = await ref.watch(
        forwardMessageAcceptedFriendIdsProvider.future,
      );
      if (disposed) {
        return ForwardMessageContacts(
          ownerProfileId: session.activeProfileId,
          contacts: const [],
        );
      }
      final contacts = <ForwardMessageContact>[];
      const requestBatchSize = 8;
      for (
        var start = 0;
        start < profileIds.length;
        start += requestBatchSize
      ) {
        if (disposed) break;
        final end = (start + requestBatchSize).clamp(0, profileIds.length);
        final batchIds = profileIds.sublist(start, end);
        final profiles = await Future.wait(
          batchIds.map((id) async {
            try {
              return await ref.read(profileProvider(id).future);
            } on ProfileUnavailableException {
              return null;
            }
          }),
        );

        if (disposed) break;
        final currentSession = ref.read(authControllerProvider).session;
        if (currentSession?.activeProfileId != session.activeProfileId ||
            currentSession?.authorizationHeader !=
                session.authorizationHeader) {
          throw const ForwardContactsStaleSessionException();
        }
        for (var index = 0; index < batchIds.length; index++) {
          contacts.add(
            ForwardMessageContact(
              profileId: batchIds[index],
              profile: profiles[index],
            ),
          );
        }
      }

      return ForwardMessageContacts(
        ownerProfileId: session.activeProfileId,
        contacts: contacts,
      );
    });
