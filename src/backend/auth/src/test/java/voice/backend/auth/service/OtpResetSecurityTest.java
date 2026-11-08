package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import org.junit.jupiter.api.Test;
import voice.backend.auth.mail.NoopMailSender;
import voice.backend.auth.repository.*;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.RefreshTokenCodec;

class OtpResetSecurityTest {
  private static final Instant NOW = Instant.parse("2026-11-02T10:00:00Z");
  private final Clock clock = Clock.fixed(NOW, ZoneOffset.UTC);
  private final InMemoryAccountRepository accounts = new InMemoryAccountRepository(clock);
  private final InMemoryOtpCodeRepository codes = new InMemoryOtpCodeRepository();
  private final InMemoryRefreshTokenRepository refresh = new InMemoryRefreshTokenRepository(clock);
  private final RefreshTokenCodec codec = new RefreshTokenCodec();
  private final BCryptPasswordHasher passwords = new BCryptPasswordHasher();
  private final OtpService service = new OtpService(accounts, codes, refresh, codec, passwords,
      new NoopMailSender(), new InMemoryOtpThrottle(clock), clock, (account, otp, now) -> {});

  @Test
  void consumedReplacementNeverResurrectsSupersededCodeForEitherType() {
    var account = accounts.create("superseded@example.com", null, "old", "regular");
    for (String type : new String[] {"email_verify", "password_reset"}) {
      codes.create(account.id(), codec.hash("111111"), type, NOW.plusSeconds(600), NOW);
      var replacement = codes.create(account.id(), codec.hash("222222"), type,
          NOW.plusSeconds(660), NOW.plusSeconds(60));
      codes.markUsed(replacement.id(), NOW.plusSeconds(61));
      assertThat(codes.findLatestValid(account.id(), type, NOW.plusSeconds(62))).isEmpty();
    }
  }

  @Test
  void latestMemoryIssuanceWinsWithFixedBackwardClocksAndShorterExpiry() {
    var account = accounts.create("clock@example.com", null, "old", "regular");
    for (String type : new String[] {"email_verify", "password_reset"}) {
      codes.create(account.id(), codec.hash("111111"), type, NOW.plusSeconds(600), NOW);
      var second = codes.create(account.id(), codec.hash("222222"), type, NOW.plusSeconds(300), NOW);
      assertThat(codes.findLatestValid(account.id(), type, NOW)).contains(second);
      var third = codes.create(account.id(), codec.hash("333333"), type,
          NOW.plusSeconds(100), NOW.minusSeconds(60));
      assertThat(codes.findLatestValid(account.id(), type, NOW)).contains(third);
      codes.markUsed(third.id(), NOW);
      assertThat(codes.findLatestValid(account.id(), type, NOW)).isEmpty();
    }
  }

  @Test
  void resetConsumesOnceRevokesRefreshAndAdvancesDurableEpoch() {
    var account = accounts.create("reset@example.com", null, passwords.hash("OldPassword99!"), "regular");
    codes.create(account.id(), codec.hash("123456"), "password_reset", NOW.plusSeconds(600), NOW);
    refresh.create(account.id(), "refresh", "{}", "jti", NOW.plusSeconds(3600), NOW);
    service.resetPassword(new ResetPasswordCommand(account.email(), "123456", "NewPassword99!"));
    var updated = accounts.findById(account.id().toString()).orElseThrow();
    assertThat(passwords.matches("NewPassword99!", updated.passwordHash())).isTrue();
    assertThat(updated.sessionEpoch()).isGreaterThan(account.sessionEpoch());
    assertThat(refresh.findByHash("refresh").orElseThrow().revoked()).isTrue();
    assertThatThrownBy(() -> service.resetPassword(new ResetPasswordCommand(account.email(), "123456", "ThirdPassword99!")))
        .isInstanceOf(AuthException.class).hasMessage("invalid_otp");
  }

  @Test
  void publicVerifyAndResetRejectSupersededCodesAfterReplacementConsumption() {
    var account = accounts.create("public@example.com", null, "old", "regular");
    for (String type : new String[] {"email_verify", "password_reset"}) {
      codes.create(account.id(), codec.hash("111111"), type, NOW.plusSeconds(600), NOW);
      codes.create(account.id(), codec.hash("222222"), type, NOW.plusSeconds(660), NOW.plusSeconds(60));
      service.verifyOtp(new VerifyOtpCommand(account.email(), null, "222222", type, null), null);
      assertThatThrownBy(() -> service.verifyOtp(
          new VerifyOtpCommand(account.email(), null, "111111", type, null), null))
          .isInstanceOf(AuthException.class).hasMessage("invalid_otp");
    }
    var resetAccount = accounts.create("public-reset@example.com", null, "old", "regular");
    codes.create(resetAccount.id(), codec.hash("111111"), "password_reset", NOW.plusSeconds(600), NOW);
    codes.create(resetAccount.id(), codec.hash("222222"), "password_reset", NOW.plusSeconds(660), NOW.plusSeconds(60));
    service.resetPassword(new ResetPasswordCommand(resetAccount.email(), "222222", "NewPassword99!"));
    assertThatThrownBy(() -> service.resetPassword(new ResetPasswordCommand(resetAccount.email(), "111111", "ThirdPassword99!")))
        .isInstanceOf(AuthException.class).hasMessage("invalid_otp");
  }
}
