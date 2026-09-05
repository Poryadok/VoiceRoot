package voice.backend.auth.config;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.Arrays;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import voice.backend.auth.mail.NoopMailSender;
import voice.backend.auth.repository.InMemoryAccountRepository;
import voice.backend.auth.repository.InMemoryOtpCodeRepository;
import voice.backend.auth.repository.InMemoryRefreshTokenRepository;
import voice.backend.auth.repository.OtpCodeRecord;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.RefreshTokenCodec;
import voice.backend.auth.service.GuestConversionOtpAcceptance;
import voice.backend.auth.service.InMemoryOtpThrottle;
import voice.backend.auth.service.OtpService;
import voice.backend.auth.service.VerifyOtpCommand;

class OtpAndMailConfigurationGuestConversionWiringTest {
  private static final Clock CLOCK =
      Clock.fixed(Instant.parse("2026-09-04T10:15:30Z"), ZoneOffset.UTC);

  @Test
  void configuredPublicOtpServiceReceivesDurableGuestAcceptanceWithoutALegacyFactoryOverload() {
    InMemoryAccountRepository accounts = new InMemoryAccountRepository();
    var guest = accounts.create("wired-guest@example.com", null, "hash", "guest");
    InMemoryOtpCodeRepository otpCodes = new InMemoryOtpCodeRepository();
    RefreshTokenCodec codec = new RefreshTokenCodec();
    OtpCodeRecord otp =
        otpCodes.create(
            guest.id(),
            codec.hash("123456"),
            "email_verify",
            CLOCK.instant().plus(Duration.ofMinutes(10)),
            CLOCK.instant());
    RecordingAcceptance acceptance = new RecordingAcceptance();

    OtpService service =
        new OtpAndMailConfiguration()
            .otpService(
                accounts,
                otpCodes,
                new InMemoryRefreshTokenRepository(),
                codec,
                new BCryptPasswordHasher(),
                new NoopMailSender(),
                new InMemoryOtpThrottle(),
                CLOCK,
                acceptance);

    service.verifyOtp(
        new VerifyOtpCommand("wired-guest@example.com", null, "123456", "email_verify", null), null);

    assertThat(acceptance.accountId).isEqualTo(guest.id());
    assertThat(acceptance.otp).isEqualTo(otp);
    assertThat(
            Arrays.stream(OtpAndMailConfiguration.class.getDeclaredMethods())
                .filter(method -> method.getName().equals("otpService"))
                .map(method -> Arrays.asList(method.getParameterTypes())))
        .allSatisfy(parameters -> assertThat(parameters).contains(GuestConversionOtpAcceptance.class));
    assertThat(
            Arrays.stream(OtpService.class.getConstructors())
                .map(constructor -> Arrays.asList(constructor.getParameterTypes())))
        .allSatisfy(parameters -> assertThat(parameters).contains(GuestConversionOtpAcceptance.class));
  }

  private static final class RecordingAcceptance implements GuestConversionOtpAcceptance {
    private UUID accountId;
    private OtpCodeRecord otp;

    @Override
    public void acceptVerifiedGuestEmailOtp(UUID accountId, OtpCodeRecord otp, Instant now) {
      this.accountId = accountId;
      this.otp = otp;
    }
  }
}
