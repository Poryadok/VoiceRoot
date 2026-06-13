import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/user_privacy_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/trust_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_primary_button.dart';
import 'privacy_presets.dart';

/// Privacy presets and per-field visibility controls.
class PrivacySettingsScreen extends ConsumerStatefulWidget {
  const PrivacySettingsScreen({super.key});

  static const Key screenKey = Key('privacy_settings_screen');
  static const Key presetKey = Key('privacy_preset');
  static const Key allowDmKey = Key('privacy_allow_dm');
  static const Key saveButtonKey = Key('privacy_save');

  @override
  ConsumerState<PrivacySettingsScreen> createState() =>
      _PrivacySettingsScreenState();
}

class _PrivacySettingsScreenState extends ConsumerState<PrivacySettingsScreen> {
  VoicePrivacySettings? _settings;
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

    final result = await ref
        .read(voiceUserPrivacyClientProvider)
        .getPrivacy(authorization: auth);

    if (!mounted) return;
    switch (result) {
      case UserPrivacyApiOk(:final data):
        setState(() {
          _settings = data;
          _loading = false;
          _error = null;
        });
      case UserPrivacyApiFailure(:final message):
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

    final result = await ref
        .read(voiceUserPrivacyClientProvider)
        .updatePrivacy(authorization: auth, settings: settings);

    if (!mounted) return;
    switch (result) {
      case UserPrivacyApiOk(:final data):
        setState(() {
          _settings = data;
          _saving = false;
        });
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(AppLocalizations.of(context)!.privacySaved)),
        );
      case UserPrivacyApiFailure(:final message):
        setState(() {
          _saving = false;
          _error = message;
        });
    }
  }

  void _applyPreset(String preset) {
    final profileId = ref.read(authControllerProvider).activeProfileId;
    if (profileId == null) return;
    setState(() {
      _settings = PrivacyPresetDefaults.forPreset(preset, profileId: profileId);
    });
  }

  String _audienceLabel(AppLocalizations l10n, String value) {
    return switch (value) {
      'everyone' => l10n.privacyAudienceEveryone,
      'friends' => l10n.privacyAudienceFriends,
      'friends_of_friends' => l10n.privacyAudienceFriendsOfFriends,
      'nobody' => l10n.privacyAudienceNobody,
      _ => value,
    };
  }

  String _presetLabel(AppLocalizations l10n, String preset) {
    return switch (preset) {
      'personal' => l10n.privacyPresetPersonal,
      'gaming' => l10n.privacyPresetGaming,
      'work' => l10n.privacyPresetWork,
      _ => preset,
    };
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final settings = _settings;

    return Scaffold(
      key: PrivacySettingsScreen.screenKey,
      backgroundColor: voice.canvas,
      appBar: AppBar(
        title: Text(l10n.privacySettingsTitle),
        backgroundColor: voice.surface,
      ),
      body: SafeArea(
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : settings == null
            ? Center(child: Text(_error ?? l10n.privacyLoadError))
            : SingleChildScrollView(
                padding: const EdgeInsets.all(24),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text(
                      l10n.privacyPresetTitle,
                      style: TextStyle(color: voice.textSecondary),
                    ),
                    const SizedBox(height: 8),
                    SegmentedButton<String>(
                      key: PrivacySettingsScreen.presetKey,
                      segments: [
                        for (final preset in PrivacyPresetDefaults.presets)
                          ButtonSegment(
                            value: preset,
                            label: Text(_presetLabel(l10n, preset)),
                          ),
                      ],
                      selected: {settings.preset},
                      onSelectionChanged: (next) => _applyPreset(next.single),
                    ),
                    const SizedBox(height: 24),
                    _AudienceDropdown(
                      key: PrivacySettingsScreen.allowDmKey,
                      label: l10n.privacyAllowDm,
                      value: settings.allowDm,
                      values: kPrivacyAudienceValues,
                      labelFor: (v) => _audienceLabel(l10n, v),
                      onChanged: (v) => setState(
                        () => _settings = settings.copyWith(allowDm: v),
                      ),
                    ),
                    const SizedBox(height: 16),
                    _VisibilitySection(
                      l10n: l10n,
                      voice: voice,
                      settings: settings,
                      audienceLabel: _audienceLabel,
                      onChanged: (next) => setState(() => _settings = next),
                    ),
                    SwitchListTile(
                      title: Text(l10n.privacyAllowGuestDm),
                      value: settings.allowGuestDm,
                      onChanged: (v) => setState(
                        () => _settings = settings.copyWith(allowGuestDm: v),
                      ),
                    ),
                    if (_error != null) ...[
                      const SizedBox(height: 12),
                      Text(
                        _error!,
                        style: TextStyle(
                          color: Theme.of(context).colorScheme.error,
                        ),
                      ),
                    ],
                    const SizedBox(height: 24),
                    VoicePrimaryButton(
                      key: PrivacySettingsScreen.saveButtonKey,
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

class _VisibilitySection extends StatelessWidget {
  const _VisibilitySection({
    required this.l10n,
    required this.voice,
    required this.settings,
    required this.audienceLabel,
    required this.onChanged,
  });

  final AppLocalizations l10n;
  final VoiceColors voice;
  final VoicePrivacySettings settings;
  final String Function(AppLocalizations l10n, String value) audienceLabel;
  final ValueChanged<VoicePrivacySettings> onChanged;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          l10n.privacyVisibilityTitle,
          style: TextStyle(color: voice.textSecondary),
        ),
        const SizedBox(height: 8),
        _AudienceDropdown(
          label: l10n.privacyShowOnline,
          value: settings.showOnline,
          values: kPrivacyAudienceValues,
          labelFor: (v) => audienceLabel(l10n, v),
          onChanged: (v) => onChanged(settings.copyWith(showOnline: v)),
        ),
        const SizedBox(height: 12),
        _AudienceDropdown(
          label: l10n.privacyShowGameStatus,
          value: settings.showGameStatus,
          values: kPrivacyAudienceValues,
          labelFor: (v) => audienceLabel(l10n, v),
          onChanged: (v) => onChanged(settings.copyWith(showGameStatus: v)),
        ),
        const SizedBox(height: 12),
        _AudienceDropdown(
          label: l10n.privacyShowMmRating,
          value: settings.showMmRating,
          values: kPrivacyAudienceValues,
          labelFor: (v) => audienceLabel(l10n, v),
          onChanged: (v) => onChanged(settings.copyWith(showMmRating: v)),
        ),
        const SizedBox(height: 12),
        _AudienceDropdown(
          label: l10n.privacyShowPhone,
          value: settings.showPhone,
          values: kPrivacyPhoneAudienceValues,
          labelFor: (v) => audienceLabel(l10n, v),
          onChanged: (v) => onChanged(settings.copyWith(showPhone: v)),
        ),
        const SizedBox(height: 12),
        _AudienceDropdown(
          label: l10n.privacyShowStories,
          value: settings.showStories,
          values: kPrivacyAudienceValues,
          labelFor: (v) => audienceLabel(l10n, v),
          onChanged: (v) => onChanged(settings.copyWith(showStories: v)),
        ),
        const SizedBox(height: 12),
        _AudienceDropdown(
          label: l10n.privacyAllowFriendRequests,
          value: settings.allowFriendRequests,
          values: kPrivacyFriendRequestAudienceValues,
          labelFor: (v) => audienceLabel(l10n, v),
          onChanged: (v) =>
              onChanged(settings.copyWith(allowFriendRequests: v)),
        ),
      ],
    );
  }
}

class _AudienceDropdown extends StatelessWidget {
  const _AudienceDropdown({
    super.key,
    required this.label,
    required this.value,
    required this.values,
    required this.labelFor,
    required this.onChanged,
  });

  final String label;
  final String value;
  final List<String> values;
  final String Function(String value) labelFor;
  final ValueChanged<String> onChanged;

  @override
  Widget build(BuildContext context) {
    return DropdownButtonFormField<String>(
      initialValue: values.contains(value) ? value : values.first,
      decoration: InputDecoration(labelText: label),
      items: [
        for (final v in values)
          DropdownMenuItem(value: v, child: Text(labelFor(v))),
      ],
      onChanged: (v) {
        if (v != null) onChanged(v);
      },
    );
  }
}
