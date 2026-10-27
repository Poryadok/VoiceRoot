import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/web_gateway_config.dart';

void main() {
  Map<String, Object> document(String app, String api, String livekit) => {
    'version': 1,
    'app_origin': app,
    'api_base_url': api,
    'livekit_url': livekit,
  };

  final dev = document(
    'https://dev.comrade.click',
    'https://api-dev.comrade.click',
    'wss://livekit-dev.comrade.click',
  );
  final staging = document(
    'https://app.comrade.click',
    'https://voice.comrade.click',
    'wss://livekit.comrade.click',
  );

  test(
    'one compiled client uses each explicitly bound environment document',
    () async {
      for (final binding in [dev, staging]) {
        final origin = binding['app_origin']! as String;
        final requests = <http.Request>[];
        final config = await loadWebGatewayConfig(
          appUri: Uri.parse('$origin/chat/selected?view=inbox'),
          client: MockClient((request) async {
            requests.add(request);
            return http.Response(jsonEncode(binding), 200);
          }),
        );
        expect(config.baseUrl, binding['api_base_url']);
        expect(config.livekitUrl, binding['livekit_url']);
        expect(config.scheduledMessagesEnabled, isFalse);
        expect(
          requests.single.url.toString(),
          '$origin/voice-runtime-config.json',
        );
        expect(requests.single.followRedirects, isFalse);
      }
    },
  );

  test(
    'foreign environment config cannot fall back to compiled DEV URL',
    () async {
      final requests = <Uri>[];
      await expectLater(
        loadWebGatewayConfig(
          appUri: Uri.parse(staging['app_origin']! as String),
          client: MockClient((request) async {
            requests.add(request.url);
            return http.Response(jsonEncode(dev), 200);
          }),
        ),
        throwsFormatException,
      );
      expect(requests, [
        Uri.parse('https://app.comrade.click/voice-runtime-config.json'),
      ]);
    },
  );

  for (final status in [302, 404, 503]) {
    test(
      'HTTP $status config response denies bootstrap without a fallback',
      () async {
        await expectLater(
          loadWebGatewayConfig(
            appUri: Uri.parse(staging['app_origin']! as String),
            client: MockClient((_) async => http.Response('', status)),
          ),
          throwsFormatException,
        );
      },
    );
  }

  test(
    'missing, malformed, unsupported and unsafe bindings deny bootstrap',
    () async {
      final invalid = <Object?>[
        null,
        [],
        {...staging}..remove('app_origin'),
        {...staging, 'version': 2},
        {...staging, 'api_base_url': ''},
        {...staging, 'api_base_url': 'http://voice.comrade.click'},
        {...staging, 'api_base_url': 'https://user:pass@voice.comrade.click'},
        {...staging, 'api_base_url': 'https://voice.comrade.click/api'},
        {...staging, 'api_base_url': 'https://voice.comrade.click?env=dev'},
        {...staging, 'api_base_url': 'https://voice.comrade.click#dev'},
        {...staging, 'livekit_url': 'https://livekit.comrade.click'},
        {...staging, 'livekit_url': ''},
        {...staging, 'scheduled_messages_enabled': true},
      ];
      for (final binding in invalid) {
        await expectLater(
          loadWebGatewayConfig(
            appUri: Uri.parse(staging['app_origin']! as String),
            client: MockClient(
              (_) async => http.Response(jsonEncode(binding), 200),
            ),
          ),
          throwsFormatException,
          reason: '$binding',
        );
      }
      await expectLater(
        loadWebGatewayConfig(
          appUri: Uri.parse(staging['app_origin']! as String),
          client: MockClient(
            (_) async => http.Response('<html>SPA fallback</html>', 200),
          ),
        ),
        throwsFormatException,
      );
    },
  );
}
