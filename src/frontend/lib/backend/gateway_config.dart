/// API Gateway base URL (scheme + host + optional port), no trailing path.
/// Set at compile time: `--dart-define=VOICE_API_BASE_URL=https://voice.example`.
class GatewayConfig {
  const GatewayConfig({required this.baseUrl, this.livekitUrl = ''});

  /// Empty string when not defined (tests and local runs inject via [Provider]).
  factory GatewayConfig.fromEnvironment() {
    const url = String.fromEnvironment('VOICE_API_BASE_URL', defaultValue: '');
    const livekit = String.fromEnvironment(
      'VOICE_LIVEKIT_URL',
      defaultValue: '',
    );
    return GatewayConfig(baseUrl: url, livekitUrl: livekit);
  }

  final String baseUrl;
  final String livekitUrl;

  bool get hasBaseUrl => baseUrl.isNotEmpty;

  /// Compile-time LiveKit URL from [VOICE_LIVEKIT_URL].
  bool get hasLivekitUrl => livekitUrl.isNotEmpty;

  /// Voice calls are available when the gateway is configured; join token carries
  /// `livekit_url` and [effectiveLivekitFallback] covers local compose defaults.
  bool get canPlaceVoiceCalls => hasBaseUrl;

  /// Client-side LiveKit WS fallback when the API omits or returns a docker host.
  String get effectiveLivekitFallback {
    if (livekitUrl.isNotEmpty) return livekitUrl;
    final base = Uri.tryParse(baseUrl);
    if (base == null) return '';
    final host = base.host.toLowerCase();
    if (host == '127.0.0.1' || host == 'localhost') {
      return 'ws://127.0.0.1:7880';
    }
    return '';
  }
}
