import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/moderation_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/trust_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_primary_button.dart';

/// Submit a moderation sanction appeal (docs/features/reports.md § Апелляция).
Future<void> showAppealSheet(BuildContext context) {
  return showVoiceBottomSheet<void>(
    context: context,
    child: const AppealSheet(),
  );
}

class AppealSheet extends ConsumerStatefulWidget {
  const AppealSheet({super.key});

  static const Key sheetKey = Key('appeal_sheet');
  static const Key sanctionFieldKey = Key('appeal_sanction_id');
  static const Key reasonFieldKey = Key('appeal_reason');
  static const Key submitButtonKey = Key('appeal_submit');

  @override
  ConsumerState<AppealSheet> createState() => _AppealSheetState();
}

class _AppealSheetState extends ConsumerState<AppealSheet> {
  final _sanctionController = TextEditingController();
  final _reasonController = TextEditingController();
  var _busy = false;
  String? _error;
  String? _success;

  @override
  void dispose() {
    _sanctionController.dispose();
    _reasonController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final session = ref.read(authControllerProvider).session;
    if (session == null) return;

    final sanctionId = _sanctionController.text.trim();
    final reason = _reasonController.text.trim();
    if (sanctionId.isEmpty || reason.isEmpty) {
      setState(() => _error = AppLocalizations.of(context)!.appealValidationError);
      return;
    }

    setState(() {
      _busy = true;
      _error = null;
      _success = null;
    });

    final result = await ref.read(voiceModerationClientProvider).submitAppeal(
      authorization: session.authorizationHeader,
      sanctionId: sanctionId,
      reason: reason,
    );

    if (!mounted) return;
    switch (result) {
      case ModerationApiOk():
        setState(() {
          _busy = false;
          _success = AppLocalizations.of(context)!.appealSubmittedMessage;
        });
      case ModerationApiFailure(:final message):
        setState(() {
          _busy = false;
          _error = message;
        });
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);

    return SafeArea(
      child: Padding(
        key: AppealSheet.sheetKey,
        padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(l10n.appealTitle, style: Theme.of(context).textTheme.titleLarge),
            const SizedBox(height: 8),
            Text(
              l10n.appealDescription,
              style: TextStyle(color: voice.textSecondary),
            ),
            const SizedBox(height: 16),
            TextField(
              key: AppealSheet.sanctionFieldKey,
              controller: _sanctionController,
              enabled: !_busy && _success == null,
              decoration: InputDecoration(
                labelText: l10n.appealSanctionIdLabel,
                border: const OutlineInputBorder(),
              ),
            ),
            const SizedBox(height: 12),
            TextField(
              key: AppealSheet.reasonFieldKey,
              controller: _reasonController,
              enabled: !_busy && _success == null,
              minLines: 3,
              maxLines: 5,
              decoration: InputDecoration(
                labelText: l10n.appealReasonLabel,
                border: const OutlineInputBorder(),
              ),
            ),
            if (_error != null) ...[
              const SizedBox(height: 12),
              Text(_error!, style: TextStyle(color: voice.error)),
            ],
            if (_success != null) ...[
              const SizedBox(height: 12),
              Text(_success!, style: TextStyle(color: voice.success)),
            ],
            const SizedBox(height: 16),
            VoicePrimaryButton(
              key: AppealSheet.submitButtonKey,
              onPressed: _busy || _success != null ? null : _submit,
              isLoading: _busy,
              child: Text(l10n.appealSubmit),
            ),
          ],
        ),
      ),
    );
  }
}
