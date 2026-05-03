import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'shell/three_column_shell.dart';
import 'state/gateway_providers.dart';
import 'backend/gateway_client.dart';

class VoiceApp extends ConsumerWidget {
  const VoiceApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final health = ref.watch(gatewayHealthProvider);
    return MaterialApp(
      title: 'Voice',
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
    final text = asyncHealth.when(
      data: (r) => switch (r) {
        GatewayHealthOk() => 'Gateway: ok',
        GatewayHealthFailure(:final message) => 'Gateway: $message',
      },
      loading: () => 'Gateway: checking…',
      error: (e, _) => 'Gateway: error ($e)',
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
