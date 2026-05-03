/// API Gateway base URL (scheme + host + optional port), no trailing path.
/// Set at compile time: `--dart-define=VOICE_API_BASE_URL=https://voice.example`.
class GatewayConfig {
  const GatewayConfig({required this.baseUrl});

  /// Empty string when not defined (tests and local runs inject via [Provider]).
  factory GatewayConfig.fromEnvironment() {
    const url = String.fromEnvironment(
      'VOICE_API_BASE_URL',
      defaultValue: '',
    );
    return GatewayConfig(baseUrl: url);
  }

  final String baseUrl;

  bool get hasBaseUrl => baseUrl.isNotEmpty;
}
