import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';

GatewayHttpClient gatewayHttpForTest(
  http.Client httpClient, {
  GatewayConfig config = const GatewayConfig(baseUrl: 'http://api.test'),
}) {
  return GatewayHttpClient(httpClient: httpClient, config: config);
}
