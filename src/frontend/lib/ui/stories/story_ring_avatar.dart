import 'package:flutter/material.dart';

import '../../l10n/app_localizations.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_avatar.dart';

/// Avatar with optional active-story ring (Telegram-style).
class StoryRingAvatar extends StatelessWidget {
  const StoryRingAvatar({
    super.key,
    required this.displayName,
    this.imageUrl,
    this.hasActiveStory = false,
    this.size = 48,
    this.onTap,
  });

  static const storyRingKey = Key('story_ring_active');

  final String displayName;
  final String? imageUrl;
  final bool hasActiveStory;
  final double size;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final radius = size / 2;
    final avatar = VoiceAvatar(
      imageUrl: imageUrl,
      label: displayName,
      radius: radius - (hasActiveStory ? 2 : 0),
    );

    Widget child = avatar;
    if (hasActiveStory) {
      child = Container(
        key: storyRingKey,
        padding: const EdgeInsets.all(2),
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          gradient: LinearGradient(
            colors: [
              voice.profileAccent,
              Theme.of(context).colorScheme.secondary,
            ],
          ),
        ),
        child: avatar,
      );
    }

    if (onTap != null) {
      child = InkWell(
        onTap: onTap,
        customBorder: const CircleBorder(),
        child: child,
      );
    }

    if (hasActiveStory) {
      child = Semantics(
        label: _activeStorySemanticsLabel(context),
        button: onTap != null,
        explicitChildNodes: true,
        child: child,
      );
    }

    return child;
  }

  static String _activeStorySemanticsLabel(BuildContext context) {
    try {
      return AppLocalizations.of(context)!.storyRingActiveLabel;
    } catch (_) {
      return 'Active story';
    }
  }
}
