import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/api_errors.dart';
import '../backend/friends_client.dart';
import '../backend/users_client.dart';
import 'auth_providers.dart';
import 'gateway_providers.dart';

final activeProfileProvider = FutureProvider<VoiceProfile?>((ref) async {
  final profileId = ref.watch(authControllerProvider).activeProfileId;
  if (profileId == null) return null;
  return ref.watch(profileProvider(profileId).future);
});

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

final profileProvider = FutureProvider.family<VoiceProfile?, String>((
  ref,
  profileId,
) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) return null;
  final result = await ref
      .watch(voiceUsersClientProvider)
      .getProfile(authorization: auth, profileId: profileId);
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
    this.errorStatusCode,
    this.lastQuery,
  });

  final List<VoiceProfile> results;
  final bool isLoading;
  final String? errorMessage;
  final int? errorStatusCode;
  final String? lastQuery;

  SearchProfilesState copyWith({
    List<VoiceProfile>? results,
    bool? isLoading,
    String? errorMessage,
    int? errorStatusCode,
    bool clearError = false,
    String? lastQuery,
  }) {
    return SearchProfilesState(
      results: results ?? this.results,
      isLoading: isLoading ?? this.isLoading,
      errorMessage: clearError ? null : (errorMessage ?? this.errorMessage),
      errorStatusCode: clearError
          ? null
          : (errorStatusCode ?? this.errorStatusCode),
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

    state = state.copyWith(
      isLoading: true,
      clearError: true,
      lastQuery: trimmed,
    );
    final result = await _ref
        .read(voiceUsersClientProvider)
        .searchProfiles(authorization: auth, query: trimmed);
    switch (result) {
      case UsersApiOk(:final data):
        state = state.copyWith(
          results: data.profiles,
          isLoading: false,
          clearError: true,
        );
      case UsersApiFailure(:final message, :final statusCode):
        state = state.copyWith(
          isLoading: false,
          errorMessage: message,
          errorStatusCode: statusCode,
          results: [],
        );
    }
  }
}

final searchProfilesControllerProvider =
    StateNotifierProvider<SearchProfilesController, SearchProfilesState>((ref) {
      return SearchProfilesController(ref);
    });

class ProfileActions {
  ProfileActions(this._ref);

  final Ref _ref;

  Future<String?> updateBasicProfile({
    required String displayName,
    required String bio,
    String? avatarUrl,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    final profileId = _ref.read(authControllerProvider).activeProfileId;
    if (auth == null || profileId == null) return 'not_authenticated';
    final result = await _ref
        .read(voiceUsersClientProvider)
        .updateProfile(
          authorization: auth,
          displayName: displayName,
          bio: bio,
          avatarUrl: avatarUrl,
        );
    return switch (result) {
      UsersApiOk() => _refreshActiveProfile(profileId),
      UsersApiFailure(:final message) => message,
    };
  }

  String? _refreshActiveProfile(String profileId) {
    _ref.invalidate(profileProvider(profileId));
    _ref.invalidate(activeProfileProvider);
    return null;
  }
}

final profileActionsProvider = Provider<ProfileActions>((ref) {
  return ProfileActions(ref);
});

final friendsListProvider = FutureProvider<FriendsListData>((ref) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) {
    throw StateError('not_authenticated');
  }
  final result = await ref
      .watch(voiceFriendsClientProvider)
      .listFriends(authorization: auth);
  return switch (result) {
    FriendsApiOk(:final data) => data,
    FriendsApiFailure(:final statusCode)
        when isBackendUnavailable(statusCode) =>
      throw const BackendUnavailableException(),
    FriendsApiFailure(:final message) => throw Exception(message),
  };
});

final friendRequestsProvider = FutureProvider<FriendRequestsData>((ref) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) {
    throw StateError('not_authenticated');
  }
  final result = await ref
      .watch(voiceFriendsClientProvider)
      .listFriendRequests(authorization: auth);
  return switch (result) {
    FriendsApiOk(:final data) => data,
    FriendsApiFailure(:final statusCode)
        when isBackendUnavailable(statusCode) =>
      throw const BackendUnavailableException(),
    FriendsApiFailure(:final message) => throw Exception(message),
  };
});

class SocialActions {
  SocialActions(this._ref);

  final Ref _ref;

  Future<String?> sendFriendInvitation(String targetProfileId) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final result = await _ref
        .read(voiceFriendsClientProvider)
        .sendFriendInvitation(
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
    final result = await _ref
        .read(voiceFriendsClientProvider)
        .acceptFriendInvitation(
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
    final result = await _ref
        .read(voiceFriendsClientProvider)
        .declineFriendInvitation(
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
