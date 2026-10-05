import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../l10n/app_localizations.dart';
import '../routing/app_router.dart';

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

    return Scaffold(
      key: const ValueKey('deep-link-error-screen'),
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  message,
                  textAlign: TextAlign.center,
                  style: Theme.of(context).textTheme.headlineSmall,
                ),
                const SizedBox(height: 24),
                FilledButton.icon(
                  key: const ValueKey('deep-link-error-home'),
                  onPressed: () {
                    final router = GoRouter.of(context);
                    Navigator.of(context).pop();
                    router.go(VoiceAppRoutes.home);
                  },
                  icon: const Icon(Icons.home_outlined),
                  label: Text(localizations.deepLinkReturnHome),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
