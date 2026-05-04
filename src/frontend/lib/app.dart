import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'backend/gateway_client.dart';
import 'l10n/app_localizations.dart';
import 'shell/three_column_shell.dart';
import 'state/gateway_providers.dart';

class VoiceApp extends ConsumerWidget {
  const VoiceApp({super.key, this.locale});

  /// When non-null (e.g. in tests), forces [MaterialApp.locale] instead of the platform locale.
  final Locale? locale;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final health = ref.watch(gatewayHealthProvider);
    return MaterialApp(
      locale: locale,
      onGenerateTitle: (ctx) => AppLocalizations.of(ctx)!.appTitle,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: Colors.indigo),
        useMaterial3: true,
      ),
      home: Scaffold(
        body: SafeArea(
          child: ThreeColumnShell(
            header: _GatewayStatusBar(asyncHealth: health),
          ),
        ),
      ),
    );
  }
}

class _GatewayStatusBar extends StatelessWidget {
  const _GatewayStatusBar({required this.asyncHealth});

  final AsyncValue<GatewayHealthResult> asyncHealth;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final text = asyncHealth.when(
      data: (r) => switch (r) {
        GatewayHealthOk() => l10n.gatewayStatusOk,
        GatewayHealthFailure(:final message) => message == kGatewayMissingBaseUrlDetail
            ? l10n.gatewayMissingBaseUrl
            : l10n.gatewayStatusFailure(message),
      },
      loading: () => l10n.gatewayStatusChecking,
      error: (e, _) => l10n.gatewayStatusError(e.toString()),
    );
    return Material(
      elevation: 1,
      child: SizedBox(
        height: 40,
        width: double.infinity,
        child: Align(
          alignment: Alignment.centerLeft,
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12),
            child: Text(text, key: const Key('gateway_status_text')),
          ),
        ),
      ),
    );
  }
}
