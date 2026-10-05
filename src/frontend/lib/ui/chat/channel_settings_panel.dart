import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/chats_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/channel_settings_provider.dart';
import '../../state/chat_providers.dart';
import '../../theme/voice_colors.dart';
import '../api_error_messages.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';

/// Standalone channel policy editor for owners and admins.
class ChannelSettingsPanel extends ConsumerStatefulWidget {
  const ChannelSettingsPanel({super.key, required this.chatId, this.onClose});

  static const Key panelKey = Key('channel_settings_panel');
  static const Key threadsToggleKey = Key('channel_settings_threads');
  static const Key memberPostsToggleKey = Key('channel_settings_member_posts');
  static const Key saveKey = Key('channel_settings_save');
  static const Key cancelKey = Key('channel_settings_cancel');
  static const Key closeKey = Key('channel_settings_close');

  final String chatId;
  final VoidCallback? onClose;

  static Future<void> show(BuildContext context, {required String chatId}) {
    final container = ProviderScope.containerOf(context);
    return showVoiceBottomSheet<void>(
      context: context,
      scrollable: false,
      child: UncontrolledProviderScope(
        container: container,
        child: ChannelSettingsPanel(chatId: chatId),
      ),
    );
  }

  @override
  ConsumerState<ChannelSettingsPanel> createState() =>
      _ChannelSettingsPanelState();
}

typedef _SettingsContext = (String, String, String, String);

class _ChannelSettingsPanelState extends ConsumerState<ChannelSettingsPanel> {
  _SettingsContext? _activeContext;
  VoiceChat? _serverChat;
  bool? _threadsDraft;
  bool? _memberPostsDraft;
  bool _loading = true;
  bool _saving = false;
  bool _authorized = false;
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
  void didUpdateWidget(covariant ChannelSettingsPanel oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.chatId != widget.chatId) _reloadSettings(force: true);
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(authControllerProvider, (_, _) => _reloadSettings());
    ref.listen(authorizationHeaderProvider, (_, _) => _reloadSettings());
    final l10n = AppLocalizations.of(context)!;
    final contextKey = _captureContext();
    if (contextKey != _activeContext && !_loading) {
      _reloadSettings();
    }

    if (_loading) {
      return SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            key: ChannelSettingsPanel.panelKey,
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(l10n.channelSettingsTitle),
              const VoiceListSkeleton(rowCount: 2),
            ],
          ),
        ),
      );
    }
    if (_loadErrorStatusCode != null) {
      return SafeArea(
        child: VoiceStatePanel(
          title: l10n.channelSettingsTitle,
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
    if (!_authorized || _serverChat == null || contextKey != _activeContext) {
      return const SizedBox.shrink();
    }

    final voice = VoiceColors.of(context);
    final chat = _serverChat!;
    final threadsEnabled = _threadsDraft ?? chat.threadsEnabled;
    final memberPostsEnabled = _memberPostsDraft ?? chat.allowUserMainFeed;
    final hasChanges =
        threadsEnabled != chat.threadsEnabled ||
        memberPostsEnabled != chat.allowUserMainFeed;

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
        child: Column(
          key: ChannelSettingsPanel.panelKey,
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    l10n.channelSettingsTitle,
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                ),
                IconButton(
                  key: ChannelSettingsPanel.closeKey,
                  tooltip: l10n.chatInfoTitle,
                  onPressed: _saving ? null : _close,
                  icon: const Icon(Icons.close),
                ),
              ],
            ),
            Divider(height: 1, color: voice.borderDefault),
            SwitchListTile(
              key: ChannelSettingsPanel.threadsToggleKey,
              title: Text(l10n.channelSettingsThreads),
              value: threadsEnabled,
              onChanged: _saving
                  ? null
                  : (value) => setState(() => _threadsDraft = value),
            ),
            SwitchListTile(
              key: ChannelSettingsPanel.memberPostsToggleKey,
              title: Text(l10n.channelSettingsMemberPosts),
              value: memberPostsEnabled,
              onChanged: _saving
                  ? null
                  : (value) => setState(() => _memberPostsDraft = value),
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
                  key: ChannelSettingsPanel.cancelKey,
                  onPressed: _saving ? null : _close,
                  child: Text(l10n.commonCancel),
                ),
                const SizedBox(width: 8),
                FilledButton(
                  key: ChannelSettingsPanel.saveKey,
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

  Future<void> _reloadSettings({bool force = false}) async {
    final contextKey = _captureContext();
    if (!force && contextKey == _activeContext && (_loading || _authorized)) {
      return;
    }
    final generation = ++_generation;
    _activeContext = contextKey;
    if (!mounted) return;
    setState(() {
      _serverChat = null;
      _threadsDraft = null;
      _memberPostsDraft = null;
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
    if (chat == null || !chat.isChannel || chat.isSpaceChannel) {
      setState(() {
        _loading = false;
        _authorized = false;
      });
      return;
    }

    ref.invalidate(channelSettingsAuthorityProvider(widget.chatId));
    var canManage = false;
    try {
      canManage = await ref.read(
        channelSettingsAuthorityProvider(widget.chatId).future,
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
        !currentChat.isChannel ||
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
    final threads = _threadsDraft;
    final memberPosts = _memberPostsDraft;
    final threadsToSave = threads != null && threads != chat.threadsEnabled
        ? threads
        : null;
    final memberPostsToSave =
        memberPosts != null && memberPosts != chat.allowUserMainFeed
        ? memberPosts
        : null;
    if (threadsToSave == null && memberPostsToSave == null) return;
    if (!_isCurrent(contextKey, _generation)) return;
    final generation = _generation;
    setState(() {
      _saving = true;
      _saveErrorStatusCode = null;
    });

    final result = await ref
        .read(voiceChatsClientProvider)
        .updateGroup(
          authorization: contextKey.$4,
          chatId: contextKey.$1,
          threadsEnabled: threadsToSave,
          allowUserMainFeed: memberPostsToSave,
        );
    if (!_isCurrent(contextKey, generation)) return;
    switch (result) {
      case ChatsApiFailure(:final statusCode):
        setState(() {
          _saving = false;
          _saveErrorStatusCode = statusCode;
        });
      case ChatsApiOk():
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
        ref.invalidate(channelSettingsAuthorityProvider(widget.chatId));
        var canManage = false;
        try {
          canManage = await ref.read(
            channelSettingsAuthorityProvider(widget.chatId).future,
          );
        } catch (_) {
          canManage = false;
        }
        if (!_isCurrent(contextKey, generation)) return;
        final refreshedChat = ref
            .read(chatListControllerProvider)
            .items
            .where((item) => item.chat.id == widget.chatId)
            .map((item) => item.chat)
            .firstOrNull;
        if (!canManage ||
            refreshedChat == null ||
            !refreshedChat.isChannel ||
            refreshedChat.isSpaceChannel) {
          setState(() {
            _saving = false;
            _authorized = false;
            _serverChat = null;
            _threadsDraft = null;
            _memberPostsDraft = null;
          });
          return;
        }
        setState(() {
          _saving = false;
          _serverChat = refreshedChat;
          _threadsDraft = null;
          _memberPostsDraft = null;
          _authorized = true;
          _saveErrorStatusCode = null;
        });
    }
  }
}
