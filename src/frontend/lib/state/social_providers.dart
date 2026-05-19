import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/friends_client.dart';
import '../backend/users_client.dart';
import 'auth_providers.dart';
import 'gateway_providers.dart';

final voiceUsersClientProvider = Provider<VoiceUsersClient>((ref) {
  return VoiceUsersClient(
    httpClient: ref.watch(httpClientProvider),
    config: ref.watch(gatewayConfigProvider),
  );
});

final voiceFriendsClientProvider = Provider<VoiceFriendsClient>((ref) {
  return VoiceFriendsClient(
    httpClient: ref.watch(httpClientProvider),
    config: ref.watch(gatewayConfigProvider),
  );
});

final profileProvider =
    FutureProvider.family<VoiceProfile?, String>((ref, profileId) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) return null;
  final result =
      await ref.watch(voiceUsersClientProvider).getProfile(
            authorization: auth,
            profileId: profileId,
          );
  return switch (result) {
    UsersApiOk(:final data) => data,
    UsersApiFailure() => null,
  };
});

final presenceProvider =
    FutureProvider.family<VoicePresence?, String>((ref, profileId) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) return null;
  final result =
      await ref.watch(voiceUsersClientProvider).getPresence(
            authorization: auth,
            profileId: profileId,
          );
  return switch (result) {
    UsersApiOk(:final data) => data,
    UsersApiFailure() => null,
  };
});

class SearchProfilesState {
  const SearchProfilesState({
    this.results = const [],
    this.isLoading = false,
    this.errorMessage,
    this.lastQuery,
  });

  final List<VoiceProfile> results;
  final bool isLoading;
  final String? errorMessage;
  final String? lastQuery;

  SearchProfilesState copyWith({
    List<VoiceProfile>? results,
    bool? isLoading,
    String? errorMessage,
    bool clearError = false,
    String? lastQuery,
  }) {
    return SearchProfilesState(
      results: results ?? this.results,
      isLoading: isLoading ?? this.isLoading,
      errorMessage: clearError ? null : (errorMessage ?? this.errorMessage),
      lastQuery: lastQuery ?? this.lastQuery,
    );
  }
}

class SearchProfilesController extends StateNotifier<SearchProfilesState> {
  SearchProfilesController(this._ref) : super(const SearchProfilesState());

  final Ref _ref;

  Future<void> search(String query) async {
    final trimmed = query.trim();
    if (trimmed.isEmpty) {
      state = state.copyWith(results: [], clearError: true, lastQuery: '');
      return;
    }
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return;

    state = state.copyWith(isLoading: true, clearError: true, lastQuery: trimmed);
    final result = await _ref.read(voiceUsersClientProvider).searchProfiles(
          authorization: auth,
          query: trimmed,
        );
    switch (result) {
      case UsersApiOk(:final data):
        state = state.copyWith(
          results: data.profiles,
          isLoading: false,
          clearError: true,
        );
      case UsersApiFailure(:final message):
        state = state.copyWith(
          isLoading: false,
          errorMessage: message,
          results: [],
        );
    }
  }
}

final searchProfilesControllerProvider =
    StateNotifierProvider<SearchProfilesController, SearchProfilesState>((ref) {
  return SearchProfilesController(ref);
});

final friendsListProvider = FutureProvider<FriendsListData?>((ref) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) return null;
  final result =
      await ref.watch(voiceFriendsClientProvider).listFriends(authorization: auth);
  return switch (result) {
    FriendsApiOk(:final data) => data,
    FriendsApiFailure() => null,
  };
});

final friendRequestsProvider = FutureProvider<FriendRequestsData?>((ref) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) return null;
  final result = await ref
      .watch(voiceFriendsClientProvider)
      .listFriendRequests(authorization: auth);
  return switch (result) {
    FriendsApiOk(:final data) => data,
    FriendsApiFailure() => null,
  };
});

class SocialActions {
  SocialActions(this._ref);

  final Ref _ref;

  Future<String?> sendFriendInvitation(String targetProfileId) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final result = await _ref.read(voiceFriendsClientProvider).sendFriendInvitation(
          authorization: auth,
          targetProfileId: targetProfileId,
        );
    _invalidateSocialLists();
    return switch (result) {
      FriendsApiEmpty() => null,
      FriendsApiFailure(:final message) => message,
      FriendsApiOk() => null,
    };
  }

  Future<String?> acceptFriendInvitation(String requesterProfileId) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final result =
        await _ref.read(voiceFriendsClientProvider).acceptFriendInvitation(
              authorization: auth,
              requesterProfileId: requesterProfileId,
            );
    _invalidateSocialLists();
    return switch (result) {
      FriendsApiEmpty() => null,
      FriendsApiFailure(:final message) => message,
      FriendsApiOk() => null,
    };
  }

  Future<String?> declineFriendInvitation(String requesterProfileId) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final result =
        await _ref.read(voiceFriendsClientProvider).declineFriendInvitation(
              authorization: auth,
              requesterProfileId: requesterProfileId,
            );
    _invalidateSocialLists();
    return switch (result) {
      FriendsApiEmpty() => null,
      FriendsApiFailure(:final message) => message,
      FriendsApiOk() => null,
    };
  }

  void _invalidateSocialLists() {
    _ref.invalidate(friendsListProvider);
    _ref.invalidate(friendRequestsProvider);
  }
}

final socialActionsProvider = Provider<SocialActions>((ref) {
  return SocialActions(ref);
});
