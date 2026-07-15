import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/users_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/social_providers.dart';
import '../../state/subscription_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_theme_providers.dart';
import '../core/profile_accent_dot.dart';

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
    final auth = ref.watch(authControllerProvider);
    final activeId = auth.activeProfileId;
    final isGuest = auth.isGuest;
    final profilesAsync = ref.watch(myProfilesProvider);
    final activeProfileAsync = ref.watch(activeProfileProvider);
    final activeProfile = activeProfileAsync.valueOrNull;
    final switching = ref.watch(profileSwitchInProgressProvider);

    Widget sessionText(String label) {
      return Text(
        label,
        key: const Key('auth_session_profile'),
        overflow: TextOverflow.ellipsis,
        style: TextStyle(color: voice.textPrimary),
      );
    }

    String fallbackLabel() => l10n.authSessionProfile(activeId ?? '…');

    String resolvedLabel(List<VoiceProfile> profiles) =>
        _labelFor(profiles, activeId, l10n, activeProfile, isGuest);

    return profilesAsync.when(
      loading: () {
        if (activeProfile != null && activeProfile.id == activeId) {
          return sessionText(
            profileSessionBarLabel(activeProfile, isGuest: isGuest),
          );
        }
        return sessionText(fallbackLabel());
      },
      error: (_, _) {
        if (activeProfile != null && activeProfile.id == activeId) {
          return sessionText(
            profileSessionBarLabel(activeProfile, isGuest: isGuest),
          );
        }
        return sessionText(fallbackLabel());
      },
      data: (profiles) {
        if (profiles.length <= 1 || activeId == null) {
          return sessionText(resolvedLabel(profiles));
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
                  child: Row(
                    children: [
                      _ProfileAccentFor(profile),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          _profileMenuLabel(profile, l10n),
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                    ],
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
    VoiceProfile? activeProfile,
    bool isGuest,
  ) {
    if (activeId == null) return l10n.authSessionProfile('…');

    VoiceProfile? resolved;
    if (activeProfile != null && activeProfile.id == activeId) {
      resolved = activeProfile;
    } else {
      for (final profile in profiles) {
        if (profile.id == activeId) {
          resolved = profile;
          break;
        }
      }
    }

    if (resolved != null) {
      return profileSessionBarLabel(resolved, isGuest: isGuest);
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

class _ProfileAccentFor extends ConsumerWidget {
  const _ProfileAccentFor(this.profile);

  final VoiceProfile profile;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final accentAsync = ref.watch(profileAccentColorProvider(profile.id));
    return accentAsync.when(
      data: (color) => ProfileAccentDot(size: 10, color: color),
      loading: () => const ProfileAccentDot(size: 10),
      error: (_, _) => const ProfileAccentDot(size: 10),
    );
  }
}
