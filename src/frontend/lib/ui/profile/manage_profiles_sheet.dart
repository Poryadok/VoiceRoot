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
import '../core/voice_bottom_sheet.dart';
import 'create_profile_sheet.dart';

class ManageProfilesSheet extends ConsumerStatefulWidget {
  const ManageProfilesSheet({super.key});

  static const Key sheetKey = Key('manage_profiles_sheet');
  static const Key addProfileKey = Key('manage_profiles_add');

  @override
  ConsumerState<ManageProfilesSheet> createState() => _ManageProfilesSheetState();
}

class _ManageProfilesSheetState extends ConsumerState<ManageProfilesSheet> {
  String? _error;
  String? _deletingProfileId;

  Future<void> _confirmDelete(VoiceProfile profile) async {
    final l10n = AppLocalizations.of(context)!;
    if (profile.isPrimary) {
      setState(() => _error = l10n.manageProfilesDeletePrimaryBlocked);
      return;
    }
    final activeId = ref.read(authControllerProvider).activeProfileId;
    if (activeId == profile.id) {
      setState(() => _error = l10n.manageProfilesDeleteActiveBlocked);
      return;
    }

    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l10n.manageProfilesDeleteConfirmTitle),
        content: Text(l10n.manageProfilesDeleteConfirmMessage),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(l10n.commonCancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(l10n.commonDelete),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;

    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;

    setState(() {
      _deletingProfileId = profile.id;
      _error = null;
    });

    final result = await ref.read(voiceUsersClientProvider).deleteProfile(
      authorization: auth,
      profileId: profile.id,
    );

    if (!mounted) return;
    switch (result) {
      case UsersApiOk():
        ref.invalidate(myProfilesProvider);
        setState(() => _deletingProfileId = null);
      case UsersApiFailure(:final message):
        setState(() {
          _deletingProfileId = null;
          _error = message;
        });
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final profilesAsync = ref.watch(myProfilesProvider);

    return SafeArea(
      child: Padding(
        key: ManageProfilesSheet.sheetKey,
        padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
        child: profilesAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (_, _) => Text(l10n.backendUnavailable),
          data: (profiles) {
            return Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  l10n.manageProfilesTitle,
                  style: Theme.of(context).textTheme.titleLarge,
                ),
                const SizedBox(height: 16),
                for (final profile in profiles)
                  ListTile(
                    key: Key('manage_profiles_row_${profile.id}'),
                    contentPadding: EdgeInsets.zero,
                    leading: _ProfileAccentFor(profile),
                    title: Text(profile.displayName),
                    subtitle: Text(
                      profile.isPrimary
                          ? '${profile.handle} · ${l10n.manageProfilesPrimaryBadge}'
                          : profile.handle,
                    ),
                    trailing: profile.isPrimary
                        ? null
                        : IconButton(
                            key: Key('manage_profiles_delete_${profile.id}'),
                            tooltip: l10n.manageProfilesDeleteAction,
                            onPressed: _deletingProfileId == profile.id
                                ? null
                                : () => _confirmDelete(profile),
                            icon: _deletingProfileId == profile.id
                                ? const SizedBox(
                                    width: 18,
                                    height: 18,
                                    child: CircularProgressIndicator(strokeWidth: 2),
                                  )
                                : Icon(Icons.delete_outline, color: voice.error),
                          ),
                  ),
                if (_error != null) ...[
                  const SizedBox(height: 8),
                  Text(_error!, style: TextStyle(color: voice.error)),
                ],
                const SizedBox(height: 12),
                OutlinedButton.icon(
                  key: ManageProfilesSheet.addProfileKey,
                  onPressed: () async {
                    Navigator.of(context).pop();
                    await showCreateProfileSheet(context);
                  },
                  icon: const Icon(Icons.add),
                  label: Text(l10n.createProfileAddAction),
                ),
              ],
            );
          },
        ),
      ),
    );
  }
}

Future<void> showManageProfilesSheet(BuildContext context) {
  return showVoiceBottomSheet<void>(
    context: context,
    child: const ManageProfilesSheet(),
  );
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
