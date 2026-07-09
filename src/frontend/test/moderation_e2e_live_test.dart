import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/moderation_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test(
    'moderation moderation: report submit accepted',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final reporter = await ctx.registerUser('p14-reporter');
      final target = await ctx.registerUser('p14-target');

      final moderation = VoiceModerationClient(gateway: ctx.gatewayHttp());
      final report = await moderation.createReport(
        authorization: reporter.authorizationHeader,
        targetType: 'user',
        targetId: target.activeProfileId,
        category: 'spam',
      );
      expect(report, isA<ModerationApiOk<ReportSubmission>>());
      expect(
        (report as ModerationApiOk<ReportSubmission>).data.reportId,
        isNotEmpty,
      );
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
