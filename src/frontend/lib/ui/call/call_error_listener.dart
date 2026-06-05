import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/call_providers.dart';

class CallErrorListener extends ConsumerWidget {
  const CallErrorListener({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    ref.listen(callControllerProvider, (prev, next) {
      if (next.phase != CallPhase.failed || next.errorMessage == null) {
        return;
      }
      if (prev?.phase == CallPhase.failed &&
          prev?.errorMessage == next.errorMessage) {
        return;
      }
      final l10n = AppLocalizations.of(context)!;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          key: const Key('call_error_snackbar'),
          content: Text(l10n.callFailed(next.errorMessage!)),
        ),
      );
      ref.read(callControllerProvider.notifier).dismissFailure();
    });
    return child;
  }
}
