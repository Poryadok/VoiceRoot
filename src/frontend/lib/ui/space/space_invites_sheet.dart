import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/space_providers.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_list_row.dart';
import '../core/voice_state_panel.dart';

/// Bottom sheet: create, list, copy, and revoke space invite links.
class SpaceInvitesSheet extends ConsumerStatefulWidget {
  const SpaceInvitesSheet({super.key, required this.spaceId});

  static const Key sheetKey = Key('space_invites_sheet');
  static const Key createButtonKey = Key('space_invites_create');
  static const Key maxUsesFieldKey = Key('space_invites_max_uses');

  final String spaceId;

  static Future<void> show(BuildContext context, {required String spaceId}) {
    final container = ProviderScope.containerOf(context);
    return showVoiceBottomSheet<void>(
      context: context,
      scrollable: true,
      child: UncontrolledProviderScope(
        container: container,
        child: SpaceInvitesSheet(spaceId: spaceId),
      ),
    );
  }

  @override
  ConsumerState<SpaceInvitesSheet> createState() => _SpaceInvitesSheetState();
}

class _SpaceInvitesSheetState extends ConsumerState<SpaceInvitesSheet> {
  final _maxUsesController = TextEditingController();
  var _showAdvanced = false;
  var _creating = false;

  @override
  void dispose() {
    _maxUsesController.dispose();
    super.dispose();
  }

  Future<void> _createInvite() async {
    setState(() => _creating = true);
    final l10n = AppLocalizations.of(context)!;
    int? maxUses;
    final maxUsesText = _maxUsesController.text.trim();
    if (maxUsesText.isNotEmpty) {
      maxUses = int.tryParse(maxUsesText);
      if (maxUses == null || maxUses < 1) {
        if (mounted) {
          setState(() => _creating = false);
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text(l10n.spaceInviteMaxUsesInvalid)),
          );
        }
        return;
      }
    }

    final err = await ref.read(spaceInviteActionsProvider).createInvite(
      spaceId: widget.spaceId,
      maxUses: maxUses,
    );
    if (!mounted) return;
    setState(() => _creating = false);
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.spaceInviteCreateError(err))),
      );
    }
  }

  Future<void> _copyLink(String link) async {
    await Clipboard.setData(ClipboardData(text: link));
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(AppLocalizations.of(context)!.spaceInviteCopied)),
    );
  }

  Future<void> _revoke(String inviteId) async {
    final l10n = AppLocalizations.of(context)!;
    final err = await ref.read(spaceInviteActionsProvider).revokeInvite(
      spaceId: widget.spaceId,
      inviteId: inviteId,
    );
    if (!mounted) return;
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.spaceInviteRevokeError(err))),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final invitesAsync = ref.watch(spaceInvitesProvider(widget.spaceId));

    return SafeArea(
      key: SpaceInvitesSheet.sheetKey,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 8, 20, 20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(l10n.spaceInvitesTitle, style: theme.textTheme.titleLarge),
            const SizedBox(height: 8),
            Text(l10n.spaceInvitesSubtitle, style: theme.textTheme.bodyMedium),
            const SizedBox(height: 16),
            if (_showAdvanced)
              TextField(
                key: SpaceInvitesSheet.maxUsesFieldKey,
                controller: _maxUsesController,
                keyboardType: TextInputType.number,
                decoration: InputDecoration(
                  labelText: l10n.spaceInviteMaxUsesLabel,
                  hintText: l10n.spaceInviteMaxUsesHint,
                ),
              ),
            if (_showAdvanced) const SizedBox(height: 8),
            Row(
              children: [
                TextButton(
                  onPressed: () => setState(() => _showAdvanced = !_showAdvanced),
                  child: Text(l10n.spaceInviteAdvancedToggle),
                ),
                const Spacer(),
                FilledButton(
                  key: SpaceInvitesSheet.createButtonKey,
                  onPressed: _creating ? null : _createInvite,
                  child: _creating
                      ? const SizedBox(
                          width: 18,
                          height: 18,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : Text(l10n.spaceInviteCreate),
                ),
              ],
            ),
            const SizedBox(height: 16),
            invitesAsync.when(
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (e, _) => VoiceStatePanel(
                title: l10n.spaceInvitesLoadError,
                message: e.toString(),
                actionLabel: l10n.spaceInvitesRetry,
                onAction: () =>
                    ref.invalidate(spaceInvitesProvider(widget.spaceId)),
              ),
              data: (invites) {
                if (invites.isEmpty) {
                  return VoiceStatePanel(title: l10n.spaceInvitesEmpty);
                }
                return Column(
                  children: [
                    for (final invite in invites)
                      VoiceListRow(
                        key: Key('space_invite_${invite.id}'),
                        title: invite.code,
                        subtitle: l10n.spaceInviteUses(
                          invite.useCount,
                          invite.maxUses != null ? ' / ${invite.maxUses}' : '',
                        ),
                        trailing: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            IconButton(
                              key: Key('copy_invite_${invite.id}'),
                              icon: const Icon(Icons.link),
                              tooltip: l10n.spaceInviteCopy,
                              onPressed: () => _copyLink(invite.inviteLink),
                            ),
                            IconButton(
                              key: Key('revoke_invite_${invite.id}'),
                              icon: const Icon(Icons.delete_outline),
                              tooltip: l10n.spaceInviteRevoke,
                              onPressed: () => _revoke(invite.id),
                            ),
                          ],
                        ),
                      ),
                  ],
                );
              },
            ),
          ],
        ),
      ),
    );
  }
}
