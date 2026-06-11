import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/spaces_client.dart';
import '../../state/space_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_avatar.dart';
/// Left rail: vertical list of joined spaces (Discord-style server icons).
class SpaceRail extends ConsumerWidget {
  const SpaceRail({super.key, this.onSpaceSelected});

  static const Key railKey = Key('space_rail');

  final void Function(String spaceId)? onSpaceSelected;
  static Key spaceIconKey(String spaceId) => Key('space_icon_$spaceId');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final voice = VoiceColors.of(context);
    final spacesAsync = ref.watch(mySpacesProvider);
    final selectedSpaceId = ref.watch(selectedSpaceIdProvider);

    return ColoredBox(
      color: voice.muted,
      child: spacesAsync.when(
        loading: () => const Center(
          child: SizedBox(
            width: 24,
            height: 24,
            child: CircularProgressIndicator(strokeWidth: 2),
          ),
        ),
        error: (error, _) => Center(
          child: IconButton(
            tooltip: 'Retry',
            onPressed: () => ref.invalidate(mySpacesProvider),
            icon: const Icon(Icons.refresh, size: 20),
          ),
        ),
        data: (data) => ListView(
          key: railKey,
          padding: const EdgeInsets.symmetric(vertical: 8),
          children: [
            for (final space in data.spaces)
              _SpaceIconButton(
                key: spaceIconKey(space.id),
                space: space,
                selected: selectedSpaceId == space.id,
                onTap: () {
                  if (onSpaceSelected != null) {
                    onSpaceSelected!(space.id);
                  } else {
                    ref.read(selectedSpaceIdProvider.notifier).state = space.id;
                  }
                },
              ),
          ],
        ),
      ),
    );
  }
}

class _SpaceIconButton extends StatelessWidget {
  const _SpaceIconButton({
    super.key,
    required this.space,
    required this.selected,
    required this.onTap,
  });

  final VoiceSpace space;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4, horizontal: 8),
      child: Material(
        color: selected ? voice.elevated : Colors.transparent,
        borderRadius: BorderRadius.circular(16),
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(16),
          child: Tooltip(
            message: space.name,
            child: Padding(
              padding: const EdgeInsets.all(4),
              child: VoiceAvatar(
                imageUrl: space.iconUrl,
                label: space.name,
                radius: 20,
              ),
            ),
          ),
        ),
      ),
    );
  }
}
