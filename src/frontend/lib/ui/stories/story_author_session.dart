import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';

/// Pins a Story form to its opening account/profile installation, allowing
/// ordinary access-token refresh but never a replacement login or profile.
class StoryAuthorSession {
  StoryAuthorSession._(this.profileId, this._matches);
  final String? profileId;
  final bool Function(WidgetRef) _matches;

  factory StoryAuthorSession.capture(WidgetRef ref) {
    final controller = ref.read(authControllerProvider.notifier);
    final identity = controller.gatewayRequestIdentity;
    final install = controller.sessionInstallGeneration;
    return StoryAuthorSession._(identity?.profileId, (ref) {
      final current = ref.read(authControllerProvider.notifier);
      return identity != null &&
          identical(current, controller) &&
          current.gatewayRequestIdentity == identity &&
          current.sessionInstallGeneration == install;
    });
  }

  bool isCurrent(WidgetRef ref) => _matches(ref);
}

class StorySessionExpiredPanel extends StatelessWidget {
  const StorySessionExpiredPanel({super.key});
  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(l10n.storySessionChanged),
            TextButton(
              onPressed: () => Navigator.of(context).pop(),
              child: Text(l10n.commonCancel),
            ),
          ],
        ),
      ),
    );
  }
}
