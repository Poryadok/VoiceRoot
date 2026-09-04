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
  void successfulGuestEmailVerificationUsesDurableAcceptanceAndDoesNotNeedUserOrEventContinuation() {
    InMemoryAccountRepository accounts = new InMemoryAccountRepository();
    var guest = accounts.create("guest@example.com", null, "hash", "guest");
    RefreshTokenCodec codec = new RefreshTokenCodec();
    InMemoryOtpCodeRepository codes = new InMemoryOtpCodeRepository();
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
    assertThat(codes.findLatestValid(guest.id(), "email_verify", CLOCK.instant())).isEmpty();
  }

  private static final class RecordingAcceptance implements GuestConversionOtpAcceptance {
    private final List<AcceptanceCall> calls = new ArrayList<>();

    @Override
    public void acceptVerifiedGuestEmailOtp(UUID accountId, OtpCodeRecord otp, Instant now) {
      calls.add(new AcceptanceCall(accountId, otp, now));
    }
  }

  private record AcceptanceCall(UUID accountId, OtpCodeRecord otp, Instant now) {}
}
