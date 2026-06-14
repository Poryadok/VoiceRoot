import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/subscription_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/subscription_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_primary_button.dart';

/// Pick two profiles to keep when downgrading from premium.
class ProfileDowngradePickerScreen extends ConsumerStatefulWidget {
  const ProfileDowngradePickerScreen({super.key});

  static const Key pickerKey = Key('downgrade_profile_picker');

  @override
  ConsumerState<ProfileDowngradePickerScreen> createState() =>
      _ProfileDowngradePickerScreenState();
}

class _ProfileDowngradePickerScreenState
    extends ConsumerState<ProfileDowngradePickerScreen> {
  final _selected = <String>{};
  var _submitting = false;
  String? _error;

  Future<void> _submit() async {
    if (_selected.length != 2) return;
    final session = ref.read(authControllerProvider).session;
    if (session == null) return;
    setState(() {
      _submitting = true;
      _error = null;
    });
    final result = await ref
        .read(voiceSubscriptionClientProvider)
        .submitDowngradeProfiles(
          authorization: session.authorizationHeader,
          keptProfileIds: _selected.toList(growable: false),
        );
    if (!mounted) return;
    switch (result) {
      case SubscriptionApiOk():
        Navigator.of(context).maybePop();
      case SubscriptionApiFailure(:final message):
        setState(() {
          _submitting = false;
          _error = message;
        });
    }
  }

  void _toggleProfile(String profileId, bool selected) {
    setState(() {
      if (selected) {
        if (_selected.length >= 2) return;
        _selected.add(profileId);
      } else {
        _selected.remove(profileId);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final profilesAsync = ref.watch(myProfilesProvider);

    return KeyedSubtree(
      key: ProfileDowngradePickerScreen.pickerKey,
      child: profilesAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (_, _) => Center(child: Text(l10n.subscriptionProfilesLoadError)),
        data: (profiles) {
          return Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  l10n.downgradeProfilePickerTitle,
                  style: Theme.of(context).textTheme.titleLarge,
                ),
                const SizedBox(height: 8),
                Text(
                  l10n.downgradeProfilePickerHint,
                  style: TextStyle(color: voice.textSecondary),
                ),
                const SizedBox(height: 16),
                Expanded(
                  child: ListView.builder(
                    itemCount: profiles.length,
                    itemBuilder: (context, index) {
                      final profile = profiles[index];
                      final checked = _selected.contains(profile.id);
                      final disabled =
                          !checked && _selected.length >= 2;
                      return CheckboxListTile(
                        value: checked,
                        onChanged: disabled && !checked
                            ? null
                            : (value) =>
                                  _toggleProfile(profile.id, value ?? false),
                        title: Text(profile.displayName),
                        subtitle: profile.isPrimary
                            ? Text(l10n.downgradeProfilePrimary)
                            : null,
                      );
                    },
                  ),
                ),
                if (_error != null) ...[
                  Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
                  const SizedBox(height: 8),
                ],
                VoicePrimaryButton(
                  onPressed: _selected.length == 2 && !_submitting
                      ? _submit
                      : null,
                  isLoading: _submitting,
                  child: Text(l10n.downgradeProfilePickerConfirm),
                ),
              ],
            ),
          );
        },
      ),
    );
  }
}
