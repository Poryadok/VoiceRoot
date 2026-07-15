import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/users_client.dart';
import '../state/auth_providers.dart';
import '../state/chat_providers.dart';
import '../state/matchmaking_providers.dart';
import '../state/matchmaking_search_controller.dart';
import '../state/shell_providers.dart';
import '../state/social_providers.dart';
import '../state/space_providers.dart';
import '../state/subscription_providers.dart';
import '../theme/voice_theme_providers.dart';

/// Side effects when [AuthState.activeProfileId] changes (multi-profile.md).
final profileContextCoordinatorProvider = Provider<void>((ref) {
  ref.listen<AuthState>(authControllerProvider, (previous, next) {
    final prevId = previous?.activeProfileId;
    final nextId = next.activeProfileId;
    if (prevId == null || nextId == null || prevId == nextId) return;
    if (!next.isAuthenticated) return;

    unawaited(_onActiveProfileChanged(ref));
  });
});

Future<void> _onActiveProfileChanged(Ref ref) async {
  ref.invalidate(activeProfileProvider);
  ref.invalidate(friendsListProvider);
  ref.invalidate(myProfilesProvider);
  ref.invalidate(subscriptionProvider);

  final hub = ref.read(realtimeHubProvider);
  await hub.reconnectWithNewSession();
  unawaited(ref.read(chatListControllerProvider.notifier).loadInitial());

  final auth = ref.read(authorizationHeaderProvider);
  final activeSearch = ref.read(activeSearchSessionProvider);
  if (auth != null && activeSearch != null) {
    unawaited(
      ref.read(voiceMatchmakingClientProvider).cancelSearch(
        authorization: auth,
        sessionId: activeSearch.id,
      ),
    );
    ref.read(activeSearchSessionProvider.notifier).state = null;
    ref.read(matchmakingSearchControllerProvider.notifier).clearRecovery();
  }

  final selectedSpaceId = ref.read(selectedSpaceIdProvider);
  if (selectedSpaceId != null) {
    try {
      final spaces = await ref.read(mySpacesProvider.future);
      final stillMember = spaces.spaces.any((s) => s.id == selectedSpaceId);
      if (!stillMember) {
        ref.read(shellNavigationProvider).exitSpace();
        ref.read(selectedChatIdProvider.notifier).state = null;
      }
    } on Object {
      ref.read(shellNavigationProvider).exitSpace();
      ref.read(selectedChatIdProvider.notifier).state = null;
    }
  }

  final profileId = ref.read(authControllerProvider).activeProfileId;
  if (profileId != null) {
    ref.invalidate(profileAccentColorProvider(profileId));
    unawaited(_migrateLocalAccentIfNeeded(ref, profileId));
  }
}

Future<void> _migrateLocalAccentIfNeeded(Ref ref, String profileId) async {
  final auth = ref.read(authorizationHeaderProvider);
  if (auth == null) return;
  final profile = await ref.read(profileProvider(profileId).future);
  if (profile == null) return;
  if (profile.accentColor != null && profile.accentColor!.isNotEmpty) return;

  final storage = ref.read(profileAccentStorageProvider);
  final override = await storage.readOverride(profileId);
  final index = await storage.readProfileIndex(profileId);
  String? hex = override;
  if (hex == null && index != null) {
    final catalog = await ref.read(voiceTokenCatalogProvider.future);
    hex = _colorToHex(catalog.profileAccentAt(index));
  }
  if (hex == null) return;

  final result = await ref.read(voiceUsersClientProvider).updateProfile(
    authorization: auth,
    accentColor: hex,
  );
  if (result is UsersApiOk) {
    await storage.clearOverride(profileId);
    ref.invalidate(profileProvider(profileId));
  }
}

String _colorToHex(Color color) {
  final v = color.toARGB32() & 0xFFFFFF;
  return '#${v.toRadixString(16).padLeft(6, '0').toUpperCase()}';
}
