import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/shell_providers.dart';
import '../../state/space_providers.dart';
import '../core/voice_bottom_sheet.dart';

/// Bottom sheet: paste invite code and join a space.
class JoinSpaceInviteSheet extends ConsumerStatefulWidget {
  const JoinSpaceInviteSheet({super.key, this.initialCode});

  static const Key sheetKey = Key('join_space_invite_sheet');
  static const Key codeFieldKey = Key('join_space_invite_code');
  static const Key submitKey = Key('join_space_invite_submit');

  final String? initialCode;

  static Future<void> show(BuildContext context, {String? code}) {
    final container = ProviderScope.containerOf(context);
    return showVoiceBottomSheet<void>(
      context: context,
      scrollable: false,
      child: UncontrolledProviderScope(
        container: container,
        child: JoinSpaceInviteSheet(initialCode: code),
      ),
    );
  }

  @override
  ConsumerState<JoinSpaceInviteSheet> createState() =>
      _JoinSpaceInviteSheetState();
}

class _JoinSpaceInviteSheetState extends ConsumerState<JoinSpaceInviteSheet> {
  late final TextEditingController _codeController;
  var _submitting = false;

  @override
  void initState() {
    super.initState();
    _codeController = TextEditingController(text: widget.initialCode ?? '');
  }

  @override
  void dispose() {
    _codeController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final code = _codeController.text.trim();
    if (code.isEmpty || _submitting) return;

    setState(() => _submitting = true);
    final l10n = AppLocalizations.of(context)!;
    final joinResult = await ref.read(spaceInviteActionsProvider).joinByInvite(
      code: code,
    );
    if (!mounted) return;
    setState(() => _submitting = false);

    if (joinResult.error != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.spaceInviteJoinError(joinResult.error!))),
      );
      return;
    }

    final spaceId = joinResult.spaceId;
    if (!mounted) return;
    Navigator.of(context).pop();
    if (spaceId != null) {
      ref.read(shellNavigationProvider).selectSpace(spaceId);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);

    return SafeArea(
      key: JoinSpaceInviteSheet.sheetKey,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 8, 20, 20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(l10n.spaceInviteJoinTitle, style: theme.textTheme.titleLarge),
            const SizedBox(height: 8),
            Text(l10n.spaceInviteJoinSubtitle, style: theme.textTheme.bodyMedium),
            const SizedBox(height: 16),
            TextField(
              key: JoinSpaceInviteSheet.codeFieldKey,
              controller: _codeController,
              decoration: InputDecoration(
                labelText: l10n.spaceInviteJoinCodeLabel,
                hintText: l10n.spaceInviteJoinCodeHint,
              ),
              onSubmitted: (_) => _submit(),
            ),
            const SizedBox(height: 16),
            FilledButton(
              key: JoinSpaceInviteSheet.submitKey,
              onPressed: _submitting ? null : _submit,
              child: _submitting
                  ? const SizedBox(
                      width: 18,
                      height: 18,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Text(l10n.spaceInviteJoinSubmit),
            ),
          ],
        ),
      ),
    );
  }
}
