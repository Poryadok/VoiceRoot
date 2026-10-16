import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/api_errors.dart';
import '../backend/chats_client.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';

const int kGroupMembersPageSize = 500;
const String kGroupMembersStaleContext = 'stale_context';

class GroupMembersAuthContext {
  const GroupMembersAuthContext({
    required this.authorization,
    required this.accountId,
    required this.profileId,
  });

  final String authorization;
  final String accountId;
  final String profileId;
}

class GroupMembersManagementState {
  const GroupMembersManagementState({
    this.members = const <ChatMember>[],
    this.isLoading = false,
    this.error,
    this.isMutating = false,
  });

  final List<ChatMember> members;
  final bool isLoading;
  final Object? error;
  final bool isMutating;

  GroupMembersManagementState copyWith({
    List<ChatMember>? members,
    bool? isLoading,
    Object? error,
    bool clearError = false,
    bool? isMutating,
  }) => GroupMembersManagementState(
    members: members ?? this.members,
    isLoading: isLoading ?? this.isLoading,
    error: clearError ? null : (error ?? this.error),
    isMutating: isMutating ?? this.isMutating,
  );
}

final groupMembersManagementProvider =
    StateNotifierProvider.family<
      GroupMembersManagementController,
      GroupMembersManagementState,
      String
    >((ref, chatId) {
      final controller = GroupMembersManagementController(ref, chatId);
      ref.listen(authControllerProvider, (previous, next) {
        final before = previous?.session;
        final after = next.session;
        if (before?.authorizationHeader != after?.authorizationHeader ||
            before?.accountId != after?.accountId ||
            before?.activeProfileId != after?.activeProfileId) {
          controller.load();
        }
      });
      controller.load();
      return controller;
    });

class GroupMembersManagementController
    extends StateNotifier<GroupMembersManagementState> {
  GroupMembersManagementController(this._ref, this.chatId)
    : super(const GroupMembersManagementState(isLoading: true));

  final Ref _ref;
  final String chatId;

  GroupMembersAuthContext? captureContext() {
    final session = _ref.read(authControllerProvider).session;
    if (session == null || session.activeProfileId.isEmpty) return null;
    return GroupMembersAuthContext(
      authorization: session.authorizationHeader,
      accountId: session.accountId,
      profileId: session.activeProfileId,
    );
  }

  bool isCurrentContext(GroupMembersAuthContext expected) {
    final session = _ref.read(authControllerProvider).session;
    return session != null &&
        session.authorizationHeader == expected.authorization &&
        session.accountId == expected.accountId &&
        session.activeProfileId == expected.profileId;
  }

  Future<void> load({GroupMembersAuthContext? expectedContext}) async {
    final expected = expectedContext ?? captureContext();
    if (expected == null) {
      state = state.copyWith(isLoading: false, error: 'not_authenticated');
      return;
    }
    state = state.copyWith(
      members: const <ChatMember>[],
      isLoading: true,
      clearError: true,
      isMutating: false,
    );
    final members = <ChatMember>[];
    final seenCursors = <String>{};
    String? cursor;
    try {
      while (true) {
        final result = await _ref
            .read(voiceChatsClientProvider)
            .listGroupMembers(
              authorization: expected.authorization,
              chatId: chatId,
              cursor: cursor,
              pageSize: kGroupMembersPageSize,
            );
        if (!mounted || !isCurrentContext(expected)) return;
        switch (result) {
          case ChatsApiFailure(:final statusCode):
            state = state.copyWith(
              isLoading: false,
              error: isBackendUnavailable(statusCode)
                  ? const BackendUnavailableException()
                  : StateError('group_members_load_failed'),
            );
            return;
          case ChatsApiOk(:final data):
            members.addAll(data.members);
            final next = data.nextCursor;
            if (next == null || next.isEmpty) {
              state = state.copyWith(
                members: List<ChatMember>.unmodifiable(members),
                isLoading: false,
                clearError: true,
              );
              return;
            }
            if (!seenCursors.add(next)) {
              state = state.copyWith(
                isLoading: false,
                error: 'invalid_pagination',
              );
              return;
            }
            cursor = next;
        }
      }
    } catch (error) {
      if (mounted && isCurrentContext(expected)) {
        state = state.copyWith(
          isLoading: false,
          error: error is BackendUnavailableException
              ? error
              : StateError('group_members_load_failed'),
        );
      }
    }
  }

  Future<String?> addMembers(
    List<String> profileIds, {
    required GroupMembersAuthContext expectedContext,
  }) => _mutate(
    expectedContext,
    (authorization) => _ref
        .read(voiceChatsClientProvider)
        .addGroupMembers(
          authorization: authorization,
          chatId: chatId,
          profileIds: profileIds,
        ),
  );

  Future<String?> removeMember(
    String profileId, {
    required GroupMembersAuthContext expectedContext,
  }) => _mutate(
    expectedContext,
    (authorization) => _ref
        .read(voiceChatsClientProvider)
        .removeGroupMember(
          authorization: authorization,
          chatId: chatId,
          profileId: profileId,
        ),
  );

  Future<String?> transferOwnership(
    String profileId, {
    required GroupMembersAuthContext expectedContext,
  }) => _mutate(
    expectedContext,
    (authorization) => _ref
        .read(voiceChatsClientProvider)
        .transferGroupOwnership(
          authorization: authorization,
          chatId: chatId,
          newOwnerProfileId: profileId,
        ),
  );

  Future<String?> leaveGroup({
    required GroupMembersAuthContext expectedContext,
  }) => _mutate(
    expectedContext,
    (authorization) => _ref
        .read(voiceChatsClientProvider)
        .leaveGroup(authorization: authorization, chatId: chatId),
  );

  Future<String?> _mutate(
    GroupMembersAuthContext expected,
    Future<ChatsApiResult<void>> Function(String authorization) request,
  ) async {
    if (!isCurrentContext(expected)) return kGroupMembersStaleContext;
    if (state.isMutating) return 'mutation_in_progress';
    state = state.copyWith(isMutating: true, clearError: true);
    try {
      final result = await request(expected.authorization);
      if (!mounted || !isCurrentContext(expected)) {
        return kGroupMembersStaleContext;
      }
      switch (result) {
        case ChatsApiFailure(:final message):
          state = state.copyWith(isMutating: false, clearError: true);
          return message;
        case ChatsApiOk<void>():
          state = state.copyWith(isMutating: false, clearError: true);
          _ref.invalidate(groupMembersProvider(chatId));
          await load(expectedContext: expected);
          return isCurrentContext(expected) ? null : kGroupMembersStaleContext;
      }
    } catch (_) {
      if (!mounted || !isCurrentContext(expected)) {
        return kGroupMembersStaleContext;
      }
      state = state.copyWith(isMutating: false, clearError: true);
      return 'unknown_error';
    }
  }
}
