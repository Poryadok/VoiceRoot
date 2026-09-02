import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/moderation_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';

import 'support/gateway_test_client.dart';

void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';

  group('VoiceModerationClient.submitAppeal', () {
    test('POST /api/v1/moderation/appeals', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/moderation/appeals');
        expect(req.headers['Authorization'], auth);
        expect(req.body, contains('sanction-1'));
        expect(req.body, contains('mistake'));
        return utf8JsonResponse(
          '{"appeal":{"id":"appeal-1","sanction_id":"sanction-1","status":"pending"}}',
          status: 201,
        );
      });
      final client = VoiceModerationClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final r = await client.submitAppeal(
        authorization: auth,
        sanctionId: 'sanction-1',
        reason: 'mistake',
      );
      expect(r, isA<ModerationApiOk<AppealSubmission>>());
      final appeal = (r as ModerationApiOk<AppealSubmission>).data;
      expect(appeal.appealId, 'appeal-1');
      expect(appeal.status, 'pending');
    });
  });
}
