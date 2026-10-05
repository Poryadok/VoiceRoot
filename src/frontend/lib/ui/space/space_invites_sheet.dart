import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:qr_flutter/qr_flutter.dart';

import '../../backend/space_permissions.dart';
import '../../backend/spaces_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
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

  var _showAdvanced = false;
  var _creating = false;
  var _expiry = _InviteExpiry.never;
  int? _maxUses;

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

  _InviteActionScope _captureActionScope() {
    final authorization = ref.read(authorizationHeaderProvider);
    final auth = ref.read(authControllerProvider);
    return (
      session: auth.session,
      authorization: authorization,
      accountId: auth.session?.accountId,
      profileId: auth.activeProfileId,
      spaceId: widget.spaceId,
    );
  }

  bool _isCurrentActionScope(_InviteActionScope expected) {
    if (!mounted || widget.spaceId != expected.spaceId) return false;
    if (ref.read(authorizationHeaderProvider) != expected.authorization) {
      return false;
    }
    final auth = ref.read(authControllerProvider);
    return identical(auth.session, expected.session) &&
        auth.session?.accountId == expected.accountId &&
        auth.activeProfileId == expected.profileId;
  }

  Future<bool?> _refreshInvitePermission(_InviteActionScope expected) async {
    try {
      return await ref.refresh(
        spacePermissionProvider((
          spaceId: expected.spaceId,
          permission: SpacePermissions.spaceManageInvites,
          chatId: null,
          voiceRoomId: null,
        )).future,
      );
    } catch (_) {
      return null;
    }
  }

  List<SpaceInvite> _activeInvites(List<SpaceInvite> invites) {
    final now = DateTime.now().toUtc();
    return invites
        .where((invite) {
          final expiry = invite.expiresAt?.toUtc();
          final expired = expiry != null && !now.isBefore(expiry);
          final exhausted =
              invite.maxUses != null && invite.useCount >= invite.maxUses!;
          return invite.spaceId == widget.spaceId &&
              invite.revokedAt == null &&
              !expired &&
              !exhausted;
        })
        .toList(growable: false);
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

  Future<void> _createInvite() async {
    final actionScope = _captureActionScope();
    setState(() => _creating = true);
    final l10n = AppLocalizations.of(context)!;
    final canCreate = await _refreshInvitePermission(actionScope);
    if (!mounted) return;
    if (!_isCurrentActionScope(actionScope)) {
      setState(() => _creating = false);
      return;
    }
    if (canCreate != true) {
      setState(() => _creating = false);
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
          spaceId: actionScope.spaceId,
          maxUses: _maxUses,
          expiresAt: _expiresAt,
        );
    if (!mounted) return;
    if (!_isCurrentActionScope(actionScope)) {
      setState(() => _creating = false);
      return;
    }
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

  Future<void> _revoke(String inviteId, _InviteActionScope actionScope) async {
    if (!_isCurrentActionScope(actionScope)) return;
    final l10n = AppLocalizations.of(context)!;
    final canRevoke = await _refreshInvitePermission(actionScope);
    if (!mounted || !_isCurrentActionScope(actionScope)) return;
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
        .revokeInvite(spaceId: actionScope.spaceId, inviteId: inviteId);
    if (!mounted || !_isCurrentActionScope(actionScope)) return;
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
    final actionScope = _captureActionScope();
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
    if (confirmed == true && _isCurrentActionScope(actionScope)) {
      await _revoke(inviteId, actionScope);
    }
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
              DropdownButtonFormField<int?>(
                key: SpaceInvitesSheet.maxUsesFieldKey,
                decoration: InputDecoration(
                  labelText: l10n.spaceInviteMaxUsesLabel,
                ),
                initialValue: _maxUses,
                items: [
                  for (final option in _InviteMaxUses.values)
                    DropdownMenuItem<int?>(
                      key: Key('space_invite_max_uses_choice_${option.key}'),
                      value: option.value,
                      child: Text(
                        option.value?.toString() ?? l10n.spaceInviteMaxUsesHint,
                      ),
                    ),
                ],
                onChanged: (value) => setState(() => _maxUses = value),
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
                final activeInvites = _activeInvites(invites);
                if (activeInvites.isEmpty) {
                  return VoiceStatePanel(title: l10n.spaceInvitesEmpty);
                }
                return Column(
                  children: [
                    for (final invite in activeInvites)
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

enum _InviteMaxUses {
  one(1),
  five(5),
  ten(10),
  twentyFive(25),
  fifty(50),
  hundred(100),
  unlimited(null);

  const _InviteMaxUses(this.value);

  final int? value;

  String get key => value?.toString() ?? 'unlimited';
}

typedef _InviteActionScope = ({
  Object? session,
  String? authorization,
  String? accountId,
  String? profileId,
  String spaceId,
});
