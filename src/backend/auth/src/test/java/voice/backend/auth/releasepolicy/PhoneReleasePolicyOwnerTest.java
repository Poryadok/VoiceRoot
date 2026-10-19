package voice.backend.auth.releasepolicy;

import static org.assertj.core.api.Assertions.*;
import static org.mockito.Mockito.*;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.*;
import org.junit.jupiter.api.Test;
import voice.backend.auth.service.*;
import voice.backend.auth.repository.*;
import voice.backend.auth.security.*;
import voice.backend.auth.mail.*;
import voice.backend.auth.sessionepoch.InMemorySessionEpochFloorStore;

class PhoneReleasePolicyOwnerTest {
  static final Clock CLOCK = Clock.systemUTC();
  static final String PASSWORD = "correct password 123";
  final InMemoryAccountRepository accounts = spy(new InMemoryAccountRepository());
  final InMemoryRefreshTokenRepository refresh = spy(new InMemoryRefreshTokenRepository(CLOCK));
  final BCryptPasswordHasher passwords = spy(new BCryptPasswordHasher());
  final RefreshTokenCodec codec = new RefreshTokenCodec();
  final InMemoryTokenBlacklist blacklist = spy(new InMemoryTokenBlacklist(CLOCK));
  final InMemoryOtpCodeRepository codes = spy(new InMemoryOtpCodeRepository());
  final MailSender mail = mock(MailSender.class);
  final OtpThrottle throttle = spy(new InMemoryOtpThrottle(CLOCK));
  final JwtService jwt = JwtService.forTests("voice-auth", "voice-client", "test-key", Duration.ofMinutes(15), CLOCK);
  final InMemoryAccountRestoreTokenStore restoreTokens = spy(new InMemoryAccountRestoreTokenStore());
  final voice.backend.auth.events.AuthEventPublisher events = mock(voice.backend.auth.events.AuthEventPublisher.class);
  final InMemorySessionEpochFloorStore floors = spy(new InMemorySessionEpochFloorStore());

  AuthService auth() throws Exception {
    return auth(ReleasePolicyTest.alpha());
  }

  AuthService auth(ReleasePolicy policy) throws Exception {
    var profiles = new voice.backend.auth.userdb.InMemoryPrimaryProfileProvisioner();
    var properties = new voice.backend.auth.config.AuthProperties();
    properties.setPersistence(voice.backend.auth.config.AuthProperties.PersistenceMode.MEMORY);
    var service = new AuthService(accounts, refresh, codec, passwords,
        jwt,
        blacklist, new TotpService(properties), new BackupCodeService(new InMemoryBackupCodeRepository()),
        CLOCK, Duration.ofDays(30), profiles,
        new voice.backend.auth.userdb.InMemoryPhoneHashResolver(accounts, profiles),
        new InMemorySubscriptionTierStore(), new voice.backend.auth.userdb.NoOpProfileSwitchValidator(),
        new InMemoryE2EKeyBackupRepository(), events,
        new io.micrometer.core.instrument.simple.SimpleMeterRegistry(), restoreTokens,
        mail, floors);
    service.configureReleasePolicy(policy);
    return service;
  }
  @Test void registrationRejectsPhoneAndMixedCredentialsBeforeAnyOwnerEffects() throws Exception {
    var auth = auth(); clearInvocations(accounts, refresh, passwords, mail);
    for (String email : new String[] {null, "mixed@example.com"}) {
      assertThatThrownBy(() -> auth.register(new RegisterCommand(email, "+15551234567", PASSWORD, false, "{}")))
          .isInstanceOf(AuthException.class).hasMessage("capability_unavailable");
    }
    verifyNoInteractions(accounts, refresh, passwords, mail);
  }
  @Test void loginDeniesPreexistingPhoneAndEmailFallbackWithoutSessionOrCredentialEffects() throws Exception {
    var auth = auth();
    var phone = accounts.create("existing@example.com", "+15551234567", passwords.hash(PASSWORD), "regular");
    clearInvocations(accounts, refresh, passwords, blacklist, mail);
    for (String email : new String[] {null, phone.email()}) {
      assertThatThrownBy(() -> auth.login(new LoginCommand(email, phone.phone(), PASSWORD, "", "{}")))
          .isInstanceOf(AuthException.class).hasMessage("capability_unavailable");
    }
    verifyNoInteractions(accounts, refresh, passwords, blacklist, mail);
  }
  @Test void validGuestPhoneConversionHasNoEffectsAndEmailGuestControlsIssueRealSessions() throws Exception {
    var auth = auth();
    var guest = auth.register(new RegisterCommand(null, null, PASSWORD, true, "{}"));
    var email = auth.register(new RegisterCommand("control@example.com", null, PASSWORD, false, "{}"));
    assertThat(guest.accessToken()).isNotBlank(); assertThat(email.refreshToken()).isNotBlank();
    var loggedIn = auth.login(new LoginCommand("control@example.com", null, PASSWORD, "", "{}"));
    assertThat(loggedIn.accountId()).isEqualTo(email.accountId());
    var before = refresh.listActiveByAccount(java.util.UUID.fromString(guest.accountId()));
    clearInvocations(refresh, passwords, blacklist, mail);
    assertThatThrownBy(() -> auth.convertGuest(guest.accessToken(),
        new ConvertGuestCommand("conversion@example.com", "+15551234567", PASSWORD)))
        .isInstanceOf(AuthException.class).hasMessage("capability_unavailable");
    verifyNoInteractions(refresh, passwords, mail);
    verify(blacklist, never()).revoke(anyString(), any());
    assertThat(refresh.listActiveByAccount(java.util.UUID.fromString(guest.accountId()))).isEqualTo(before);
    assertThat(accounts.findById(guest.accountId()).orElseThrow().email()).isNull();
    assertThat(auth.validate(guest.accessToken()).userId()).isEqualTo(guest.accountId());
    assertThat(auth.withClock(CLOCK).releasePolicy()).isSameAs(auth.releasePolicy());
  }
  @Test void otpMixedPhoneDeniesBeforeLookupThrottleCodeOrMailAndEmailResetStillWorks() throws Exception {
    var auth = auth();
    var account = accounts.create("otp@example.com", null, passwords.hash(PASSWORD), "regular");
    var otp = new OtpService(accounts, codes, refresh, codec, passwords, mail, throttle, CLOCK,
        (id, code, now) -> { throw new AssertionError("not guest acceptance"); });
    otp.configureReleasePolicy(auth.releasePolicy());
    clearInvocations(accounts, codes, refresh, passwords, mail, throttle);
    assertThatThrownBy(() -> otp.sendOtp(new SendOtpCommand(account.email(), "+15551234567", "password_reset", null), auth))
        .isInstanceOf(AuthException.class).hasMessage("capability_unavailable");
    assertThatThrownBy(() -> otp.verifyOtp(new VerifyOtpCommand(account.email(), "+15551234567", "123456", "password_reset", null), auth))
        .isInstanceOf(AuthException.class).hasMessage("capability_unavailable");
    verifyNoInteractions(accounts, codes, refresh, passwords, mail, throttle);
    otp.sendOtp(new SendOtpCommand(account.email(), null, "password_reset", null), auth);
    verify(mail).sendOtpEmail(eq(account.email()), anyString(), contains("Voice verification code"));
    assertThat(codes.findLatestValid(account.id(), "password_reset", CLOCK.instant())).isPresent();
    var now = CLOCK.instant();
    codes.create(account.id(), codec.hash("234567"), "password_reset", now.plusSeconds(600), now.plusMillis(1));
    otp.resetPassword(new ResetPasswordCommand(account.email(), "234567", "replacement password"));
    assertThat(passwords.matches("replacement password", accounts.findById(account.id().toString()).orElseThrow().passwordHash())).isTrue();
  }

  @Test void preexistingPhoneRefreshCannotConsumeCredentialOrIssueReplacement() throws Exception {
    var development = auth(ReleasePolicy.development());
    var old = development.register(new RegisterCommand(null, "+15550000001", PASSWORD, false, "{}"));
    var alpha = auth();
    var before = refresh.findByHash(codec.hash(old.refreshToken())).orElseThrow();
    clearInvocations(refresh, passwords, blacklist);
    assertThatThrownBy(() -> alpha.refresh(new RefreshCommand(old.refreshToken(), "{}")))
        .isInstanceOf(AuthException.class).hasMessage("capability_unavailable");
    verify(refresh, never()).create(any(), any(), anyString(), anyString(), anyString(), any(), any());
    verify(refresh, never()).revokeIfActive(anyString(), any());
    verify(blacklist, never()).revoke(anyString(), any());
    assertThat(refresh.findByHash(codec.hash(old.refreshToken()))).contains(before);
  }

  @Test void deniedPhoneProfileSwitchPreservesCredentialsAndDoesNotPrepareFloor() throws Exception {
    var development = auth(ReleasePolicy.development());
    var old = development.register(new RegisterCommand(null, "+15550000003", PASSWORD, false, "{}"));
    var alpha = auth();
    var before = refresh.findByHash(codec.hash(old.refreshToken()));
    clearInvocations(blacklist, refresh, floors);
    assertThatThrownBy(() -> alpha.switchActiveProfile(old.accessToken(), java.util.UUID.randomUUID().toString(), "{}"))
        .isInstanceOf(AuthException.class).hasMessage("capability_unavailable");
    verify(blacklist, never()).revoke(anyString(), any());
    verify(floors, never()).recordAtLeast(any(), anyLong());
    verifyNoInteractions(refresh);
    assertThat(alpha.validate(old.accessToken()).userId()).isEqualTo(old.accountId());
    assertThat(refresh.findByHash(codec.hash(old.refreshToken()))).isEqualTo(before);
  }

  @Test void allowedEmailProfileSwitchRevokesOldAccessAndIssuesTargetSession() throws Exception {
    var alpha = auth();
    var old = alpha.register(new RegisterCommand("switch@example.com", null, PASSWORD, false, "{}"));
    var target = java.util.UUID.randomUUID().toString();
    var switched = alpha.switchActiveProfile(old.accessToken(), target, "{}");
    assertThat(alpha.validate(switched.accessToken()).profileId()).isEqualTo(target);
    assertThat(switched.refreshToken()).isNotBlank();
    assertThatThrownBy(() -> alpha.validate(old.accessToken())).hasMessage("token_revoked");
  }

  @Test void deniedPhoneRestorePreservesDeletedAccountReusableTokenAndNoEventOrFloor() throws Exception {
    var account = accounts.create(null, "+15550000004", passwords.hash(PASSWORD), "regular");
    accounts.markDeleted(account.id(), CLOCK.instant());
    restoreTokens.store("phone-restore", account.id(), AccountRestoreTokenStore.RESTORE_TTL);
    var deleted = accounts.findById(account.id().toString()).orElseThrow();
    var alpha = auth();
    clearInvocations(accounts, restoreTokens, events, floors, refresh);
    for (int attempt = 0; attempt < 2; attempt++) {
      assertThatThrownBy(() -> alpha.restoreAccount("phone-restore"))
          .isInstanceOf(AuthException.class).hasMessage("capability_unavailable");
    }
    verify(restoreTokens, never()).consume(anyString());
    verify(accounts, never()).restoreDeleted(any());
    verify(floors, never()).recordAtLeast(any(), anyLong());
    verifyNoInteractions(events, refresh);
    assertThat(restoreTokens.peek("phone-restore")).contains(account.id());
    assertThat(accounts.findById(account.id().toString())).contains(deleted);
  }

  @Test void allowedEmailRestoreConsumesTokenRestoresPublishesAndIssuesSession() throws Exception {
    var account = accounts.create("restore@example.com", null, passwords.hash(PASSWORD), "regular");
    accounts.markDeleted(account.id(), CLOCK.instant());
    restoreTokens.store("email-restore", account.id(), AccountRestoreTokenStore.RESTORE_TTL);
    var alpha = auth();
    var restored = alpha.restoreAccount("email-restore");
    assertThat(alpha.validate(restored.accessToken()).userId()).isEqualTo(account.id().toString());
    assertThat(restored.refreshToken()).isNotBlank();
    assertThat(restoreTokens.peek("email-restore")).isEmpty();
    assertThat(accounts.findById(account.id().toString()).orElseThrow().status()).isEqualTo("active");
    verify(events).publishAccountRestored(account.id());
    assertThatThrownBy(() -> alpha.restoreAccount("email-restore")).hasMessage("invalid_token");
  }

  @Test void grpcAndHttpExposeTheCapabilityFailureForPublicPhoneRequest() throws Exception {
    var auth = auth();
    var grpc = new voice.backend.auth.grpc.AuthGrpcService(auth, null);
    var failure = new java.util.concurrent.atomic.AtomicReference<Throwable>();
    var observer = new io.grpc.stub.StreamObserver<app.voice.auth.v1.RegisterResponse>() {
      public void onNext(app.voice.auth.v1.RegisterResponse value) { throw new AssertionError("unexpected session"); }
      public void onCompleted() { throw new AssertionError("unexpected success"); }
      public void onError(Throwable error) {
        failure.set(error);
      }
    };
    grpc.register(app.voice.auth.v1.RegisterRequest.newBuilder().setPhone("+15551234567")
        .setPassword(PASSWORD).build(), observer);
    assertThat(failure.get()).isNotNull();
    assertThat(io.grpc.Status.fromThrowable(failure.get()).getCode()).isEqualTo(io.grpc.Status.Code.FAILED_PRECONDITION);
    assertThat(io.grpc.Status.fromThrowable(failure.get()).getDescription()).isEqualTo("capability_unavailable");
    var rest = new voice.backend.auth.rest.AuthRestController(auth, null, null);
    var mvc = org.springframework.test.web.servlet.setup.MockMvcBuilders.standaloneSetup(rest).build();
    mvc.perform(org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post("/api/v1/auth/register")
        .contentType(org.springframework.http.MediaType.APPLICATION_JSON)
        .content("{\"phone\":\"+15551234567\",\"password\":\"correct password 123\",\"guest\":false}"))
        .andExpect(org.springframework.test.web.servlet.result.MockMvcResultMatchers.status().isPreconditionFailed())
        .andExpect(org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath("$.error").value("capability_unavailable"));
  }

  @Test void oldValidPhonePrincipalCannotCompleteTwoFactorIssuanceOrRewriteEnrollment() throws Exception {
    var development = auth(ReleasePolicy.development());
    var old = development.register(new RegisterCommand(null, "+15550000002", PASSWORD, false, "{}"));
    development.enable2FA(old.accessToken(), PASSWORD);
    var alpha = auth();
    clearInvocations(accounts, refresh);
    assertThatThrownBy(() -> alpha.verify2FA(old.accessToken(), "000000"))
        .isInstanceOf(AuthException.class).hasMessage("capability_unavailable");
    verify(accounts, never()).setTotpEnabled(any(), anyBoolean());
    verifyNoInteractions(refresh);
    assertThat(accounts.findById(old.accountId()).orElseThrow().totpEnabled()).isFalse();
  }
}
