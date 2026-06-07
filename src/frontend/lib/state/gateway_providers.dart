import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:http/http.dart' as http;

import '../backend/gateway_client.dart';
import '../backend/gateway_config.dart';
final gatewayConfigProvider = Provider<GatewayConfig>((ref) {
  return GatewayConfig.fromEnvironment();
});

final httpClientProvider = Provider<http.Client>((ref) {
  final client = http.Client();
  ref.onDispose(client.close);
  return client;
});

final voiceGatewayClientProvider = Provider<VoiceGatewayClient>((ref) {
  return VoiceGatewayClient(
    httpClient: ref.watch(httpClientProvider),
    config: ref.watch(gatewayConfigProvider),
  );
});

final gatewayHealthProvider = FutureProvider<GatewayHealthResult>((ref) {
  return ref.watch(voiceGatewayClientProvider).fetchHealth();
});
