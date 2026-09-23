import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/state/auth_providers.dart';

import 'support/live_gateway_harness.dart';

class _TrackingClient extends http.BaseClient {
  _TrackingClient(this.inner);

  final http.Client inner;
  int sends = 0;
  int verifies = 0;

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) {
    if (request.url.path == '/api/v1/auth/otp/send') sends++;
    if (request.url.path == '/api/v1/auth/otp/verify') verifies++;
    return inner.send(request);
  }
}

void main() {
  test(
    'A1 pending email survives logout and fresh login before verification',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;
      final trackedHttp = _TrackingClient(ctx.httpClient);
      final auth = VoiceAuthClient(
        gateway: GatewayHttpClient(httpClient: trackedHttp, config: ctx.config),
      );
      final storage = InMemoryAuthSessionStorage();
      final email = qaUniqueEmail('a1-pending-reentry');
      final first = AuthController(
        authClient: auth,
        storage: storage,
        guestCredentialsStorage: InMemoryGuestCredentialsStorage(),
      );
      await first.register(email: email, password: qaPassword);
      final original = first.state.session;
      expect(original, isNotNull);
      expect(original!.accountType, 'guest');
      expect(original.emailVerificationRequired, isTrue);
      expect(first.state.isEmailVerificationPending, isTrue);

      await first.logout();
      expect(first.state.session, isNull);
      expect(await storage.read(), isNull);
      expect(await ctx.protectedRouteStatus(original.accessToken), 401);
      first.dispose();

      AuthSession? sessionAtRouting;
      final reentry = AuthController(
        authClient: auth,
        storage: storage,
        guestCredentialsStorage: InMemoryGuestCredentialsStorage(),
        onAuthenticated: () async {
          sessionAtRouting = await storage.read();
        },
      );
      addTearDown(reentry.dispose);
      await reentry.login(email: email, password: qaPassword);
      final restricted = reentry.state.session;
      expect(restricted, isNotNull);
      expect(restricted!.accountId, original.accountId);
      expect(restricted.activeProfileId, original.activeProfileId);
      expect(restricted.accessToken, isNot(original.accessToken));
      expect(restricted.accountType, 'guest');
      expect(restricted.emailVerificationRequired, isTrue);
      expect(reentry.state.isEmailVerificationPending, isTrue);
      expect(sessionAtRouting, isNull);
      expect(trackedHttp.sends, 0, reason: 'fresh login must not replay OTP');
      expect(
        trackedHttp.verifies,
        0,
        reason: 'fresh login must not replay OTP',
      );

      final status = await auth.getEmailVerificationStatus(session: restricted);
      expect(status, isA<AuthApiOk<EmailVerificationRecoveryState>>());
      expect(
        (status as AuthApiOk<EmailVerificationRecoveryState>).data,
        EmailVerificationRecoveryState.emailPending,
      );
      // Registration already sent the one initial code. Resend is throttled for
      // one minute, and fresh login must not silently request another code.
      final code = await waitForLiveVerificationCode(
        httpClient: ctx.httpClient,
        stubBaseUrl: liveAuthMailStubBaseUrl(),
        email: email,
      );
      final verifyResult = await reentry.verifyEmailVerification(code);
      if (verifyResult == 'email_verification_promotion_pending') {
        final pendingStatus = await auth.getEmailVerificationStatus(
          session: reentry.state.session!,
        );
        expect(pendingStatus, isA<AuthApiOk<EmailVerificationRecoveryState>>());
        expect(
          (pendingStatus as AuthApiOk<EmailVerificationRecoveryState>).data,
          anyOf(
            EmailVerificationRecoveryState.promotionPending,
            EmailVerificationRecoveryState.regular,
          ),
        );
        var promoted = false;
        for (var attempt = 0; attempt < 24; attempt++) {
          await Future<void>.delayed(const Duration(seconds: 5));
          if (await reentry.resumeEmailVerificationPromotion() == null) {
            promoted = true;
            break;
          }
        }
        expect(promoted, isTrue, reason: 'bounded promotion retry');
      } else {
        expect(verifyResult, isNull);
      }
      expect(trackedHttp.verifies, 1, reason: 'OTP must be consumed once');
      expect(trackedHttp.sends, 0, reason: 'no implicit OTP resend');
      final regular = reentry.state.session;
      expect(regular, isNotNull);
      expect(regular!.accountId, original.accountId);
      expect(regular.activeProfileId, original.activeProfileId);
      expect(regular.accountType, 'regular');
      expect(regular.accessToken, isNot(restricted.accessToken));
      expect((await storage.read())?.accessToken, regular.accessToken);
      expect(sessionAtRouting?.accessToken, regular.accessToken);
      expect(reentry.state.isEmailVerificationPending, isFalse);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
