import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/users_client.dart';
import '../../backend/auth_session.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/profile_switch_coordinator.dart';
import '../../state/social_providers.dart';
import '../../state/subscription_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_theme_providers.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';
import '../settings/privacy_presets.dart';
import '../settings/subscription_settings_screen.dart';
import 'profile_edit_sheet.dart';

class CreateProfileSheet extends ConsumerStatefulWidget {
  const CreateProfileSheet({super.key, this.avatarPicker});

  static const Key sheetKey = Key('create_profile_sheet');
  static const Key displayNameFieldKey = Key('create_profile_display_name');
  static const Key usernameFieldKey = Key('create_profile_username');
  static const Key presetKey = Key('create_profile_preset');
  static const Key accentPickerKey = Key('create_profile_accent_picker');
  static const Key avatarButtonKey = Key('create_profile_avatar');
  static const Key submitKey = Key('create_profile_submit');

  final ProfileAvatarPicker? avatarPicker;

  @override
  ConsumerState<CreateProfileSheet> createState() => _CreateProfileSheetState();
}

class _CreateProfileSheetState extends ConsumerState<CreateProfileSheet> {
  final _displayNameController = TextEditingController();
  final _usernameController = TextEditingController();
  String _preset = PrivacyPresetDefaults.presets.first;
  int _selectedAccentIndex = 0;
  ProfileAvatarFile? _avatar;
  var _submitting = false;
  String? _error;
  _CreatedProfileReceipt? _receipt;
  var _switchApplied = false;

  @override
  void dispose() {
    _displayNameController.dispose();
    _usernameController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final l10n = AppLocalizations.of(context)!;
    final receipt = _receipt;
    if (receipt != null) {
      await _recover(receipt, l10n);
      return;
    }
    final name = _displayNameController.text.trim();
    if (name.isEmpty) {
      setState(() => _error = l10n.profileErrorDisplayNameRequired);
      return;
    }
    if (name.length > kProfileDisplayNameMaxLength) {
      setState(() => _error = l10n.profileErrorDisplayNameTooLong);
      return;
    }

    final session = ref.read(authControllerProvider).session;
    if (session == null) return;
    final auth = session.authorizationHeader;
    final username = _usernameController.text.trim();
    final accentColor = _accentHexForSubmit(context);

    setState(() {
      _submitting = true;
      _error = null;
    });

    final result = await ref
        .read(voiceUsersClientProvider)
        .createProfile(
          authorization: auth,
          displayName: name,
          username: username.isEmpty ? null : username,
          preset: _preset,
          accentColor: accentColor,
        );

    if (!mounted) return;
    switch (result) {
      case UsersApiOk(:final data):
        _receipt = _CreatedProfileReceipt(
          profile: data,
          sourceSession: session,
          displayName: name,
          username: username,
          preset: _preset,
          accentColor: accentColor,
        );
        if (!_isCurrentSource(_receipt!)) {
          _showRecoveryError(l10n.profileCreateSessionChanged);
          return;
        }
        if (data.accountId != session.accountId) {
          _showRecoveryError(l10n.profileCreateRecoveryFailed);
          return;
        }
        if (data.id.trim().isEmpty) {
          _showRecoveryError(l10n.profileCreateRecoveryFailed);
          return;
        }
        ref.invalidate(myProfilesProvider);
        await _recover(_receipt!, l10n);
      case UsersApiFailure(:final message, :final statusCode, :final errorCode):
        if (statusCode == 429 || errorCode == 'resource_exhausted') {
          if (!mounted) return;
          Navigator.of(context).pop();
          await Navigator.of(context).push(
            MaterialPageRoute<void>(
              builder: (_) => const SubscriptionSettingsScreen(),
            ),
          );
          return;
        }
        setState(() {
          _submitting = false;
          _error = _safeCreateError(l10n, message);
        });
    }
  }

  String _safeCreateError(AppLocalizations l10n, String message) {
    if (message == 'display_name_required') {
      return l10n.profileErrorDisplayNameRequired;
    }
    if (message == 'display_name_too_long') {
      return l10n.profileErrorDisplayNameTooLong;
    }
    return l10n.profileEditSaveError(l10n.commonRetry);
  }

  Future<void> _recover(
    _CreatedProfileReceipt receipt,
    AppLocalizations l10n,
  ) async {
    if (receipt.profile.accountId != receipt.accountId) {
      _showRecoveryError(l10n.profileCreateRecoveryFailed);
      return;
    }
    if (receipt.profile.id.trim().isEmpty) {
      _showRecoveryError(l10n.profileCreateRecoveryFailed);
      return;
    }
    final current = ref.read(authControllerProvider).session;
    if (current == null || current.accountId != receipt.accountId) {
      _showRecoveryError(l10n.profileCreateSessionChanged);
      return;
    }
    setState(() {
      _submitting = true;
      _error = null;
    });

    if (!_switchApplied) {
      if (current.activeProfileId != receipt.sourceProfileId) {
        _showRecoveryError(l10n.profileCreateSessionChanged);
        return;
      }
      ref.read(profileSwitchInProgressProvider.notifier).state = true;
      ProfileSwitchResult result;
      try {
        result = await ref
            .read(profileSwitchCoordinatorProvider)
            .switchTo(receipt.profile.id);
      } on Object {
        if (mounted) _showRecoveryError(l10n.profileCreateRecoveryFailed);
        return;
      } finally {
        if (mounted) {
          ref.read(profileSwitchInProgressProvider.notifier).state = false;
        }
      }
      if (!mounted) return;
      if (result is! ProfileSwitchApplied) {
        _showRecoveryError(
          _isCurrentSource(receipt)
              ? l10n.profileCreateRecoveryFailed
              : l10n.profileCreateSessionChanged,
        );
        return;
      }
      if (result.handoff.nextSession.accountId != receipt.accountId ||
          result.handoff.nextSession.activeProfileId != receipt.profile.id ||
          ref.read(authControllerProvider).session !=
              result.handoff.nextSession) {
        _showRecoveryError(l10n.profileCreateSessionChanged);
        return;
      }
      receipt.targetSession = result.handoff.nextSession;
      if (!_isCurrentTarget(receipt)) {
        _showRecoveryError(l10n.profileCreateSessionChanged);
        return;
      }
      _switchApplied = true;
    } else if (!_isCurrentTarget(receipt)) {
      _showRecoveryError(l10n.profileCreateSessionChanged);
      return;
    }

    final targetSession = ref.read(authControllerProvider).session;
    if (targetSession == null || !_isCurrentTarget(receipt)) {
      _showRecoveryError(l10n.profileCreateSessionChanged);
      return;
    }
    if (_avatar != null) {
      final avatarErr = await _uploadAvatarForActiveProfile(
        l10n,
        receipt,
        targetSession.authorizationHeader,
        _avatar!,
      );
      if (!mounted) return;
      if (avatarErr != null) {
        _showRecoveryError(avatarErr);
        return;
      }
    }
    if (!_isCurrentTarget(receipt)) {
      _showRecoveryError(l10n.profileCreateSessionChanged);
      return;
    }
    if (mounted) Navigator.of(context).pop(true);
  }

  bool _isCurrentTarget(_CreatedProfileReceipt receipt) {
    final session = ref.read(authControllerProvider).session;
    final targetSession = receipt.targetSession;
    return session != null && targetSession != null && session == targetSession;
  }

  bool _isCurrentSource(_CreatedProfileReceipt receipt) {
    final session = ref.read(authControllerProvider).session;
    return session != null && session == receipt.sourceSession;
  }

  void _showRecoveryError(String error) {
    if (!mounted) return;
    setState(() {
      _submitting = false;
      _error = error;
    });
  }

  String? _accentHexForSubmit(BuildContext context) {
    final catalog = ref.read(voiceTokenCatalogProvider).value;
    if (catalog == null || catalog.profileAccentDefaults.isEmpty) {
      return null;
    }
    final color =
        catalog.profileAccentDefaults[_selectedAccentIndex %
            catalog.profileAccentDefaults.length];
    return '#${(color.toARGB32() & 0xFFFFFF).toRadixString(16).padLeft(6, '0').toUpperCase()}';
  }

  Future<void> _pickAvatar() async {
    final picker = widget.avatarPicker ?? defaultPickProfileAvatar;
    final picked = await picker();
    if (!mounted || picked == null) return;
    setState(() => _avatar = picked);
  }

  Future<String?> _uploadAvatarForActiveProfile(
    AppLocalizations l10n,
    _CreatedProfileReceipt receipt,
    String authorization,
    ProfileAvatarFile avatar,
  ) async {
    if (!_isCurrentTarget(receipt)) return l10n.profileCreateSessionChanged;
    final client = ref.read(voiceUsersClientProvider);
    final presignResult = await client.createAvatarPresignedUpload(
      authorization: authorization,
      contentType: avatar.contentType,
      contentLength: avatar.bytes.length,
    );
    switch (presignResult) {
      case UsersApiFailure():
        if (!_isCurrentTarget(receipt)) {
          return l10n.profileCreateSessionChanged;
        }
        return _avatarRecoveryError(l10n);
      case UsersApiOk(:final data):
        if (!_isCurrentTarget(receipt)) {
          return l10n.profileCreateSessionChanged;
        }
        final uploadResult = await client.uploadAvatarBytes(
          uploadUrl: Uri.parse(data.uploadUrl),
          requiredHeaders: data.requiredHeaders,
          bytes: avatar.bytes,
        );
        switch (uploadResult) {
          case UsersApiFailure():
            if (!_isCurrentTarget(receipt)) {
              return l10n.profileCreateSessionChanged;
            }
            return _avatarRecoveryError(l10n);
          case UsersApiOk():
            if (!_isCurrentTarget(receipt)) {
              return l10n.profileCreateSessionChanged;
            }
            final updateResult = await client.updateProfile(
              authorization: authorization,
              avatarUrl: data.publicUrl,
            );
            if (!_isCurrentTarget(receipt)) {
              return l10n.profileCreateSessionChanged;
            }
            return switch (updateResult) {
              UsersApiOk() => null,
              UsersApiFailure() => _avatarRecoveryError(l10n),
            };
        }
    }
  }

  String _avatarRecoveryError(AppLocalizations l10n) =>
      l10n.profileCreateRecoveryFailed;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final profilesAsync = ref.watch(myProfilesProvider);
    final tier = ref.watch(subscriptionTierProvider);
    final maxProfiles = tier == 'premium' ? 5 : 2;
    final catalogAsync = ref.watch(voiceTokenCatalogProvider);

    return SafeArea(
      child: Padding(
        key: CreateProfileSheet.sheetKey,
        padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
        child: profilesAsync.when(
          loading: () => const VoiceListSkeleton(rowCount: 3),
          error: (_, _) => VoiceStatePanel(
            title: l10n.backendUnavailable,
            icon: Icons.cloud_off_outlined,
          ),
          data: (profiles) {
            if (_receipt == null && profiles.length >= maxProfiles) {
              return Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text(
                    l10n.createProfileTitle,
                    style: Theme.of(context).textTheme.titleLarge,
                  ),
                  const SizedBox(height: 12),
                  Text(
                    l10n.createProfileLimitReached,
                    style: TextStyle(color: voice.textSecondary),
                  ),
                  const SizedBox(height: 16),
                  FilledButton(
                    onPressed: () {
                      Navigator.of(context).pop();
                      Navigator.of(context).push(
                        MaterialPageRoute<void>(
                          builder: (_) => const SubscriptionSettingsScreen(),
                        ),
                      );
                    },
                    child: Text(l10n.createProfileOpenSubscription),
                  ),
                ],
              );
            }

            return Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  l10n.createProfileTitle,
                  style: Theme.of(context).textTheme.titleLarge,
                ),
                const SizedBox(height: 16),
                Row(
                  children: [
                    CircleAvatar(
                      radius: 28,
                      backgroundImage: _avatar != null
                          ? MemoryImage(_avatar!.bytes)
                          : null,
                      child: _avatar == null
                          ? Icon(
                              Icons.person_outline,
                              color: voice.textSecondary,
                            )
                          : null,
                    ),
                    const SizedBox(width: 16),
                    OutlinedButton.icon(
                      key: CreateProfileSheet.avatarButtonKey,
                      onPressed: _submitting ? null : _pickAvatar,
                      icon: const Icon(Icons.photo_outlined),
                      label: Text(l10n.profileAvatarChange),
                    ),
                  ],
                ),
                const SizedBox(height: 16),
                TextField(
                  key: CreateProfileSheet.displayNameFieldKey,
                  controller: _displayNameController,
                  decoration: InputDecoration(
                    labelText: l10n.profileDisplayNameLabel,
                  ),
                  textInputAction: TextInputAction.done,
                  enabled: !_submitting && _receipt == null,
                ),
                const SizedBox(height: 16),
                TextField(
                  key: CreateProfileSheet.usernameFieldKey,
                  controller: _usernameController,
                  decoration: InputDecoration(
                    labelText: l10n.profileCreateUsernameLabel,
                  ),
                  textInputAction: TextInputAction.done,
                  enabled: !_submitting && _receipt == null,
                ),
                const SizedBox(height: 16),
                Text(
                  l10n.createProfilePresetHint,
                  style: TextStyle(color: voice.textSecondary),
                ),
                const SizedBox(height: 8),
                SegmentedButton<String>(
                  key: CreateProfileSheet.presetKey,
                  segments: [
                    ButtonSegment(
                      value: 'personal',
                      label: Text(l10n.privacyPresetPersonal),
                    ),
                    ButtonSegment(
                      value: 'gaming',
                      label: Text(l10n.privacyPresetGaming),
                    ),
                    ButtonSegment(
                      value: 'work',
                      label: Text(l10n.privacyPresetWork),
                    ),
                  ],
                  selected: {_preset},
                  onSelectionChanged: _submitting || _receipt != null
                      ? null
                      : (next) => setState(() => _preset = next.first),
                ),
                const SizedBox(height: 16),
                Text(
                  l10n.settingsAccent,
                  style: TextStyle(color: voice.textSecondary),
                ),
                const SizedBox(height: 8),
                catalogAsync.when(
                  data: (catalog) => Wrap(
                    key: CreateProfileSheet.accentPickerKey,
                    spacing: 8,
                    runSpacing: 8,
                    children: [
                      for (
                        var i = 0;
                        i < catalog.profileAccentDefaults.length;
                        i++
                      )
                        GestureDetector(
                          onTap: _submitting || _receipt != null
                              ? null
                              : () => setState(() => _selectedAccentIndex = i),
                          child: Container(
                            width: 32,
                            height: 32,
                            decoration: BoxDecoration(
                              color: catalog.profileAccentDefaults[i],
                              shape: BoxShape.circle,
                              border: Border.all(
                                color: _selectedAccentIndex == i
                                    ? voice.textPrimary
                                    : voice.borderDefault,
                                width: _selectedAccentIndex == i ? 2 : 1,
                              ),
                            ),
                          ),
                        ),
                    ],
                  ),
                  loading: () => const LinearProgressIndicator(minHeight: 2),
                  error: (_, _) => const SizedBox.shrink(),
                ),
                if (_error != null) ...[
                  const SizedBox(height: 12),
                  Text(_error!, style: TextStyle(color: voice.error)),
                ],
                const SizedBox(height: 20),
                FilledButton(
                  key: CreateProfileSheet.submitKey,
                  onPressed: _submitting ? null : _submit,
                  child: _submitting
                      ? const SizedBox(
                          width: 18,
                          height: 18,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : Text(
                          _receipt == null
                              ? l10n.createProfileSubmit
                              : l10n.commonRetry,
                        ),
                ),
              ],
            );
          },
        ),
      ),
    );
  }
}

class _CreatedProfileReceipt {
  _CreatedProfileReceipt({
    required this.profile,
    required this.sourceSession,
    required this.displayName,
    required this.username,
    required this.preset,
    required this.accentColor,
  });

  final VoiceProfile profile;
  final AuthSession sourceSession;
  AuthSession? targetSession;
  final String displayName;
  final String username;
  final String preset;
  final String? accentColor;

  String get accountId => sourceSession.accountId;
  String get sourceProfileId => sourceSession.activeProfileId;
}

Future<bool?> showCreateProfileSheet(BuildContext context) {
  return showVoiceBottomSheet<bool>(
    context: context,
    child: const CreateProfileSheet(),
  );
}
