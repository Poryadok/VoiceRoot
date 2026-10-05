import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:qr_flutter/qr_flutter.dart';

import '../../backend/space_permissions.dart';
import '../../backend/spaces_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/space_providers.dart';
import '../api_error_messages.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_disabled_action.dart';
import '../core/voice_list_row.dart';
import '../core/voice_skeleton.dart';
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
  static const _expiryFieldKey = Key('space_invite_expiry_field');

  final _maxUsesController = TextEditingController();
  var _showAdvanced = false;
  var _creating = false;
  var _expiry = _InviteExpiry.never;

  DateTime? get _expiresAt {
    final duration = _expiry.duration;
    return duration == null ? null : DateTime.now().toUtc().add(duration);
  }

  String _expiryLabel(AppLocalizations l10n, _InviteExpiry expiry) =>
      switch (expiry) {
        _InviteExpiry.minutes30 => l10n.spaceInviteExpiry30Minutes,
        _InviteExpiry.hour1 => l10n.spaceInviteExpiry1Hour,
        _InviteExpiry.hours6 => l10n.spaceInviteExpiry6Hours,
        _InviteExpiry.hours12 => l10n.spaceInviteExpiry12Hours,
        _InviteExpiry.day1 => l10n.spaceInviteExpiry1Day,
        _InviteExpiry.days7 => l10n.spaceInviteExpiry7Days,
        _InviteExpiry.never => l10n.spaceInviteExpiryNever,
      };

  SpacePermissionQuery get _invitePermissionQuery => (
    spaceId: widget.spaceId,
    permission: SpacePermissions.spaceManageInvites,
    chatId: null,
    voiceRoomId: null,
  );

  Future<bool?> _refreshInvitePermission() async {
    try {
      return await ref.refresh(
        spacePermissionProvider(_invitePermissionQuery).future,
      );
    } catch (_) {
      return null;
    }
  }

  void _showPermissionDenied() {
    if (!mounted) return;
    final l10n = AppLocalizations.of(context)!;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(
          spacePermissionDeniedReason(
            l10n,
            SpacePermissions.spaceManageInvites,
          ),
        ),
      ),
    );
  }

  void _showPermissionUnavailable() {
    if (!mounted) return;
    final l10n = AppLocalizations.of(context)!;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(commonActionErrorMessage(l10n))));
  }

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

    final canCreate = await _refreshInvitePermission();
    if (canCreate != true) {
      if (mounted) setState(() => _creating = false);
      if (canCreate == false) {
        _showPermissionDenied();
      } else {
        _showPermissionUnavailable();
      }
      return;
    }

    final err = await ref
        .read(spaceInviteActionsProvider)
        .createInvite(
          spaceId: widget.spaceId,
          maxUses: maxUses,
          expiresAt: _expiresAt,
        );
    if (!mounted) return;
    setState(() => _creating = false);
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            err == 'not_authenticated'
                ? l10n.spaceInviteCreateError(err)
                : commonActionErrorMessage(l10n),
          ),
        ),
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
    final canRevoke = await _refreshInvitePermission();
    if (canRevoke != true) {
      if (canRevoke == false) {
        _showPermissionDenied();
      } else {
        _showPermissionUnavailable();
      }
      return;
    }
    final err = await ref
        .read(spaceInviteActionsProvider)
        .revokeInvite(spaceId: widget.spaceId, inviteId: inviteId);
    if (!mounted) return;
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            err == 'not_authenticated'
                ? l10n.spaceInviteRevokeError(err)
                : commonActionErrorMessage(l10n),
          ),
        ),
      );
    }
  }

  Future<void> _confirmRevoke(String inviteId) async {
    final l10n = AppLocalizations.of(context)!;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(l10n.spaceInviteRevokeConfirmTitle),
        content: Text(l10n.spaceInviteRevokeConfirmBody),
        actions: [
          TextButton(
            key: const Key('space_invite_revoke_cancel'),
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: Text(l10n.commonCancel),
          ),
          TextButton(
            key: const Key('space_invite_revoke_confirm'),
            onPressed: () => Navigator.of(dialogContext).pop(true),
            child: Text(l10n.spaceInviteRevoke),
          ),
        ],
      ),
    );
    if (confirmed == true && mounted) await _revoke(inviteId);
  }

  Future<void> _showQr(String link) => showDialog<void>(
    context: context,
    builder: (dialogContext) {
      final l10n = AppLocalizations.of(dialogContext)!;
      return Dialog(
        child: SizedBox(
          width: 260,
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(l10n.spaceInviteQrTitle),
                const SizedBox(height: 16),
                Semantics(
                  image: true,
                  label: l10n.spaceInviteQrSemanticLabel,
                  child: SizedBox(
                    width: 200,
                    height: 200,
                    child: QrImageView(
                      key: const Key('space_invite_qr_image'),
                      data: link,
                      size: 200,
                      backgroundColor: Colors.white,
                      semanticsLabel: l10n.spaceInviteQrSemanticLabel,
                    ),
                  ),
                ),
                const SizedBox(height: 12),
                SelectableText(link),
                const SizedBox(height: 12),
                TextButton(
                  key: const Key('space_invite_qr_close'),
                  onPressed: () => Navigator.of(dialogContext).pop(),
                  child: Text(l10n.commonCancel),
                ),
              ],
            ),
          ),
        ),
      );
    },
  );

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final permissionAsync = ref.watch(
      spacePermissionProvider(_invitePermissionQuery),
    );

    if (permissionAsync.isLoading) {
      return const SafeArea(child: VoiceListSkeleton(rowCount: 3));
    }
    if (permissionAsync.hasError) {
      return SafeArea(
        child: VoiceStatePanel(
          title: l10n.spaceInvitesLoadError,
          message: commonActionErrorMessage(l10n),
          actionLabel: l10n.spaceInvitesRetry,
          onAction: () =>
              ref.invalidate(spacePermissionProvider(_invitePermissionQuery)),
        ),
      );
    }
    if (permissionAsync.valueOrNull != true) {
      return SafeArea(
        child: VoiceStatePanel(
          title: l10n.spaceInvitesTitle,
          message: spacePermissionDeniedReason(
            l10n,
            SpacePermissions.spaceManageInvites,
          ),
          icon: Icons.lock_outlined,
        ),
      );
    }

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
            PopupMenuButton<_InviteExpiry>(
              key: _expiryFieldKey,
              tooltip: l10n.spaceInviteExpiryLabel,
              onSelected: (value) => setState(() => _expiry = value),
              itemBuilder: (context) => [
                for (final expiry in _InviteExpiry.values)
                  PopupMenuItem(
                    key: Key('space_invite_expiry_choice_${expiry.key}'),
                    value: expiry,
                    child: Text(_expiryLabel(l10n, expiry)),
                  ),
              ],
              child: InputDecorator(
                decoration: InputDecoration(
                  labelText: l10n.spaceInviteExpiryLabel,
                  border: const OutlineInputBorder(),
                ),
                child: Row(
                  children: [
                    Expanded(child: Text(_expiryLabel(l10n, _expiry))),
                    const Icon(Icons.arrow_drop_down),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 8),
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
                  onPressed: () =>
                      setState(() => _showAdvanced = !_showAdvanced),
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
              loading: () => const VoiceListSkeleton(rowCount: 4),
              error: (e, _) => VoiceStatePanel(
                title: l10n.spaceInvitesLoadError,
                message: spaceInvitesErrorMessage(l10n, e),
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
                        subtitle: _inviteSubtitle(l10n, invite),
                        trailing: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            IconButton(
                              key: Key('qr_invite_${invite.id}'),
                              icon: const Icon(Icons.qr_code_2),
                              tooltip: l10n.spaceInviteShowQr,
                              onPressed: () => _showQr(invite.inviteLink),
                            ),
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
                              onPressed: () => _confirmRevoke(invite.id),
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

  String _inviteSubtitle(AppLocalizations l10n, SpaceInvite invite) {
    final uses = l10n.spaceInviteUses(
      invite.useCount,
      invite.maxUses != null ? ' / ${invite.maxUses}' : '',
    );
    final expiry = invite.expiresAt;
    if (expiry == null) return uses;
    final localExpiry = expiry.toLocal();
    final material = MaterialLocalizations.of(context);
    final date = material.formatMediumDate(localExpiry);
    final time = material.formatTimeOfDay(TimeOfDay.fromDateTime(localExpiry));
    return '$uses · ${l10n.spaceInviteExpiresAt('$date $time')}';
  }
}

enum _InviteExpiry {
  minutes30('30m', Duration(minutes: 30)),
  hour1('1h', Duration(hours: 1)),
  hours6('6h', Duration(hours: 6)),
  hours12('12h', Duration(hours: 12)),
  day1('1d', Duration(days: 1)),
  days7('7d', Duration(days: 7)),
  never('never', null);

  const _InviteExpiry(this.key, this.duration);

  final String key;
  final Duration? duration;
}
