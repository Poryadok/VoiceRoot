import 'package:flutter/material.dart';

import '../../l10n/app_localizations.dart';
import '../../theme/voice_colors.dart';

/// Story visibility selector (everyone / friends / close friends).
class StoryAudiencePicker extends StatelessWidget {
  const StoryAudiencePicker({
    super.key,
    required this.value,
    required this.onChanged,
  });

  static const visibilityKey = Key('story_create_visibility');

  final String value;
  final ValueChanged<String> onChanged;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);

    return Column(
      key: visibilityKey,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          l10n.storyCreateVisibilityLabel,
          style: TextStyle(color: voice.textSecondary),
        ),
        const SizedBox(height: 8),
        SegmentedButton<String>(
          segments: [
            ButtonSegment(
              value: 'everyone',
              label: Text(l10n.storyVisibilityEveryone),
            ),
            ButtonSegment(
              value: 'friends',
              label: Text(l10n.storyVisibilityFriends),
            ),
            ButtonSegment(
              value: 'close_friends',
              label: Text(l10n.storyVisibilityCloseFriends),
            ),
          ],
          selected: {_normalize(value)},
          onSelectionChanged: (selection) => onChanged(selection.first),
        ),
      ],
    );
  }

  static String _normalize(String value) {
    return switch (value) {
      'public' => 'everyone',
      'close_friends' => 'close_friends',
      'friends' => 'friends',
      _ => 'everyone',
    };
  }
}
