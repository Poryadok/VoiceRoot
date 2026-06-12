import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/gateway_http.dart';
import '../backend/search_client.dart';
import 'auth_providers.dart';
import 'gateway_providers.dart' show gatewayConfigProvider, httpClientProvider;

final voiceSearchClientProvider = Provider<VoiceSearchClient>((ref) {
  return VoiceSearchClient(
    gateway: GatewayHttpClient(
      httpClient: ref.watch(httpClientProvider),
      config: ref.watch(gatewayConfigProvider),
      authorizationProvider: () =>
          ref.read(authControllerProvider).session?.authorizationHeader,
    ),
  );
});
