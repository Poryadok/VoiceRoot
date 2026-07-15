import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/users_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/social_providers.dart';
import '../../state/subscription_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_bottom_sheet.dart';
import '../settings/privacy_presets.dart';
import '../settings/subscription_settings_screen.dart';

class CreateProfileSheet extends ConsumerStatefulWidget {
  const CreateProfileSheet({super.key});

  static const Key sheetKey = Key('create_profile_sheet');
  static const Key displayNameFieldKey = Key('create_profile_display_name');
  static const Key presetKey = Key('create_profile_preset');
  static const Key submitKey = Key('create_profile_submit');

  @override
  ConsumerState<CreateProfileSheet> createState() => _CreateProfileSheetState();
}

class _CreateProfileSheetState extends ConsumerState<CreateProfileSheet> {
  final _displayNameController = TextEditingController();
  String _preset = PrivacyPresetDefaults.presets.first;
  var _submitting = false;
  String? _error;

  @override
  void dispose() {
    _displayNameController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final l10n = AppLocalizations.of(context)!;
    final name = _displayNameController.text.trim();
    if (name.isEmpty) {
      setState(() => _error = l10n.profileErrorDisplayNameRequired);
      return;
    }
    if (name.length > kProfileDisplayNameMaxLength) {
      setState(() => _error = l10n.profileErrorDisplayNameTooLong);
      return;
    }

    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;

    setState(() {
      _submitting = true;
      _error = null;
    });

    final result = await ref.read(voiceUsersClientProvider).createProfile(
      authorization: auth,
      displayName: name,
      preset: _preset,
    );

    if (!mounted) return;
    switch (result) {
      case UsersApiOk(:final data):
        ref.invalidate(myProfilesProvider);
        await ref
            .read(authControllerProvider.notifier)
            .switchActiveProfile(data.id);
        if (mounted) Navigator.of(context).pop(true);
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
          _error = message;
        });
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final profilesAsync = ref.watch(myProfilesProvider);
    final tier = ref.watch(subscriptionTierProvider);
    final maxProfiles = tier == 'premium' ? 5 : 2;

    return SafeArea(
      child: Padding(
        key: CreateProfileSheet.sheetKey,
        padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
        child: profilesAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (_, _) => Text(l10n.backendUnavailable),
          data: (profiles) {
            if (profiles.length >= maxProfiles) {
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
                TextField(
                  key: CreateProfileSheet.displayNameFieldKey,
                  controller: _displayNameController,
                  decoration: InputDecoration(
                    labelText: l10n.profileDisplayNameLabel,
                  ),
                  textInputAction: TextInputAction.done,
                  enabled: !_submitting,
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
                  onSelectionChanged: _submitting
                      ? null
                      : (next) => setState(() => _preset = next.first),
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
                      : Text(l10n.createProfileSubmit),
                ),
              ],
            );
          },
        ),
      ),
    );
  }
}

Future<bool?> showCreateProfileSheet(BuildContext context) {
  return showVoiceBottomSheet<bool>(context: context, child: const CreateProfileSheet());
}
