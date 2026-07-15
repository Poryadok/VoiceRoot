import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/users_client.dart';
import '../../state/auth_providers.dart';
import '../../state/subscription_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_theme_providers.dart';
import '../core/profile_accent_dot.dart';

/// Mobile profile switcher: swipe avatar left/right to cycle profiles.
class ProfileAvatarSwitcher extends ConsumerWidget {
  const ProfileAvatarSwitcher({
    super.key,
    required this.sessionLabel,
  });

  static const switcherKey = Key('profile_avatar_switcher');

  final String sessionLabel;

  Future<void> _switchTo(
    WidgetRef ref,
    BuildContext context,
    List<VoiceProfile> profiles,
    int nextIndex,
  ) async {
    final profile = profiles[nextIndex];
    final auth = ref.read(authControllerProvider);
    if (auth.activeProfileId == profile.id) return;

    ref.read(profileSwitchInProgressProvider.notifier).state = true;
    try {
      final err = await ref
          .read(authControllerProvider.notifier)
          .switchActiveProfile(profile.id);
      if (err == null) {
        HapticFeedback.selectionClick();
        ref.invalidate(myProfilesProvider);
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Text(profile.displayName),
              duration: const Duration(seconds: 2),
            ),
          );
        }
      }
    } finally {
      ref.read(profileSwitchInProgressProvider.notifier).state = false;
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final voice = VoiceColors.of(context);
    final activeId = ref.watch(authControllerProvider).activeProfileId;
    final profilesAsync = ref.watch(myProfilesProvider);
    final switching = ref.watch(profileSwitchInProgressProvider);

    return profilesAsync.when(
      loading: () => Text(
        sessionLabel,
        key: const Key('auth_session_profile'),
        overflow: TextOverflow.ellipsis,
        style: TextStyle(color: voice.textPrimary),
      ),
      error: (_, _) => Text(
        sessionLabel,
        key: const Key('auth_session_profile'),
        overflow: TextOverflow.ellipsis,
        style: TextStyle(color: voice.textPrimary),
      ),
      data: (profiles) {
        if (profiles.length <= 1 || activeId == null) {
          return Text(
            sessionLabel,
            key: const Key('auth_session_profile'),
            overflow: TextOverflow.ellipsis,
            style: TextStyle(color: voice.textPrimary),
          );
        }

        final currentIndex = profiles.indexWhere((p) => p.id == activeId);
        final safeIndex = currentIndex >= 0 ? currentIndex : 0;

        return GestureDetector(
          key: switcherKey,
          onHorizontalDragEnd: switching
              ? null
              : (details) {
                  final velocity = details.primaryVelocity ?? 0;
                  if (velocity.abs() < 80) return;
                  final next = velocity < 0
                      ? (safeIndex + 1) % profiles.length
                      : (safeIndex - 1 + profiles.length) % profiles.length;
                  _switchTo(ref, context, profiles, next);
                },
          child: Row(
            children: [
              if (switching)
                const SizedBox(
                  width: 14,
                  height: 14,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              else
                _ProfileAccentFor(profiles[safeIndex]),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  profiles[safeIndex].displayName,
                  key: const Key('auth_session_profile'),
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(color: voice.textPrimary),
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

class _ProfileAccentFor extends ConsumerWidget {
  const _ProfileAccentFor(this.profile);

  final VoiceProfile profile;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final accentAsync = ref.watch(profileAccentColorProvider(profile.id));
    return accentAsync.when(
      data: (color) => ProfileAccentDot(size: 12, color: color),
      loading: () => const ProfileAccentDot(size: 12),
      error: (_, _) => const ProfileAccentDot(size: 12),
    );
  }
}
