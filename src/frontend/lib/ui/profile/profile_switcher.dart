import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/users_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/subscription_providers.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';

/// Desktop dropdown to switch the active profile (multi-profile/verification (docs/features/multi-profile.md)).
class ProfileSwitcher extends ConsumerWidget {
  const ProfileSwitcher({super.key});

  static const Key switcherKey = Key('profile_switcher');

  Future<void> _switchProfile(WidgetRef ref, String profileId) async {
    ref.read(profileSwitchInProgressProvider.notifier).state = true;
    try {
      final err = await ref
          .read(authControllerProvider.notifier)
          .switchActiveProfile(profileId);
      if (err == null) {
        ref.invalidate(activeProfileProvider);
        ref.invalidate(profileProvider(profileId));
        ref.invalidate(myProfilesProvider);
        ref.invalidate(subscriptionProvider);
      }
    } finally {
      ref.read(profileSwitchInProgressProvider.notifier).state = false;
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final activeId = ref.watch(authControllerProvider).activeProfileId;
    final profilesAsync = ref.watch(myProfilesProvider);
    final switching = ref.watch(profileSwitchInProgressProvider);

    return profilesAsync.when(
      loading: () => Text(
        l10n.authSessionProfile(activeId ?? '…'),
        key: const Key('auth_session_profile'),
        overflow: TextOverflow.ellipsis,
        style: TextStyle(color: voice.textPrimary),
      ),
      error: (_, _) => Text(
        l10n.authSessionProfile(activeId ?? '…'),
        key: const Key('auth_session_profile'),
        overflow: TextOverflow.ellipsis,
        style: TextStyle(color: voice.textPrimary),
      ),
      data: (profiles) {
        if (profiles.length <= 1 || activeId == null) {
          final label = _labelFor(profiles, activeId, l10n);
          return Text(
            label,
            key: const Key('auth_session_profile'),
            overflow: TextOverflow.ellipsis,
            style: TextStyle(color: voice.textPrimary),
          );
        }
        return DropdownButtonHideUnderline(
          child: DropdownButton<String>(
            key: switcherKey,
            isExpanded: true,
            value: activeId,
            icon: switching
                ? const SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : Icon(Icons.expand_more, color: voice.textSecondary),
            items: [
              for (final profile in profiles)
                DropdownMenuItem<String>(
                  value: profile.id,
                  child: Text(
                    _profileMenuLabel(profile, l10n),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
            ],
            onChanged: switching
                ? null
                : (nextId) {
                    if (nextId == null || nextId == activeId) return;
                    _switchProfile(ref, nextId);
                  },
          ),
        );
      },
    );
  }

  String _labelFor(
    List<VoiceProfile> profiles,
    String? activeId,
    AppLocalizations l10n,
  ) {
    if (activeId == null) return l10n.authSessionProfile('…');
    for (final profile in profiles) {
      if (profile.id == activeId) return profile.handle;
    }
    return l10n.authSessionProfile(activeId);
  }

  String _profileMenuLabel(VoiceProfile profile, AppLocalizations l10n) {
    if (profile.isPrimary) {
      return '${profile.displayName} (${l10n.downgradeProfilePrimary})';
    }
    return profile.displayName;
  }
}
