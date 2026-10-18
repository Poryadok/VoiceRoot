import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../backend/gateway_http.dart';
import '../../state/space_leave.dart';
import '../../state/space_providers.dart';
import '../core/voice_bottom_sheet.dart';

class LeaveSpaceSheet extends ConsumerStatefulWidget {
  const LeaveSpaceSheet({super.key, required this.spaceId});
  final String spaceId;
  static const confirmKey = Key('space_leave_confirm');
  static const ownerHintKey = Key('space_leave_owner_hint');
  static const errorKey = Key('space_leave_error');

  static Future<void> show(BuildContext context, {required String spaceId}) {
    final container = ProviderScope.containerOf(context);
    return showVoiceBottomSheet<void>(
      context: context,
      scrollable: true,
      child: UncontrolledProviderScope(
        container: container,
        child: LeaveSpaceSheet(spaceId: spaceId),
      ),
    );
  }

  @override
  ConsumerState<LeaveSpaceSheet> createState() => _LeaveSpaceSheetState();
}

class _LeaveSpaceSheetState extends ConsumerState<LeaveSpaceSheet> {
  bool _busy = false;
  SpaceLeaveResult? _result;
  GatewayRequestIdentity? _identity;

  @override
  void initState() {
    super.initState();
    _identity = ref.read(spaceLeaveIdentityProvider)();
  }

  Future<void> _leave() async {
    if (_busy || _identity != ref.read(spaceLeaveIdentityProvider)()) return;
    setState(() {
      _busy = true;
      _result = null;
    });
    final result = await ref
        .read(spaceLeaveActionsProvider)
        .leave(widget.spaceId);
    if (!mounted) return;
    if (result == SpaceLeaveResult.success &&
        ModalRoute.of(context)?.isCurrent == true) {
      Navigator.of(context).pop();
      return;
    }
    setState(() {
      _busy = false;
      _result = result;
    });
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final space = ref.watch(spaceProvider(widget.spaceId)).valueOrNull;
    final profileId = ref.watch(spaceViewerProfileIdProvider);
    final sameActor =
        _identity != null &&
        _identity == ref.read(spaceLeaveIdentityProvider)();
    final owner = space != null && space.ownerProfileId == profileId;
    return PopScope(
      canPop: !_busy,
      child: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                l10n.spaceLeaveTitle,
                style: Theme.of(context).textTheme.titleLarge,
              ),
              const SizedBox(height: 16),
              Text(l10n.spaceLeaveMessage),
              if (owner)
                Padding(
                  padding: const EdgeInsets.only(top: 12),
                  child: Text(
                    l10n.spaceLeaveOwnerHint,
                    key: LeaveSpaceSheet.ownerHintKey,
                  ),
                ),
              if (_result != null)
                Padding(
                  padding: const EdgeInsets.only(top: 12),
                  child: Text(
                    _result == SpaceLeaveResult.uncertain
                        ? l10n.spaceLeaveUncertain
                        : l10n.commonActionFailed,
                    key: LeaveSpaceSheet.errorKey,
                  ),
                ),
              const SizedBox(height: 24),
              FilledButton(
                key: LeaveSpaceSheet.confirmKey,
                onPressed:
                    _busy ||
                        !sameActor ||
                        owner ||
                        space == null ||
                        profileId == null ||
                        _result == SpaceLeaveResult.uncertain
                    ? null
                    : _leave,
                child: _busy
                    ? const SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : Text(l10n.spaceLeaveAction),
              ),
              TextButton(
                onPressed: _busy ? null : () => Navigator.of(context).pop(),
                child: Text(l10n.commonCancel),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
