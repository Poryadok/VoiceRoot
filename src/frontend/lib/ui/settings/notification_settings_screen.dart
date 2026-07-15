import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/notification_settings_models.dart';
import '../../backend/notifications_client.dart';
import '../../l10n/app_localizations.dart';
import '../../settings/notification_quiet_hours_storage.dart';
import '../../state/auth_providers.dart';
import '../../state/push_notifications_controller.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_primary_button.dart';

/// Global and per-chat notification preferences (docs/features/notifications.md).
class NotificationSettingsScreen extends ConsumerStatefulWidget {
  const NotificationSettingsScreen({super.key, this.chatId});

  static const Key screenKey = Key('notification_settings_screen');
  static const Key saveButtonKey = Key('notification_save');
  static const Key pushPermissionKey = Key('notification_push_permission');
  static const Key quietHoursKey = Key('notification_quiet_hours');

  final String? chatId;

  bool get isChatScope => chatId != null && chatId!.isNotEmpty;

  @override
  ConsumerState<NotificationSettingsScreen> createState() =>
      _NotificationSettingsScreenState();
}

class _NotificationSettingsScreenState
    extends ConsumerState<NotificationSettingsScreen> {
  VoiceNotificationSettings? _settings;
  VoiceQuietHours _quietHours = VoiceQuietHours.defaults;
  PushPermissionStatus _pushPermission = PushPermissionStatus.notDetermined;
  var _loading = true;
  var _saving = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    final auth = ref.read(authorizationHeaderProvider);
    final profileId = ref.read(authControllerProvider).activeProfileId;
    if (auth == null || profileId == null) {
      setState(() {
        _loading = false;
        _error = 'not authenticated';
      });
      return;
    }

    final scopeType = widget.isChatScope ? 'chat' : 'global';
    final scopeId = widget.chatId;

    final settingsResult = await ref
        .read(voiceNotificationsClientProvider)
        .getSettings(
          authorization: auth,
          scopeType: scopeType,
          scopeId: scopeId,
        );

    VoiceQuietHours quietHours = VoiceQuietHours.defaults;
    if (!widget.isChatScope) {
      quietHours = await ref
          .read(notificationQuietHoursStorageProvider)
          .read(profileId);
      final push = ref.read(pushNotificationsControllerProvider);
      _pushPermission = await push.getPermissionStatus();
    }

    if (!mounted) return;
    switch (settingsResult) {
      case NotificationsApiOk(:final data):
        setState(() {
          var next = data.profileId.isEmpty
              ? data.copyWith(profileId: profileId)
              : data;
          if (widget.isChatScope) {
            next = next.copyWith(
              scopeType: 'chat',
              scopeId: widget.chatId,
            );
          }
          _settings = next;
          _quietHours = quietHours;
          _loading = false;
          _error = null;
        });
      case NotificationsApiFailure(:final message):
        setState(() {
          _loading = false;
          _error = message;
        });
    }
  }

  Future<void> _save() async {
    final settings = _settings;
    final auth = ref.read(authorizationHeaderProvider);
    if (settings == null || auth == null) return;

    setState(() {
      _saving = true;
      _error = null;
    });

    final client = ref.read(voiceNotificationsClientProvider);
    final settingsResult = await client.updateSettings(
      authorization: auth,
      settings: settings,
    );

    NotificationsApiFailure? quietFailure;
    if (!widget.isChatScope) {
      final profileId = ref.read(authControllerProvider).activeProfileId;
      if (profileId != null) {
        await ref
            .read(notificationQuietHoursStorageProvider)
            .write(profileId, _quietHours);
      }
      final quietResult = await client.setQuietHours(
        authorization: auth,
        quietHours: _quietHours,
      );
      if (quietResult is NotificationsApiFailure) {
        quietFailure = quietResult;
      }
    }

    if (!mounted) return;
    switch (settingsResult) {
      case NotificationsApiOk(:final data):
        setState(() {
          _settings = data;
          _saving = false;
        });
        final l10n = AppLocalizations.of(context)!;
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              quietFailure == null
                  ? l10n.notificationSettingsSaved
                  : l10n.notificationSettingsSavedQuietHoursFailed,
            ),
          ),
        );
      case NotificationsApiFailure(:final message):
        setState(() {
          _saving = false;
          _error = message;
        });
    }
  }

  Future<void> _showPushPermissionExplainer() async {
    final l10n = AppLocalizations.of(context)!;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l10n.notificationPushExplainerTitle),
        content: Text(l10n.notificationPushExplainerBody),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(l10n.commonCancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(l10n.notificationPushExplainerContinue),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;

    final status = await ref
        .read(pushNotificationsControllerProvider)
        .requestPermissionAndRegister();
    if (!mounted) return;
    setState(() => _pushPermission = status);
    final l10nAfter = AppLocalizations.of(context)!;
    final message = switch (status) {
      PushPermissionStatus.granted => l10nAfter.notificationPushEnabled,
      PushPermissionStatus.denied => l10nAfter.notificationPushDenied,
      PushPermissionStatus.unsupported => l10nAfter.notificationPushUnsupported,
      PushPermissionStatus.notDetermined => l10nAfter.notificationPushDenied,
    };
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
  }

  String _eventLabel(AppLocalizations l10n, String type) {
    return switch (type) {
      NotificationEventTypes.newMessage => l10n.notificationTypeNewMessage,
      NotificationEventTypes.mention => l10n.notificationTypeMention,
      NotificationEventTypes.reply => l10n.notificationTypeReply,
      NotificationEventTypes.reaction => l10n.notificationTypeReaction,
      NotificationEventTypes.friendRequest => l10n.notificationTypeFriendRequest,
      NotificationEventTypes.matchFound => l10n.notificationTypeMatchFound,
      NotificationEventTypes.system => l10n.notificationTypeSystem,
      _ => type,
    };
  }

  String _pushStatusLabel(AppLocalizations l10n) {
    return switch (_pushPermission) {
      PushPermissionStatus.granted => l10n.notificationPushStatusGranted,
      PushPermissionStatus.denied => l10n.notificationPushStatusDenied,
      PushPermissionStatus.unsupported => l10n.notificationPushStatusUnsupported,
      PushPermissionStatus.notDetermined => l10n.notificationPushStatusNotDetermined,
    };
  }

  Future<void> _pickQuietHour(
    BuildContext context, {
    required String initial,
    required ValueChanged<String> onSelected,
  }) async {
    final parts = initial.split(':');
    final initialTime = TimeOfDay(
      hour: int.tryParse(parts.first) ?? 0,
      minute: parts.length > 1 ? (int.tryParse(parts[1]) ?? 0) : 0,
    );
    final picked = await showTimePicker(context: context, initialTime: initialTime);
    if (picked == null) return;
    final formatted =
        '${picked.hour.toString().padLeft(2, '0')}:${picked.minute.toString().padLeft(2, '0')}';
    onSelected(formatted);
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final settings = _settings;
    final title = widget.isChatScope
        ? l10n.notificationChatSettingsTitle
        : l10n.notificationSettingsTitle;

    return Scaffold(
      key: NotificationSettingsScreen.screenKey,
      backgroundColor: voice.canvas,
      appBar: AppBar(
        title: Text(title),
        backgroundColor: voice.surface,
      ),
      body: SafeArea(
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : settings == null
            ? Center(child: Text(_error ?? l10n.notificationLoadError))
            : SingleChildScrollView(
                padding: const EdgeInsets.all(24),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    if (!widget.isChatScope) ...[
                      Text(
                        l10n.notificationPushSectionTitle,
                        style: TextStyle(color: voice.textSecondary),
                      ),
                      const SizedBox(height: 8),
                      ListTile(
                        key: NotificationSettingsScreen.pushPermissionKey,
                        contentPadding: EdgeInsets.zero,
                        title: Text(l10n.notificationPushEnableTitle),
                        subtitle: Text(_pushStatusLabel(l10n)),
                        trailing: _pushPermission == PushPermissionStatus.granted
                            ? Icon(Icons.check_circle_outline, color: voice.textSecondary)
                            : const Icon(Icons.chevron_right),
                        onTap: _pushPermission == PushPermissionStatus.granted
                            ? null
                            : _showPushPermissionExplainer,
                      ),
                      const SizedBox(height: 24),
                    ],
                    SwitchListTile(
                      contentPadding: EdgeInsets.zero,
                      title: Text(
                        widget.isChatScope
                            ? l10n.notificationChatEnabled
                            : l10n.notificationGlobalEnabled,
                      ),
                      value: settings.enabled,
                      onChanged: (v) =>
                          setState(() => _settings = settings.copyWith(enabled: v)),
                    ),
                    const SizedBox(height: 16),
                    Text(
                      l10n.notificationEventTypesTitle,
                      style: TextStyle(color: voice.textSecondary),
                    ),
                    const SizedBox(height: 8),
                    for (final type in NotificationEventTypes.all)
                      SwitchListTile(
                        key: Key('notification_type_$type'),
                        contentPadding: EdgeInsets.zero,
                        title: Text(_eventLabel(l10n, type)),
                        value: settings.enabled && settings.isTypeEnabled(type),
                        onChanged: settings.enabled
                            ? (v) => setState(
                                () => _settings = settings.withTypeEnabled(type, v),
                              )
                            : null,
                      ),
                    if (!widget.isChatScope) ...[
                      const SizedBox(height: 24),
                      Text(
                        l10n.notificationQuietHoursTitle,
                        style: TextStyle(color: voice.textSecondary),
                      ),
                      const SizedBox(height: 8),
                      SwitchListTile(
                        key: NotificationSettingsScreen.quietHoursKey,
                        contentPadding: EdgeInsets.zero,
                        title: Text(l10n.notificationQuietHoursEnabled),
                        value: _quietHours.enabled,
                        onChanged: (v) => setState(
                          () => _quietHours = _quietHours.copyWith(enabled: v),
                        ),
                      ),
                      if (_quietHours.enabled) ...[
                        ListTile(
                          contentPadding: EdgeInsets.zero,
                          title: Text(l10n.notificationQuietHoursStart),
                          trailing: Text(_quietHours.startTime),
                          onTap: () => _pickQuietHour(
                            context,
                            initial: _quietHours.startTime,
                            onSelected: (v) => setState(
                              () => _quietHours = _quietHours.copyWith(startTime: v),
                            ),
                          ),
                        ),
                        ListTile(
                          contentPadding: EdgeInsets.zero,
                          title: Text(l10n.notificationQuietHoursEnd),
                          trailing: Text(_quietHours.endTime),
                          onTap: () => _pickQuietHour(
                            context,
                            initial: _quietHours.endTime,
                            onSelected: (v) => setState(
                              () => _quietHours = _quietHours.copyWith(endTime: v),
                            ),
                          ),
                        ),
                        SwitchListTile(
                          contentPadding: EdgeInsets.zero,
                          title: Text(l10n.notificationQuietHoursOverrideMentions),
                          subtitle: Text(l10n.notificationQuietHoursOverrideMentionsHint),
                          value: _quietHours.overrideMentions,
                          onChanged: (v) => setState(
                            () => _quietHours =
                                _quietHours.copyWith(overrideMentions: v),
                          ),
                        ),
                      ],
                    ],
                    if (_error != null) ...[
                      const SizedBox(height: 12),
                      Text(
                        _error!,
                        style: TextStyle(color: Theme.of(context).colorScheme.error),
                      ),
                    ],
                    const SizedBox(height: 24),
                    VoicePrimaryButton(
                      key: NotificationSettingsScreen.saveButtonKey,
                      onPressed: _saving ? null : _save,
                      isLoading: _saving,
                      child: Text(l10n.commonSave),
                    ),
                  ],
                ),
              ),
      ),
    );
  }
}
