import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/users_client.dart';
import '../../state/auth_providers.dart';
import '../../state/subscription_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_theme_providers.dart';
import '../core/profile_accent_dot.dart';
import 'profile_avatar_menu.dart';

/// Mobile profile switcher: tap avatar for §1.1a menu; swipe to cycle profiles.
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
    if (profile.isFrozen) return;

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

  int? _nextSwitchableIndex(
    List<VoiceProfile> profiles,
    int currentIndex,
    bool forward,
  ) {
    if (profiles.isEmpty) return null;
    final len = profiles.length;
    for (var step = 1; step < len; step++) {
      final idx = forward
          ? (currentIndex + step) % len
          : (currentIndex - step + len) % len;
      if (!profiles[idx].isFrozen) return idx;
    }
    return null;
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

        return Builder(
          builder: (anchorContext) {
            return GestureDetector(
              key: switcherKey,
              onTap: switching
                  ? null
                  : () => showProfileAvatarMenu(
                        context: context,
                        ref: ref,
                        anchorContext: anchorContext,
                      ),
              onHorizontalDragEnd: switching
                  ? null
                  : (details) {
                      final velocity = details.primaryVelocity ?? 0;
                      if (velocity.abs() < 80) return;
                      final next = _nextSwitchableIndex(
                        profiles,
                        safeIndex,
                        velocity < 0,
                      );
                      if (next == null || next == safeIndex) return;
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
