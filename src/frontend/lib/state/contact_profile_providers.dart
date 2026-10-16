import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/spaces_client.dart';
import 'auth_providers.dart';
import 'space_providers.dart';

/// Derives mutual membership only from spaces the current caller may inspect.
/// Every page must succeed; a partial scan is never presented as an empty list.
final contactCommonSpacesProvider = FutureProvider.autoDispose
    .family<List<VoiceSpace>, String>((ref, targetProfileId) async {
      var disposed = false;
      ref.onDispose(() => disposed = true);
      final auth = ref.watch(authControllerProvider);
      final switching = ref.watch(profileSwitchInProgressProvider);
      final session = auth.session;
      if (session == null ||
          auth.isGuest ||
          switching ||
          targetProfileId == session.activeProfileId) {
        return const [];
      }

      void checkCurrent() {
        if (disposed) throw StateError('profile_context_changed');
        final current = ref.read(authControllerProvider).session;
        if (ref.read(profileSwitchInProgressProvider) ||
            current?.accountId != session.accountId ||
            current?.activeProfileId != session.activeProfileId ||
            current?.authorizationHeader != session.authorizationHeader) {
          throw StateError('profile_context_changed');
        }
      }

      final client = ref.watch(voiceSpacesClientProvider);
      const timeout = Duration(seconds: 15);
      final common = <VoiceSpace>[];
      final seenSpaces = <String>{};
      final spaceCursors = <String>{};
      String? spaceCursor;
      while (true) {
        checkCurrent();
        final page = await client
            .listMySpaces(
              authorization: session.authorizationHeader,
              cursor: spaceCursor,
              pageSize: 100,
            )
            .timeout(timeout);
        checkCurrent();
        if (page case SpacesApiFailure()) {
          throw StateError('profile_common_spaces_unavailable');
        }
        final spaces = (page as SpacesApiOk<SpaceListData>).data;
        for (final space in spaces.spaces) {
          if (space.id.isEmpty) {
            throw StateError('profile_common_space_missing_id');
          }
          if (!seenSpaces.add(space.id)) continue;
          final memberCursors = <String>{};
          String? memberCursor;
          while (true) {
            checkCurrent();
            final rosterPage = await client
                .listMembers(
                  authorization: session.authorizationHeader,
                  spaceId: space.id,
                  cursor: memberCursor,
                  pageSize: 100,
                )
                .timeout(timeout);
            checkCurrent();
            if (rosterPage case SpacesApiFailure()) {
              throw StateError('profile_common_spaces_unavailable');
            }
            final roster =
                (rosterPage as SpacesApiOk<SpaceMemberListData>).data;
            if (roster.members.any(
              (member) => member.profileId == targetProfileId,
            )) {
              common.add(space);
              break;
            }
            final next = roster.nextCursor;
            if (next == null || next.isEmpty) break;
            if (next == memberCursor || !memberCursors.add(next)) {
              throw StateError('profile_common_members_cursor_did_not_advance');
            }
            memberCursor = next;
          }
        }
        final next = spaces.nextCursor;
        if (next == null || next.isEmpty) break;
        if (next == spaceCursor || !spaceCursors.add(next)) {
          throw StateError('profile_common_spaces_cursor_did_not_advance');
        }
        spaceCursor = next;
      }
      return List.unmodifiable(common);
    });
