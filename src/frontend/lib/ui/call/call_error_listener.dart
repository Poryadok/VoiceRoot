import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/call_providers.dart';
import '../../state/social_providers.dart';
import '../privacy/privacy_action_errors.dart';

class CallErrorListener extends ConsumerWidget {
  const CallErrorListener({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    ref.listen(callControllerProvider, (prev, next) {
      if (next.errorMessage == 'voice_session_conflict') {
        if (prev?.errorMessage == next.errorMessage) return;
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (!context.mounted) return;
          _showVoiceConflictDialog(context, ref);
        });
        return;
      }

      if (next.phase != CallPhase.failed || next.errorMessage == null) {
        return;
      }
      if (prev?.phase == CallPhase.failed &&
          prev?.errorMessage == next.errorMessage) {
        return;
      }
      final message = next.errorMessage!;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!context.mounted) return;
        final l10n = AppLocalizations.of(context);
        if (l10n == null) return;
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            key: const Key('call_error_snackbar'),
            content: Text(_callErrorMessage(l10n, message)),
          ),
        );
        ref.read(callControllerProvider.notifier).dismissFailure();
      });
    });
    return child;
  }

  Future<void> _showVoiceConflictDialog(
    BuildContext context,
    WidgetRef ref,
  ) async {
    final l10n = AppLocalizations.of(context);
    if (l10n == null) return;

    final bindingId =
        ref.read(callControllerProvider).voiceBindingProfileId ??
        ref.read(authControllerProvider).activeProfileId;
    String profileName = l10n.callActive;
    if (bindingId != null) {
      final profile = await ref.read(profileProvider(bindingId).future);
      if (profile != null && profile.displayName.isNotEmpty) {
        profileName = profile.displayName;
      }
    }

    if (!context.mounted) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        key: const Key('voice_leave_current_dialog'),
        title: Text(l10n.voiceLeaveCurrentDialogTitle),
        content: Text(l10n.voiceLeaveCurrentDialogMessage(profileName)),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(MaterialLocalizations.of(ctx).cancelButtonLabel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(l10n.voiceLeaveCurrentDialogConfirm),
          ),
        ],
      ),
    );

    if (!context.mounted) return;
    if (confirmed == true) {
      await ref.read(callControllerProvider.notifier).confirmLeaveCurrentVoiceAndRetry();
    } else {
      ref.read(callControllerProvider.notifier).dismissVoiceConflict();
    }
  }

  String _callErrorMessage(AppLocalizations l10n, String errorMessage) {
    final privacy = privacyActionErrorMessage(l10n, errorMessage);
    if (privacy != errorMessage) return privacy;
    return switch (errorMessage) {
      'livekit_connect_failed' => l10n.callLivekitConnectFailed,
      'livekit_url_missing' => l10n.callLivekitConnectFailed,
      'profile already has active call' => l10n.callActiveCallExists,
      _ => l10n.callFailed(errorMessage),
    };
  }
}
