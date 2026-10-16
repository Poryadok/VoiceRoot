import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/chats_client.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';

typedef DmPermissionRequest = ({
  String authorization,
  String accountId,
  String viewerProfileId,
  String targetProfileId,
});

/// A cache key bound to the authenticated viewer and the selected target.
/// Permission is caller-relative, so target-only caching is unsafe.
final dmPermissionProvider = FutureProvider.autoDispose
    .family<bool, DmPermissionRequest>((ref, request) async {
      final auth = ref.watch(authControllerProvider);
      final session = auth.session;
      if (session == null ||
          session.authorizationHeader != request.authorization ||
          session.accountId != request.accountId ||
          auth.activeProfileId != request.viewerProfileId) {
        throw const DmPermissionLoadException();
      }

      final result = await ref
          .watch(voiceChatsClientProvider)
          .canCreateDm(
            authorization: request.authorization,
            otherProfileId: request.targetProfileId,
          );
      return switch (result) {
        ChatsApiOk(:final data) => data,
        ChatsApiFailure() => throw const DmPermissionLoadException(),
      };
    });

/// The UI intentionally does not expose authorization-service details.
class DmPermissionLoadException implements Exception {
  const DmPermissionLoadException();
}
