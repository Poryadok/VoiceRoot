import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/chats_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/group_members_management_providers.dart';
import '../../state/group_settings_provider.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';
import '../api_error_messages.dart';
import '../core/voice_avatar.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';

/// Combined v3 destination "Panel / Chat / GroupSettings" (docs/design/screen-controls.md §10.1c entry #21): the standalone group settings editor reached from Chat Info by GRP owners and admins. It edits the group identity (name), the threads and member-posts toggles, the member-additions permission, and the administrator roster roles. Membership is resolved across every member page by the scoped group settings authority provider; space chats keep their settings in the Space/Role service and are intentionally excluded.
class GroupSettingsPanel extends ConsumerStatefulWidget {
  const GroupSettingsPanel({super.key, required this.chatId, this.onClose});

  static const Key panelKey = Key('group_settings_panel');
  static const Key nameEditKey = Key('group_settings_name_edit');
  static const Key nameFieldKey = Key('group_settings_name_field');
  static const Key threadsToggleKey = Key('group_settings_threads');
  static const Key memberPostsToggleKey = Key('group_settings_member_posts');
  static const Key additionsToggleKey = Key('group_settings_additions');
  static const Key saveKey = Key('group_settings_save');
  static const Key cancelKey = Key('group_settings_cancel');
  static const Key closeKey = Key('group_settings_close');
  static const String adminTogglePrefix = 'group_settings_admin_toggle_';

  final String chatId;
  final VoidCallback? onClose;

  static Future<void> show(
    BuildContext context, {
    required String chatId,
  }) {
    final container = ProviderScope.containerOf(context);
    return showVoiceBottomSheet<void>(
      context: context,
      scrollable: false,
      child: UncontrolledProviderScope(
        container: container,
        child: GroupSettingsPanel(chatId: chatId),
      ),
    );
  }

  @override
  ConsumerState<GroupSettingsPanel> createState() =>
      _GroupSettingsPanelState();
}

typedef _SettingsContext = (String, String, String, String);

class _GroupSettingsPanelState extends ConsumerState<GroupSettingsPanel> {
  _SettingsContext? _activeContext;
  VoiceChat? _serverChat;
  String? _nameDraft;
  bool? _threadsDraft;
  bool? _memberPostsDraft;
  bool? _additionsDraft;
  final Map<String, bool> _roleDrafts = <String, bool>{};
  bool _loading = true;
  bool _saving = false;
  bool _authorized = false;
  bool _editingName = false;
  final TextEditingController _nameController = TextEditingController();
  final FocusNode _nameFocusNode = FocusNode();
  int? _loadErrorStatusCode;
  int? _saveErrorStatusCode;
  int _generation = 0;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _reloadSettings(force: true);
    });
  }

  @override
  void didUpdateWidget(covariant GroupSettingsPanel oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.chatId != widget.chatId) _reloadSettings(force: true);
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(authControllerProvider, (_, _) => _reloadSettings());
    ref.listen(authorizationHeaderProvider, (_, _) => _reloadSettings());
    final l10n = AppLocalizations.of(context)!;
    final roster = ref.watch(groupMembersManagementProvider(widget.chatId));
    final contextKey = _captureContext();
    if (contextKey != _activeContext && !_loading) {
      _reloadSettings();
    }

    if (_loading || roster.isLoading) {
      return _stateLayout(
        l10n,
        const Padding(
          padding: EdgeInsets.all(16),
          child: VoiceListSkeleton(rowCount: 2),
        ),
      );
    }
    if (_loadErrorStatusCode != null) {
      return _stateLayout(
        l10n,
        VoiceStatePanel(
          title: l10n.groupSettingsTitle,
          message: commonActionErrorMessage(
            l10n,
            statusCode: _loadErrorStatusCode,
          ),
          icon: Icons.cloud_off_outlined,
          actionLabel: l10n.commonRetry,
          onAction: _saving ? null : () => _reloadSettings(force: true),
        ),
      );
    }
    if (roster.error != null) {
      return _stateLayout(
        l10n,
        VoiceStatePanel(
          title: l10n.chatGroupMembersLoadError,
          message: groupMembersErrorMessage(l10n, roster.error!),
          icon: Icons.cloud_off_outlined,
          actionLabel: l10n.commonRetry,
          onAction: _saving
              ? null
              : () => ref
                    .read(groupMembersManagementProvider(widget.chatId).notifier)
                    .load(),
        ),
      );
    }
    if (!_authorized ||
        _serverChat == null ||
        contextKey == null ||
        contextKey != _activeContext) {
      return _stateLayout(l10n, const SizedBox.shrink());
    }

    final voice = VoiceColors.of(context);
    final chat = _serverChat!;
    final activeProfileId = contextKey.$3;
    final myRole = roster.members
        .where((member) => member.profileId == activeProfileId)
        .map((member) => member.role)
        .firstOrNull;
    final isOwnerRole = myRole == kChatRoleOwner;
    final isAdminRole = myRole == kChatRoleAdmin;

    final typedName =
        _editingName && _nameController.text.isNotEmpty
            ? _nameController.text
            : null;
    final nameEnabled =
        typedName ?? _nameDraft ?? chat.name;
    final threadsEnabled = _threadsDraft ?? chat.threadsEnabled;
    final memberPostsEnabled = _memberPostsDraft ?? chat.allowUserMainFeed;
    final permissions = roster.permissions;
    final additionsEnabled = _additionsDraft ?? permissions?.memberAdditionsEnabled;
    final nameChanged = nameEnabled != null && nameEnabled != chat.name;
    final threadsChanged = threadsEnabled != chat.threadsEnabled;
    final memberPostsChanged =
        memberPostsEnabled != chat.allowUserMainFeed;
    final additionsChanged =
        additionsEnabled != null &&
        permissions != null &&
        additionsEnabled != permissions.memberAdditionsEnabled;
    final roleChanges = _roleDrafts.isNotEmpty;
    final hasChanges =
        nameChanged ||
        threadsChanged ||
        memberPostsChanged ||
        additionsChanged ||
        roleChanges;

    final adminRows = roster.members
        .where((member) =>
            member.profileId != activeProfileId && member.role != kChatRoleOwner)
        .toList();

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
        child: Column(
          key: GroupSettingsPanel.panelKey,
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    l10n.groupSettingsTitle,
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                ),
                IconButton(
                  key: GroupSettingsPanel.closeKey,
                  tooltip: l10n.chatInfoTitle,
                  onPressed: _saving ? null : _close,
                  icon: const Icon(Icons.close),
                ),
              ],
            ),
            Divider(height: 1, color: voice.borderDefault),
            Row(
              children: [
                VoiceAvatar(
                  imageUrl: chat.avatarUrl,
                  label: chat.name ?? '',
                  radius: 20,
                ),
                Expanded(
                  child: Column(
                    key: const ValueKey('group_settings_identity'),
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        chat.name ?? '',
                        style: Theme.of(context).textTheme.titleMedium,
                      ),
                      Text(
                        l10n.groupSettingsIdentityHint,
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                      if (_editingName)
                        TextField(
                          key: GroupSettingsPanel.nameFieldKey,
                          autofocus: true,
                          controller: _nameController,
                          focusNode: _nameFocusNode,
                          decoration: InputDecoration(
                            labelText: l10n.groupSettingsNameField,
                          ),
                        ),
                    ],
                  ),
                ),
                IconButton(
                  key: GroupSettingsPanel.nameEditKey,
                  tooltip: l10n.groupSettingsIdentityHint,
                  onPressed: _saving || _serverChat == null ? null : _toggleNameEdit,
                  icon: const Icon(Icons.edit),
                ),
              ],
            ),
            Divider(height: 1, color: voice.borderDefault),
            Text(
              l10n.groupSettingsSectionLabel,
              style: Theme.of(context).textTheme.titleSmall,
            ),
            SwitchListTile(
              key: GroupSettingsPanel.threadsToggleKey,
              title: Text(l10n.channelSettingsThreads),
              value: threadsEnabled,
              onChanged: _saving
                  ? null
                  : (value) => setState(() => _threadsDraft = value),
            ),
            SwitchListTile(
              key: GroupSettingsPanel.memberPostsToggleKey,
              title: Text(l10n.channelSettingsMemberPosts),
              value: memberPostsEnabled,
              onChanged: _saving
                  ? null
                  : (value) => setState(() => _memberPostsDraft = value),
            ),
            if (permissions != null && permissions.canManagePermissions)
              SwitchListTile(
                key: GroupSettingsPanel.additionsToggleKey,
                title: Text(l10n.chatGroupMemberAdditionsPermission),
                subtitle: Text(
                  additionsEnabled == true
                      ? l10n.groupSettingsAdditionsEnabled
                      : l10n.groupSettingsAdditionsAdminsOnly,
                ),
                value: additionsEnabled == true,
                onChanged: _saving
                    ? null
                    : (value) => setState(() => _additionsDraft = value),
              ),
            Divider(height: 1, color: voice.borderDefault),
            Text(
              l10n.groupSettingsAdminsSection,
              style: Theme.of(context).textTheme.titleSmall,
            ),
            for (final row in adminRows)
              SwitchListTile(
                key: ValueKey(
                  GroupSettingsPanel.adminTogglePrefix + row.profileId,
                ),
                title: Text(
                  ref.watch(profileProvider(row.profileId)).valueOrNull
                          ?.displayName ??
                      ref.watch(profileProvider(row.profileId)).valueOrNull
                          ?.handle ??
                      row.profileId,
                ),
                subtitle: row.role == kChatRoleAdmin
                    ? Text(l10n.chatGroupRoleAdmin)
                    : null,
                value: _roleDrafts[row.profileId] ?? row.role == kChatRoleAdmin,
                onChanged: _saving
                    ? null
                    : (value) {
                        final canWrite = value
                            ? isOwnerRole || isAdminRole
                            : isOwnerRole;
                        if (!canWrite) return;
                        setState(() {
                          if (value == (row.role == kChatRoleAdmin)) {
                            _roleDrafts.remove(row.profileId);
                          } else {
                            _roleDrafts[row.profileId] = value;
                          }
                        });
                      },
              ),
            if (_saveErrorStatusCode != null)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 8),
                child: Text(
                  commonActionErrorMessage(
                    l10n,
                    statusCode: _saveErrorStatusCode,
                  ),
                ),
              ),
            Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                TextButton(
                  key: GroupSettingsPanel.cancelKey,
                  onPressed: _saving ? null : _close,
                  child: Text(l10n.commonCancel),
                ),
                const SizedBox(width: 8),
                FilledButton(
                  key: GroupSettingsPanel.saveKey,
                  onPressed: !_saving && hasChanges ? _save : null,
                  child: Text(l10n.commonSave),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _stateLayout(AppLocalizations l10n, Widget body) {
    return SafeArea(
      child: Column(
        key: GroupSettingsPanel.panelKey,
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 8, 8, 8),
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    l10n.groupSettingsTitle,
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                ),
                IconButton(
                  key: GroupSettingsPanel.closeKey,
                  tooltip: l10n.chatInfoTitle,
                  onPressed: _saving ? null : _close,
                  icon: const Icon(Icons.close),
                ),
              ],
            ),
          ),
          Divider(height: 1, color: VoiceColors.of(context).borderDefault),
          body,
        ],
      ),
    );
  }

  _SettingsContext? _captureContext() {
    final authorization = ref.read(authorizationHeaderProvider);
    final authState = ref.read(authControllerProvider);
    final profileId = authState.activeProfileId;
    final accountId = authState.session?.accountId;
    if (authorization == null ||
        profileId == null ||
        profileId.isEmpty ||
        accountId == null ||
        accountId.isEmpty) {
      return null;
    }
    return (widget.chatId, accountId, profileId, authorization);
  }

  bool _isCurrent(_SettingsContext contextKey, int generation) {
    return mounted &&
        generation == _generation &&
        widget.chatId == contextKey.$1 &&
        ref.read(authControllerProvider).session?.accountId == contextKey.$2 &&
        ref.read(authControllerProvider).activeProfileId == contextKey.$3 &&
        ref.read(authorizationHeaderProvider) == contextKey.$4;
  }

  void _close() {
    final onClose = widget.onClose;
    if (onClose != null) {
      onClose();
    } else {
      Navigator.of(context).pop();
    }
  }

  void _toggleNameEdit() {
    if (_editingName) {
      setState(() {
        _nameDraft = _nameController.text.isEmpty
            ? null
            : _nameController.text;
        _editingName = false;
      });
      return;
    }
    final chat = _serverChat;
    if (chat == null || _saving) return;
    setState(() {
      _editingName = true;
      _nameController.clear();
    });
  }

  Future<void> _reloadSettings({bool force = false}) async {
    final contextKey = _captureContext();
    if (!force &&
        contextKey == _activeContext &&
        (_loading || _authorized)) {
      return;
    }
    final generation = ++_generation;
    _activeContext = contextKey;
    if (!mounted) return;
    setState(() {
      _serverChat = null;
      _nameDraft = null;
      _threadsDraft = null;
      _memberPostsDraft = null;
      _additionsDraft = null;
      _roleDrafts.clear();
      _editingName = false;
      _loading = contextKey != null;
      _saving = false;
      _authorized = false;
      _loadErrorStatusCode = null;
      _saveErrorStatusCode = null;
    });
    if (contextKey == null) return;

    final reloaded = await ref
        .read(chatListControllerProvider.notifier)
        .reloadInitial();
    if (!_isCurrent(contextKey, generation)) return;
    if (!reloaded) {
      final status = ref.read(chatListControllerProvider).errorStatusCode;
      setState(() {
        _loading = false;
        _loadErrorStatusCode = status ?? 503;
      });
      return;
    }

    final chat = ref
        .read(chatListControllerProvider)
        .items
        .where((item) => item.chat.id == widget.chatId)
        .map((item) => item.chat)
        .firstOrNull;
    if (chat == null || !chat.isGroup || chat.isSpaceChannel) {
      setState(() {
        _loading = false;
        _authorized = false;
      });
      return;
    }

    ref.invalidate(groupSettingsAuthorityProvider(widget.chatId));
    var canManage = false;
    try {
      canManage = await ref.read(
        groupSettingsAuthorityProvider(widget.chatId).future,
      );
    } catch (_) {
      canManage = false;
    }
    if (!_isCurrent(contextKey, generation)) return;
    if (!canManage) {
      setState(() {
        _loading = false;
        _authorized = false;
      });
      return;
    }
    final currentChat = ref
        .read(chatListControllerProvider)
        .items
        .where((item) => item.chat.id == widget.chatId)
        .map((item) => item.chat)
        .firstOrNull;
    if (currentChat == null ||
        !currentChat.isGroup ||
        currentChat.isSpaceChannel) {
      setState(() {
        _loading = false;
        _authorized = false;
      });
      return;
    }
    setState(() {
      _serverChat = currentChat;
      _loading = false;
      _authorized = true;
    });
  }

  Future<void> _save() async {
    final contextKey = _captureContext();
    final chat = _serverChat;
    if (_saving || !_authorized || chat == null || contextKey == null) return;
    final name = (() {
      final typedName =
          _editingName && _nameController.text.isNotEmpty
              ? _nameController.text
              : null;
      final candidate = typedName ?? _nameDraft;
      return candidate != null && candidate != chat.name
          ? candidate
          : null;
    })();
    final threads = _threadsDraft;
    final threadsChanged =
        threads != null && threads != chat.threadsEnabled;
    final memberPosts = _memberPostsDraft;
    final memberPostsChanged =
        memberPosts != null && memberPosts != chat.allowUserMainFeed;
    final roster = ref.read(groupMembersManagementProvider(widget.chatId));
    final permissions = roster.permissions;
    final additions = _additionsDraft;
    final additionsChanged =
        additions != null &&
        permissions != null &&
        additions != permissions.memberAdditionsEnabled;
    final roleDrafts = Map<String, bool>.of(_roleDrafts);
    if (name == null &&
        !threadsChanged &&
        !memberPostsChanged &&
        !additionsChanged &&
        roleDrafts.isEmpty) {
      return;
    }
    if (!_isCurrent(contextKey, _generation)) return;
    final generation = _generation;
    setState(() {
      _saving = true;
      _saveErrorStatusCode = null;
    });

    final authorization = contextKey.$4;
    final client = ref.read(voiceChatsClientProvider);
    if (name != null || threadsChanged || memberPostsChanged) {
      final result = await client.updateGroup(
        authorization: authorization,
        chatId: widget.chatId,
        name: name,
        threadsEnabled: threadsChanged ? threads : null,
        allowUserMainFeed: memberPostsChanged ? memberPosts : null,
      );
      if (!_isCurrent(contextKey, generation)) return;
      switch (result) {
        case ChatsApiFailure(:final statusCode):
          setState(() {
            _saving = false;
            _saveErrorStatusCode = statusCode;
          });
          return;
        case ChatsApiOk():
          break;
      }
    }
    if (additionsChanged) {
      final result = await client.setMemberAdditionsEnabled(
        authorization: authorization,
        chatId: widget.chatId,
        enabled: additions,
      );
      if (!_isCurrent(contextKey, generation)) return;
      switch (result) {
        case ChatsApiFailure(:final statusCode):
          setState(() {
            _saving = false;
            _saveErrorStatusCode = statusCode;
          });
          return;
        case ChatsApiOk():
          break;
      }
    }
    for (final entry in roleDrafts.entries) {
      final result = await client.setGroupMemberRole(
        authorization: authorization,
        chatId: widget.chatId,
        profileId: entry.key,
        role: entry.value ? kChatRoleAdmin : kChatRoleMember,
      );
      if (!_isCurrent(contextKey, generation)) return;
      switch (result) {
        case ChatsApiFailure(:final statusCode):
          setState(() {
            _saving = false;
            _saveErrorStatusCode = statusCode;
          });
          return;
        case ChatsApiOk():
          break;
      }
    }

    final reloaded = await ref
        .read(chatListControllerProvider.notifier)
        .reloadInitial();
    if (!_isCurrent(contextKey, generation)) return;
    if (!reloaded) {
      final status = ref.read(chatListControllerProvider).errorStatusCode;
      setState(() {
        _saving = false;
        _saveErrorStatusCode = status ?? 503;
      });
      return;
    }
    ref.invalidate(groupSettingsAuthorityProvider(widget.chatId));
    var canManage = false;
    try {
      canManage = await ref.read(
        groupSettingsAuthorityProvider(widget.chatId).future,
      );
    } catch (_) {
      canManage = false;
    }
    if (!_isCurrent(contextKey, generation)) return;
    await ref
        .read(groupMembersManagementProvider(widget.chatId).notifier)
        .load();
    if (!_isCurrent(contextKey, generation)) return;
    final refreshedChat = ref
        .read(chatListControllerProvider)
        .items
        .where((item) => item.chat.id == widget.chatId)
        .map((item) => item.chat)
        .firstOrNull;
    if (!canManage ||
        refreshedChat == null ||
        !refreshedChat.isGroup ||
        refreshedChat.isSpaceChannel) {
      setState(() {
        _saving = false;
        _authorized = false;
        _serverChat = null;
        _nameDraft = null;
        _threadsDraft = null;
        _memberPostsDraft = null;
        _additionsDraft = null;
        _roleDrafts.clear();
        _editingName = false;
      });
      return;
    }
    setState(() {
      _saving = false;
      _serverChat = refreshedChat;
      _nameDraft = null;
      _threadsDraft = null;
      _memberPostsDraft = null;
      _additionsDraft = null;
      _roleDrafts.clear();
      _editingName = false;
      _authorized = true;
      _saveErrorStatusCode = null;
    });
  }
}
