import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/auth_session.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';

sealed class ProfileSwitchResult {
  const ProfileSwitchResult();
}

final class ProfileSwitchApplied extends ProfileSwitchResult {
  const ProfileSwitchApplied(this.handoff);

  final ProfileSwitchHandoff handoff;
}

final class ProfileSwitchRejected extends ProfileSwitchResult {
  const ProfileSwitchRejected(this.errorCode);

  final String errorCode;
}

final class ProfileSwitchSuperseded extends ProfileSwitchResult {
  const ProfileSwitchSuperseded();
}

class ProfileSwitchHandoff {
  ProfileSwitchHandoff({
    required this.generation,
    required this.previousSession,
    required this.nextSession,
    required Set<String> retiredSubscriptionIds,
  }) : retiredSubscriptionIds = Set.unmodifiable(retiredSubscriptionIds);

  final int generation;
  final AuthSession previousSession;
  final AuthSession nextSession;
  final Set<String> retiredSubscriptionIds;

  String get nextAuthorization => nextSession.authorizationHeader;
}

abstract interface class ProfileSwitchRealtimeBoundary {
  Set<String> get activeSubscriptions;

  Future<void> retireAndReconnect(ProfileSwitchHandoff handoff);
}

class ProfileSwitchCoordinator {
  ProfileSwitchCoordinator({
    required AuthController auth,
    required AuthState Function() readAuthState,
    required StateController<String?> selectedChat,
    required ProfileSwitchRealtimeBoundary realtime,
  }) : _auth = auth,
       _readAuthState = readAuthState,
       _selectedChat = selectedChat,
       _realtime = realtime;

  final AuthController _auth;
  final AuthState Function() _readAuthState;
  final StateController<String?> _selectedChat;
  final ProfileSwitchRealtimeBoundary _realtime;
  var _generation = 0;

  Future<ProfileSwitchResult> switchTo(String profileId) async {
    final previousSession = _readAuthState().session;
    if (previousSession == null) {
      return const ProfileSwitchRejected('not_authenticated');
    }

    final generation = ++_generation;
    final error = await _auth.switchActiveProfile(profileId);
    if (generation != _generation) return const ProfileSwitchSuperseded();
    if (error != null) return ProfileSwitchRejected(error);

    final nextSession = _readAuthState().session;
    if (nextSession == null) {
      return const ProfileSwitchRejected('not_authenticated');
    }

    final handoff = ProfileSwitchHandoff(
      generation: generation,
      previousSession: previousSession,
      nextSession: nextSession,
      retiredSubscriptionIds: _realtime.activeSubscriptions,
    );
    _selectedChat.state = null;
    await _realtime.retireAndReconnect(handoff);
    if (generation != _generation) return const ProfileSwitchSuperseded();
    return ProfileSwitchApplied(handoff);
  }
}

class _RealtimeHubProfileSwitchBoundary
    implements ProfileSwitchRealtimeBoundary {
  _RealtimeHubProfileSwitchBoundary(this._hub);

  final RealtimeHub _hub;

  @override
  Set<String> get activeSubscriptions => _hub.subscribedChatIds;

  @override
  Future<void> retireAndReconnect(ProfileSwitchHandoff handoff) {
    return _hub.retireAndReconnect(handoff.retiredSubscriptionIds);
  }
}

final profileSwitchRealtimeBoundaryProvider =
    Provider<ProfileSwitchRealtimeBoundary>((ref) {
      return _RealtimeHubProfileSwitchBoundary(ref.read(realtimeHubProvider));
    });

final profileSwitchCoordinatorProvider = Provider<ProfileSwitchCoordinator>((
  ref,
) {
  return ProfileSwitchCoordinator(
    auth: ref.read(authControllerProvider.notifier),
    readAuthState: () => ref.read(authControllerProvider),
    selectedChat: ref.read(selectedChatIdProvider.notifier),
    realtime: ref.read(profileSwitchRealtimeBoundaryProvider),
  );
});
