import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/spaces_client.dart';
import '../../backend/user_privacy_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/space_providers.dart';
import '../../theme/voice_colors.dart';

/// Multiselect audience control with Everyone/Nobody shortcuts and optional space picker.
class PrivacyAudiencePicker extends ConsumerWidget {
  const PrivacyAudiencePicker({
    super.key,
    required this.label,
    required this.value,
    required this.onChanged,
  });

  final String label;
  final VoicePrivacyAudience value;
  final ValueChanged<VoicePrivacyAudience> onChanged;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(label, style: TextStyle(color: voice.textSecondary)),
        const SizedBox(height: 8),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            FilterChip(
              label: Text(l10n.privacyAudienceEveryone),
              selected: value.isEveryoneShortcut,
              onSelected: (selected) {
                if (selected) onChanged(VoicePrivacyAudience.everyoneWithGuests);
              },
            ),
            FilterChip(
              label: Text(l10n.privacyAudienceNobody),
              selected: value.isNobody,
              onSelected: (selected) {
                if (selected) onChanged(VoicePrivacyAudience.nobody);
              },
            ),
            FilterChip(
              label: Text(l10n.privacyAudienceFriends),
              selected: value.friends,
              onSelected: (selected) =>
                  onChanged(value.copyWith(friends: selected)),
            ),
            FilterChip(
              label: Text(l10n.privacyAudienceFriendsOfFriends),
              selected: value.friendsOfFriends,
              onSelected: (selected) =>
                  onChanged(value.copyWith(friendsOfFriends: selected)),
            ),
            FilterChip(
              label: Text(l10n.privacyAudienceSpaceMembers),
              selected: value.spaceMembers,
              onSelected: (selected) => onChanged(
                value.copyWith(
                  spaceMembers: selected,
                  spaceIds: selected ? value.spaceIds : const [],
                ),
              ),
            ),
            FilterChip(
              label: Text(l10n.privacyAudienceIncludeGuests),
              selected: value.includeGuests,
              onSelected: (selected) =>
                  onChanged(value.copyWith(includeGuests: selected)),
            ),
          ],
        ),
        if (value.spaceMembers) ...[
          const SizedBox(height: 12),
          Text(
            l10n.privacyAudienceSpacesTitle,
            style: TextStyle(color: voice.textSecondary, fontSize: 13),
          ),
          const SizedBox(height: 8),
          _SpaceAudiencePicker(
            value: value,
            onChanged: onChanged,
          ),
        ],
      ],
    );
  }
}

class _SpaceAudiencePicker extends ConsumerWidget {
  const _SpaceAudiencePicker({
    required this.value,
    required this.onChanged,
  });

  final VoicePrivacyAudience value;
  final ValueChanged<VoicePrivacyAudience> onChanged;

  bool _isSpaceSelected(String spaceId, List<String> allIds) {
    if (value.spaceIds.isEmpty) return true;
    return value.spaceIds.contains(spaceId);
  }

  void _toggleSpace(String spaceId, List<String> allIds) {
    if (allIds.isEmpty) return;

    final currentlySelected = _isSpaceSelected(spaceId, allIds);
    if (currentlySelected) {
      if (value.spaceIds.isEmpty) {
        onChanged(
          value.copyWith(
            spaceIds: allIds.where((id) => id != spaceId).toList(),
          ),
        );
        return;
      }
      final next = value.spaceIds.where((id) => id != spaceId).toList();
      onChanged(value.copyWith(spaceIds: next));
      return;
    }

    final next = [...value.spaceIds, spaceId];
    if (next.length == allIds.length) {
      onChanged(value.copyWith(spaceIds: const []));
      return;
    }
    onChanged(value.copyWith(spaceIds: next));
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final spacesAsync = ref.watch(mySpacesProvider);

    return spacesAsync.when(
      loading: () => const SizedBox(
        height: 24,
        width: 24,
        child: CircularProgressIndicator(strokeWidth: 2),
      ),
      error: (error, stackTrace) => const SizedBox.shrink(),
      data: (SpaceListData data) {
        if (data.spaces.isEmpty) {
          return Text(
            AppLocalizations.of(context)!.privacyAudienceSpacesEmpty,
            style: TextStyle(color: VoiceColors.of(context).textDisabled),
          );
        }
        final allIds = data.spaces.map((s) => s.id).toList();
        return Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            for (final space in data.spaces)
              FilterChip(
                label: Text(space.name),
                selected: _isSpaceSelected(space.id, allIds),
                onSelected: (_) => _toggleSpace(space.id, allIds),
              ),
          ],
        );
      },
    );
  }
}
