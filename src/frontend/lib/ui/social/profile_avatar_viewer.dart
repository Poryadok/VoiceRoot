import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/social_providers.dart';
import '../a11y/focus_trap.dart';
import '../core/voice_avatar.dart';
import '../core/voice_state_panel.dart';

/// The public profile image uses the existing profile authorization boundary.
/// Story media and its file authorization are deliberately not involved.
class ProfileAvatarViewer extends ConsumerWidget {
  const ProfileAvatarViewer({super.key, required this.profileId});

  final String profileId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final profile = ref.watch(profileProvider(profileId));
    final switching = ref.watch(profileSwitchInProgressProvider);
    return VoiceFocusTrap(
      onEscape: () => Navigator.of(context).pop(),
      child: Scaffold(
        key: const Key('profile_avatar_fullscreen'),
        appBar: AppBar(
          leading: CloseButton(key: const Key('profile_avatar_close')),
          title: Text(l10n.profileOpenAvatar),
        ),
        body: SafeArea(
          child: switching
              ? const Center(child: CircularProgressIndicator())
              : profile.when(
                  skipLoadingOnRefresh: false,
                  loading: () =>
                      const Center(child: CircularProgressIndicator()),
                  error: (_, _) => Center(
                    child: VoiceStatePanel(
                      title: l10n.socialProfileUnavailable,
                      actionLabel: l10n.commonRetry,
                      onAction: () =>
                          ref.invalidate(profileProvider(profileId)),
                    ),
                  ),
                  data: (value) {
                    if (value == null) {
                      return Center(child: Text(l10n.socialProfileUnavailable));
                    }
                    final url = value.avatarUrl;
                    if (url == null || url.isEmpty) {
                      return Center(
                        key: const Key('profile_avatar_missing'),
                        child: Column(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            VoiceAvatar(label: value.displayName, radius: 64),
                            const SizedBox(height: 16),
                            Text(l10n.profileAvatarUnavailable),
                          ],
                        ),
                      );
                    }
                    return _AvatarImage(
                      key: ValueKey((profileId, url)),
                      url: url,
                      label: value.displayName,
                    );
                  },
                ),
        ),
      ),
    );
  }
}

class _AvatarImage extends StatefulWidget {
  const _AvatarImage({super.key, required this.url, required this.label});

  final String url;
  final String label;

  @override
  State<_AvatarImage> createState() => _AvatarImageState();
}

class _AvatarImageState extends State<_AvatarImage> {
  int _attempt = 0;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return SizedBox.expand(
      child: Image.network(
        widget.url,
        key: ValueKey(_attempt),
        semanticLabel: widget.label,
        fit: BoxFit.contain,
        loadingBuilder: (_, child, progress) => progress == null
            ? child
            : const Center(child: CircularProgressIndicator()),
        errorBuilder: (_, _, _) => Center(
          child: VoiceStatePanel(
            key: const Key('profile_avatar_load_error'),
            title: l10n.profileAvatarUnavailable,
            actionLabel: l10n.commonRetry,
            onAction: () async {
              await NetworkImage(widget.url).evict();
              if (mounted) setState(() => _attempt++);
            },
          ),
        ),
      ),
    );
  }
}
