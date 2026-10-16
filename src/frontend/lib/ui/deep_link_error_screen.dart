import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../l10n/app_localizations.dart';
import '../routing/app_router.dart';
import '../theme/voice_colors.dart';

/// Safe recovery screen for deep-link targets the resolver denies or cannot find.
class DeepLinkErrorScreen extends StatelessWidget {
  const DeepLinkErrorScreen({required this.statusCode, super.key});

  final int statusCode;

  @override
  Widget build(BuildContext context) {
    final localizations = AppLocalizations.of(context)!;
    final message = switch (statusCode) {
      403 => localizations.deepLinkAccessDenied,
      404 => localizations.deepLinkResourceNotFound,
      _ => throw ArgumentError.value(statusCode, 'statusCode'),
    };
    final description = statusCode == 403
        ? localizations.deepLinkAccessDeniedDescription
        : localizations.deepLinkResourceNotFoundDescription;
    final theme = Theme.of(context);
    final colors = VoiceColors.of(context);

    return Scaffold(
      key: const ValueKey('deep-link-error-screen'),
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: SizedBox(
              width: 352,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    '$statusCode',
                    textAlign: TextAlign.center,
                    style: theme.textTheme.displayLarge?.copyWith(
                      fontSize: 56,
                      height: 64 / 56,
                      fontWeight: FontWeight.w600,
                      color: colors.profileAccent,
                    ),
                  ),
                  const SizedBox(height: 10),
                  Text(
                    message,
                    textAlign: TextAlign.center,
                    style: theme.textTheme.titleLarge?.copyWith(
                      fontSize: 20,
                      height: 28 / 20,
                    ),
                  ),
                  const SizedBox(height: 4),
                  ConstrainedBox(
                    constraints: const BoxConstraints(maxWidth: 330),
                    child: Text(
                      description,
                      key: const ValueKey('deep-link-error-description'),
                      textAlign: TextAlign.center,
                      style: theme.textTheme.bodyMedium?.copyWith(
                        color: colors.textSecondary,
                      ),
                    ),
                  ),
                  const SizedBox(height: 18),
                  FilledButton(
                    key: const ValueKey('deep-link-error-home'),
                    onPressed: () {
                      final router = GoRouter.of(context);
                      Navigator.of(context).pop();
                      router.go(VoiceAppRoutes.home);
                    },
                    child: Text(localizations.deepLinkReturnHome),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
