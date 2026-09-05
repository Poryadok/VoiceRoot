package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import voice.backend.auth.mail.NoopMailSender;
import voice.backend.auth.repository.InMemoryAccountRepository;
import voice.backend.auth.repository.InMemoryOtpCodeRepository;
import voice.backend.auth.repository.InMemoryRefreshTokenRepository;
import voice.backend.auth.repository.OtpCodeRecord;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.RefreshTokenCodec;

class OtpServiceGuestConversionAcceptanceTest {
  private static final Clock CLOCK = Clock.fixed(Instant.parse("2026-09-04T10:15:30Z"), ZoneOffset.UTC);

  @Test
  void successfulGuestEmailVerificationDelegatesTheExactOtpOnceWithoutSeparateOtpConsumption() {
    InMemoryAccountRepository accounts = new InMemoryAccountRepository();
    var guest = accounts.create("guest@example.com", null, "hash", "guest");
    RefreshTokenCodec codec = new RefreshTokenCodec();
    MarkUsedMustNotBeCalledOtpCodes codes = new MarkUsedMustNotBeCalledOtpCodes();
    OtpCodeRecord record =
        codes.create(
            guest.id(), codec.hash("123456"), "email_verify", CLOCK.instant().plus(Duration.ofMinutes(10)), CLOCK.instant());
    RecordingAcceptance acceptance = new RecordingAcceptance();
    OtpService service =
        new OtpService(
            accounts,
            codes,
            new InMemoryRefreshTokenRepository(),
            codec,
            new BCryptPasswordHasher(),
            new NoopMailSender(),
            new InMemoryOtpThrottle(),
            CLOCK,
            acceptance);

    service.verifyOtp(
        new VerifyOtpCommand("guest@example.com", null, "123456", "email_verify", null), null);

    assertThat(acceptance.calls)
        .containsExactly(new AcceptanceCall(guest.id(), record, CLOCK.instant()));
  }

  @Test
  void failedGuestEmailAcceptanceLeavesOtpValidAndDoesNotFallBackToUserOrEventContinuation() {
    InMemoryAccountRepository accounts = new InMemoryAccountRepository();
    var guest = accounts.create("guest-failure@example.com", null, "hash", "guest");
    RefreshTokenCodec codec = new RefreshTokenCodec();
    InMemoryOtpCodeRepository codes = new InMemoryOtpCodeRepository();
    OtpCodeRecord record =
        codes.create(
            guest.id(), codec.hash("123456"), "email_verify", CLOCK.instant().plus(Duration.ofMinutes(10)), CLOCK.instant());
    RecordingAcceptance acceptance = new RecordingAcceptance();
    acceptance.failure = new IllegalStateException("durable acceptance failed");

    org.assertj.core.api.Assertions.assertThatThrownBy(
            () -> service(accounts, codes, codec, acceptance).verifyOtp(
                new VerifyOtpCommand("guest-failure@example.com", null, "123456", "email_verify", null), null))
        .isInstanceOf(IllegalStateException.class)
        .hasMessage("durable acceptance failed");

    assertThat(codes.findLatestValid(guest.id(), "email_verify", CLOCK.instant())).contains(record);
    assertThat(acceptance.calls).containsExactly(new AcceptanceCall(guest.id(), record, CLOCK.instant()));
    assertThat(acceptance.operationCreates).isZero();
  }

  @Test
  void regularEmailVerificationDoesNotInvokeGuestConversionAcceptance() {
    InMemoryAccountRepository accounts = new InMemoryAccountRepository();
    var regular = accounts.create("regular@example.com", null, "hash", "regular");
    RefreshTokenCodec codec = new RefreshTokenCodec();
    InMemoryOtpCodeRepository codes = new InMemoryOtpCodeRepository();
    codes.create(
        regular.id(), codec.hash("123456"), "email_verify", CLOCK.instant().plus(Duration.ofMinutes(10)), CLOCK.instant());
    RecordingAcceptance acceptance = new RecordingAcceptance();

    service(accounts, codes, codec, acceptance).verifyOtp(
        new VerifyOtpCommand("regular@example.com", null, "123456", "email_verify", null), null);

    assertThat(acceptance.calls).isEmpty();
  }

  @Test
  void guestPasswordResetDoesNotInvokeGuestConversionAcceptance() {
    InMemoryAccountRepository accounts = new InMemoryAccountRepository();
    var guest = accounts.create("guest-reset@example.com", null, "hash", "guest");
    RefreshTokenCodec codec = new RefreshTokenCodec();
    InMemoryOtpCodeRepository codes = new InMemoryOtpCodeRepository();
    codes.create(
        guest.id(), codec.hash("123456"), "password_reset", CLOCK.instant().plus(Duration.ofMinutes(10)), CLOCK.instant());
    RecordingAcceptance acceptance = new RecordingAcceptance();

    service(accounts, codes, codec, acceptance)
        .resetPassword(new ResetPasswordCommand("guest-reset@example.com", "123456", "new password 123"));

    assertThat(acceptance.calls).isEmpty();
  }

  private static OtpService service(
      InMemoryAccountRepository accounts,
      InMemoryOtpCodeRepository codes,
      RefreshTokenCodec codec,
      GuestConversionOtpAcceptance acceptance) {
    return new OtpService(
        accounts,
        codes,
        new InMemoryRefreshTokenRepository(),
        codec,
        new BCryptPasswordHasher(),
        new NoopMailSender(),
        new InMemoryOtpThrottle(),
        CLOCK,
        acceptance);
  }

  private static final class RecordingAcceptance implements GuestConversionOtpAcceptance {
    private final List<AcceptanceCall> calls = new ArrayList<>();
    private int operationCreates;
    private RuntimeException failure;

    @Override
    public void acceptVerifiedGuestEmailOtp(UUID accountId, OtpCodeRecord otp, Instant now) {
      calls.add(new AcceptanceCall(accountId, otp, now));
      if (failure != null) throw failure;
      operationCreates++;
    }
  }

  private static final class MarkUsedMustNotBeCalledOtpCodes extends InMemoryOtpCodeRepository {
    @Override
    public synchronized void markUsed(UUID id, Instant usedAt) {
      throw new AssertionError("OtpService must leave OTP consumption to the durable acceptance boundary");
    }
  }

  private record AcceptanceCall(UUID accountId, OtpCodeRecord otp, Instant now) {}
}
