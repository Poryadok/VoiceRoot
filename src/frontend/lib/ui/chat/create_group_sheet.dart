import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:uuid/uuid.dart';

import '../../backend/chats_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/create_group_friends_provider.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_layout.dart';
import '../api_error_messages.dart';
import '../a11y/focus_trap.dart';
import '../a11y/voice_focus_return.dart';

/// Minimum invitees besides the creator (3 people total per text-chat.md).
const int kMinGroupInvitees = 2;

/// Responsive group-name and multi-select flow → POST /api/v1/chats + members.
class CreateGroupSheet extends ConsumerStatefulWidget {
  const CreateGroupSheet({
    super.key,
    this.requiredMemberProfileId,
    this.expectedViewerProfileId,
  });

  final String? requiredMemberProfileId;
  final String? expectedViewerProfileId;

  static const Key sheetKey = Key('create_group_sheet');
  static const Key nameFieldKey = Key('create_group_name');
  static const Key searchFieldKey = Key('create_group_friend_search');
  static const Key submitKey = Key('create_group_submit');
  static const Key closeKey = Key('create_group_close');
  static const Key cancelKey = Key('create_group_cancel');

  static Key memberTileKey(String profileId) =>
      Key('create_group_member_$profileId');

  static Future<void> show(
    BuildContext context, {
    String? requiredMemberProfileId,
    String? expectedViewerProfileId,
  }) async {
    final container = ProviderScope.containerOf(context);
    final focusReturn = VoiceFocusReturn.capture();
    final content = UncontrolledProviderScope(
      container: container,
      child: CreateGroupSheet(
        requiredMemberProfileId: requiredMemberProfileId,
        expectedViewerProfileId: expectedViewerProfileId,
      ),
    );
    void dismiss(BuildContext routeContext) {
      Navigator.of(routeContext).pop();
      focusReturn.restore();
    }

    if (VoiceLayout.isNarrow(MediaQuery.sizeOf(context).width)) {
      await showModalBottomSheet<void>(
        context: context,
        isScrollControlled: true,
        useSafeArea: true,
        builder: (routeContext) => VoiceFocusTrap(
          onEscape: () => dismiss(routeContext),
          child: LayoutBuilder(
            builder: (context, constraints) =>
                SizedBox(height: constraints.maxHeight, child: content),
          ),
        ),
      );
    } else {
      final height = MediaQuery.sizeOf(context).height;
      final dialogHeight = height * .8 < 680 ? height * .8 : 680.0;
      await showDialog<void>(
        context: context,
        builder: (routeContext) => VoiceFocusTrap(
          onEscape: () => dismiss(routeContext),
          child: Dialog(
            constraints: BoxConstraints(maxWidth: 460, maxHeight: height * .9),
            child: SizedBox(height: dialogHeight, child: content),
          ),
        ),
      );
    }
    focusReturn.restore();
  }

  @override
  ConsumerState<CreateGroupSheet> createState() => _CreateGroupSheetState();
}

class _CreateGroupSheetState extends ConsumerState<CreateGroupSheet> {
  final _nameController = TextEditingController();
  final _searchController = TextEditingController();
  final _selected = <String>{};
  var _submitting = false;
  _CreateGroupCreateAttempt? _createAttempt;
  String? _inviteRetryChatId;
  List<String>? _inviteRetryProfileIds;
  _CreateGroupInviteRetryIdentity? _inviteRetryIdentity;

  @override
  void initState() {
    super.initState();
    final requiredId = widget.requiredMemberProfileId;
    if (requiredId != null && requiredId.isNotEmpty) {
      _selected.add(requiredId);
    }
  }

  @override
  void dispose() {
    _nameController.dispose();
    _searchController.dispose();
    super.dispose();
  }

  bool get _canSubmit {
    final name = _nameController.text.trim();
    return !_submitting &&
        name.isNotEmpty &&
        (_inviteRetryChatId != null || _selected.length >= kMinGroupInvitees) &&
        (widget.expectedViewerProfileId == null ||
            ref.read(authControllerProvider).activeProfileId ==
                widget.expectedViewerProfileId);
  }

  Future<void> _submit() async {
    if (!_canSubmit) return;
    setState(() => _submitting = true);
    final l10n = AppLocalizations.of(context)!;
    final session = ref.read(authControllerProvider).session;
    final authorization = ref.read(authorizationHeaderProvider);
    final authController = ref.read(authControllerProvider.notifier);
    final requestIdentity = authController.gatewayRequestIdentity;
    if (session == null || authorization == null || requestIdentity == null) {
      _finishWithError(l10n, 'not_authenticated');
      return;
    }
    final accountId = session.accountId;
    final activeProfileId = session.activeProfileId;
    final profileGeneration = requestIdentity.generation;
    final sessionInstallGeneration = authController.sessionInstallGeneration;
    final client = ref.read(voiceChatsClientProvider);

    try {
      final pendingChatId = _inviteRetryChatId;
      final pendingProfileIds = _inviteRetryProfileIds;
      if (pendingChatId != null || pendingProfileIds != null) {
        final retryIdentity = _inviteRetryIdentity;
        if (pendingChatId == null ||
            pendingProfileIds == null ||
            retryIdentity == null ||
            !retryIdentity.matches(
              accountId: accountId,
              profileId: activeProfileId,
              profileGeneration: profileGeneration,
              sessionInstallGeneration: sessionInstallGeneration,
            )) {
          setState(() {
            _submitting = false;
            _inviteRetryChatId = null;
            _inviteRetryProfileIds = null;
            _inviteRetryIdentity = null;
            _selected.clear();
          });
          _showSafeError(l10n, kChatActionStaleContext);
          return;
        }
      }
      late final String chatId;
      late final List<String> profileIds;
      if (pendingChatId == null || pendingProfileIds == null) {
        final selectedProfileIds = _selected.toList(growable: false);
        final name = _nameController.text.trim();
        var createAttempt = _createAttempt;
        if (createAttempt == null ||
            !createAttempt.matches(
              name: name,
              accountId: accountId,
              profileId: activeProfileId,
              profileGeneration: profileGeneration,
              sessionInstallGeneration: sessionInstallGeneration,
              selectedProfileIds: _selected,
            )) {
          createAttempt = _CreateGroupCreateAttempt(
            name: name,
            accountId: accountId,
            profileId: activeProfileId,
            profileGeneration: profileGeneration,
            sessionInstallGeneration: sessionInstallGeneration,
            selectedProfileIds: Set.unmodifiable(_selected),
            requestId: const Uuid().v4(),
          );
          _createAttempt = createAttempt;
        }
        final createResult = await client.createGroup(
          authorization: authorization,
          name: createAttempt.name,
          requestId: createAttempt.requestId,
        );
        if (!mounted) return;
        if (!_isCurrentSubmission(
          accountId,
          activeProfileId,
          profileGeneration,
          sessionInstallGeneration,
        )) {
          _createAttempt = null;
          _finishWithError(l10n, kChatActionStaleContext);
          return;
        }
        switch (createResult) {
          case ChatsApiFailure(:final message):
            _finishWithError(l10n, message);
            return;
          case ChatsApiOk(:final data):
            chatId = data.id;
            profileIds = List.unmodifiable(selectedProfileIds);
            setState(() {
              _createAttempt = null;
              _inviteRetryChatId = chatId;
              _inviteRetryProfileIds = profileIds;
              _inviteRetryIdentity = _CreateGroupInviteRetryIdentity(
                accountId: accountId,
                profileId: activeProfileId,
                profileGeneration: profileGeneration,
                sessionInstallGeneration: sessionInstallGeneration,
              );
            });
        }
      } else {
        chatId = pendingChatId;
        profileIds = pendingProfileIds;
      }

      final inviteAuthorization = ref.read(authorizationHeaderProvider);
      if (inviteAuthorization == null) {
        _finishWithError(l10n, 'not_authenticated');
        return;
      }
      final inviteResult = await client.addGroupMembers(
        authorization: inviteAuthorization,
        chatId: chatId,
        profileIds: profileIds,
      );
      if (!mounted) return;
      if (!_isCurrentSubmission(
        accountId,
        activeProfileId,
        profileGeneration,
        sessionInstallGeneration,
      )) {
        setState(() {
          _submitting = false;
          _inviteRetryChatId = null;
          _inviteRetryProfileIds = null;
          _inviteRetryIdentity = null;
        });
        _showSafeError(l10n, kChatActionStaleContext);
        return;
      }
      switch (inviteResult) {
        case ChatsApiFailure(:final message):
          _finishWithError(l10n, message);
          return;
        case ChatsApiOk():
          ref.read(chatActionsProvider).selectChat(chatId);
          _inviteRetryIdentity = null;
          Navigator.of(context).pop();
      }
    } catch (_) {
      if (!mounted) return;
      _finishWithError(l10n, 'group_action_failed');
    }
  }

  bool _isCurrentSubmission(
    String accountId,
    String activeProfileId,
    int profileGeneration,
    int sessionInstallGeneration,
  ) {
    final authController = ref.read(authControllerProvider.notifier);
    final current = ref.read(authControllerProvider).session;
    final identity = authController.gatewayRequestIdentity;
    return current?.accountId == accountId &&
        current?.activeProfileId == activeProfileId &&
        identity?.accountId == accountId &&
        identity?.profileId == activeProfileId &&
        identity?.generation == profileGeneration &&
        authController.sessionInstallGeneration == sessionInstallGeneration;
  }

  void _finishWithError(AppLocalizations l10n, String error) {
    if (!mounted) return;
    setState(() => _submitting = false);
    _showSafeError(l10n, error);
  }

  void _showSafeError(AppLocalizations l10n, String error) {
    final errorMessage = error == 'not_authenticated'
        ? l10n.chatCreateGroupError(error)
        : commonActionErrorMessage(l10n);
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(errorMessage)));
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final friendsAsync = ref.watch(createGroupFriendsProvider);
    final activeProfileId = ref.watch(authControllerProvider).activeProfileId;
    final narrow = VoiceLayout.isNarrow(MediaQuery.sizeOf(context).width);
    final viewerMatches =
        widget.expectedViewerProfileId == null ||
        activeProfileId == widget.expectedViewerProfileId;
    final theme = Theme.of(context);
    final submitButton = FilledButton(
      key: CreateGroupSheet.submitKey,
      onPressed: _canSubmit && viewerMatches ? _submit : null,
      child: _submitting
          ? const SizedBox(
              width: 20,
              height: 20,
              child: CircularProgressIndicator(strokeWidth: 2),
            )
          : Text(
              _inviteRetryChatId == null
                  ? l10n.chatCreateGroupSubmit
                  : l10n.commonRetry,
            ),
    );

    return SafeArea(
      key: CreateGroupSheet.sheetKey,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    l10n.chatCreateGroupTitle,
                    style: theme.textTheme.titleLarge,
                  ),
                ),
                IconButton(
                  key: CreateGroupSheet.closeKey,
                  tooltip: l10n.commonCancel,
                  onPressed: () => Navigator.of(context).pop(),
                  icon: const Icon(Icons.close),
                ),
              ],
            ),
            const SizedBox(height: 12),
            TextField(
              key: CreateGroupSheet.nameFieldKey,
              controller: _nameController,
              decoration: InputDecoration(
                labelText: l10n.chatCreateGroupNameLabel,
                hintText: l10n.chatCreateGroupNameHint,
              ),
              textCapitalization: TextCapitalization.sentences,
              enabled: !_submitting && _inviteRetryChatId == null,
              onChanged: (_) => setState(() => _createAttempt = null),
            ),
            const SizedBox(height: 16),
            Text(
              l10n.chatCreateGroupMembers,
              style: theme.textTheme.titleSmall,
            ),
            const SizedBox(height: 4),
            Text(
              l10n.chatCreateGroupMembersHint,
              style: theme.textTheme.bodySmall,
            ),
            if (widget.requiredMemberProfileId case final requiredId?)
              _requiredMemberTile(requiredId),
            const SizedBox(height: 8),
            TextField(
              key: CreateGroupSheet.searchFieldKey,
              controller: _searchController,
              decoration: InputDecoration(
                hintText: l10n.socialSearchHint,
                prefixIcon: const Icon(Icons.search),
              ),
              textInputAction: TextInputAction.search,
              enabled: !_submitting && _inviteRetryChatId == null,
              onChanged: (_) => setState(() {}),
            ),
            const SizedBox(height: 8),
            Expanded(
              child: friendsAsync.when(
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (e, st) => _createGroupStatePanel(
                  context,
                  title: socialListErrorMessage(l10n, e),
                  icon: Icons.cloud_off_outlined,
                  actionLabel: l10n.commonRetry,
                  onAction: () => ref.invalidate(createGroupFriendsProvider),
                ),
                data: (ids) {
                  final availableIds = widget.requiredMemberProfileId == null
                      ? ids
                      : ids
                            .where((id) => id != widget.requiredMemberProfileId)
                            .toList(growable: false);
                  if (availableIds.isEmpty) {
                    return _createGroupStatePanel(
                      context,
                      title: l10n.socialFriendsEmpty,
                      message: l10n.chatCreateGroupFriendsEmptyHint,
                      icon: Icons.people_outline,
                    );
                  }
                  final query = _searchController.text.trim().toLowerCase();
                  final searchQuery = query.startsWith('@')
                      ? query.substring(1)
                      : query;
                  var visibleIds = availableIds;
                  if (searchQuery.isNotEmpty) {
                    final profiles = {
                      for (final profileId in availableIds)
                        profileId: ref.watch(profileProvider(profileId)),
                    };
                    final failedProfileIds = profiles.entries
                        .where((entry) => entry.value.hasError)
                        .map((entry) => entry.key)
                        .toList(growable: false);
                    if (failedProfileIds.isNotEmpty) {
                      final error = profiles[failedProfileIds.first]!.error!;
                      return _createGroupStatePanel(
                        context,
                        title: socialListErrorMessage(l10n, error),
                        icon: Icons.cloud_off_outlined,
                        actionLabel: l10n.commonRetry,
                        onAction: () {
                          for (final profileId in failedProfileIds) {
                            ref.invalidate(profileProvider(profileId));
                          }
                        },
                      );
                    }
                    if (profiles.values.any(
                      (profile) => profile.isLoading && !profile.hasValue,
                    )) {
                      return const Center(child: CircularProgressIndicator());
                    }
                    visibleIds = availableIds
                        .where((profileId) {
                          final profile = profiles[profileId]?.valueOrNull;
                          final displayName = profile?.displayName
                              .toLowerCase();
                          final handle = profile?.handle
                              .replaceFirst(RegExp(r'^@'), '')
                              .toLowerCase();
                          return (displayName?.contains(searchQuery) ??
                                  false) ||
                              (handle?.contains(searchQuery) ?? false);
                        })
                        .toList(growable: false);
                    if (visibleIds.isEmpty) {
                      return _createGroupStatePanel(
                        context,
                        title: l10n.socialSearchEmpty,
                        message: l10n.socialSearchEmptyHint,
                        icon: Icons.search_off,
                      );
                    }
                  }
                  return ListView.builder(
                    itemCount: visibleIds.length,
                    itemBuilder: (context, index) {
                      final profileId = visibleIds[index];
                      final profileAsync = ref.watch(
                        profileProvider(profileId),
                      );
                      final profile = profileAsync.valueOrNull;
                      final label =
                          profile?.displayName ?? profile?.handle ?? profileId;
                      final selected = _selected.contains(profileId);
                      return CheckboxListTile(
                        key: CreateGroupSheet.memberTileKey(profileId),
                        value: selected,
                        onChanged: _submitting || _inviteRetryChatId != null
                            ? null
                            : (next) {
                                setState(() {
                                  _createAttempt = null;
                                  if (next ?? false) {
                                    _selected.add(profileId);
                                  } else {
                                    _selected.remove(profileId);
                                  }
                                });
                              },
                        title: Text(label),
                        subtitle: profile != null ? Text(profile.handle) : null,
                        secondary: CircleAvatar(
                          child: Text(
                            label.isNotEmpty ? label[0].toUpperCase() : '?',
                          ),
                        ),
                        controlAffinity: ListTileControlAffinity.leading,
                      );
                    },
                  );
                },
              ),
            ),
            if (_selected.length < kMinGroupInvitees)
              Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: Text(
                  l10n.chatCreateGroupMinMembers,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.error,
                  ),
                ),
              ),
            if (narrow)
              SizedBox(width: double.infinity, child: submitButton)
            else
              Row(
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  TextButton(
                    key: CreateGroupSheet.cancelKey,
                    onPressed: () => Navigator.of(context).pop(),
                    child: Text(l10n.commonCancel),
                  ),
                  const SizedBox(width: 8),
                  submitButton,
                ],
              ),
          ],
        ),
      ),
    );
  }
}

class _CreateGroupCreateAttempt {
  const _CreateGroupCreateAttempt({
    required this.name,
    required this.accountId,
    required this.profileId,
    required this.profileGeneration,
    required this.sessionInstallGeneration,
    required this.selectedProfileIds,
    required this.requestId,
  });

  final String name;
  final String accountId;
  final String profileId;
  final int profileGeneration;
  final int sessionInstallGeneration;
  final Set<String> selectedProfileIds;
  final String requestId;

  bool matches({
    required String name,
    required String accountId,
    required String profileId,
    required int profileGeneration,
    required int sessionInstallGeneration,
    required Set<String> selectedProfileIds,
  }) =>
      this.name == name &&
      this.accountId == accountId &&
      this.profileId == profileId &&
      this.profileGeneration == profileGeneration &&
      this.sessionInstallGeneration == sessionInstallGeneration &&
      this.selectedProfileIds.length == selectedProfileIds.length &&
      this.selectedProfileIds.containsAll(selectedProfileIds);
}

class _CreateGroupInviteRetryIdentity {
  const _CreateGroupInviteRetryIdentity({
    required this.accountId,
    required this.profileId,
    required this.profileGeneration,
    required this.sessionInstallGeneration,
  });

  final String accountId;
  final String profileId;
  final int profileGeneration;
  final int sessionInstallGeneration;

  bool matches({
    required String accountId,
    required String profileId,
    required int profileGeneration,
    required int sessionInstallGeneration,
  }) =>
      this.accountId == accountId &&
      this.profileId == profileId &&
      this.profileGeneration == profileGeneration &&
      this.sessionInstallGeneration == sessionInstallGeneration;
}

Widget _createGroupStatePanel(
  BuildContext context, {
  required String title,
  IconData? icon,
  String? message,
  String? actionLabel,
  VoidCallback? onAction,
}) {
  final theme = Theme.of(context);
  return Semantics(
    container: true,
    label: title,
    child: Center(
      child: Padding(
        padding: const EdgeInsets.all(8),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (icon != null) ...[
              Icon(icon, size: 20, color: theme.colorScheme.onSurfaceVariant),
              const SizedBox(height: 4),
            ],
            ExcludeSemantics(
              child: Text(
                title,
                textAlign: TextAlign.center,
                style: theme.textTheme.titleSmall,
              ),
            ),
            if (message != null && message.isNotEmpty) ...[
              const SizedBox(height: 4),
              Text(
                message,
                textAlign: TextAlign.center,
                style: theme.textTheme.bodySmall,
              ),
            ],
            if (actionLabel != null && onAction != null)
              TextButton(onPressed: onAction, child: Text(actionLabel)),
          ],
        ),
      ),
    ),
  );
}

extension on _CreateGroupSheetState {
  Widget _requiredMemberTile(String profileId) {
    final profile = ref.watch(profileProvider(profileId)).valueOrNull;
    final label = profile?.displayName ?? profile?.handle ?? profileId;
    return CheckboxListTile(
      key: CreateGroupSheet.memberTileKey(profileId),
      value: true,
      onChanged: null,
      title: Text(label),
      subtitle: profile == null ? null : Text(profile.handle),
      secondary: const Icon(Icons.lock_outline),
      controlAffinity: ListTileControlAffinity.leading,
    );
  }
}
