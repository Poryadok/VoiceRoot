package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import java.time.*;
import java.util.Optional;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import voice.backend.auth.config.AuthProperties;
import voice.backend.auth.events.NoopAuthEventPublisher;
import voice.backend.auth.events.AuthEventPublisher;
import voice.backend.auth.mail.NoopMailSender;
import voice.backend.auth.repository.*;
import voice.backend.auth.security.*;
import voice.backend.auth.sessionepoch.*;
import voice.backend.auth.userdb.*;

class AuthServiceRestoreSessionEpochTest {
  private static final Clock CLOCK=Clock.fixed(Instant.parse("2026-05-01T10:00:00Z"),ZoneOffset.UTC);
  @Test void floorFailureLeavesRestoreTokenAndDeletedAccountForHealthyRedisAheadRetry() throws Exception {
    Harness h=new Harness(); Account account=h.accounts.create("restore-epoch@example.com",null,"hash","regular");
    h.accounts.markDeleted(account.id(),CLOCK.instant()); h.tokens.store("restore-token",account.id(),Duration.ofDays(1)); h.floor.failure=new IllegalStateException("redis down");
    assertThatThrownBy(()->h.service.restoreAccount("restore-token")).isInstanceOf(SessionEpochFloorUnavailableException.class);
    assertThat(h.floor.calls).isEqualTo(1); assertThat(h.tokens.consumeCalls).isZero(); assertThat(h.accounts.restoreCalls).isZero(); assertThat(h.events.restoreCalls).isZero(); assertThat(h.refresh.createCalls).isZero(); assertThat(h.accounts.findById(account.id().toString()).orElseThrow().status()).isEqualTo("deleted");
    h.floor.failure=null; h.floor.result=7L; h.floor.calls=0; AuthSession restored=h.service.restoreAccount("restore-token");
    var claims=com.nimbusds.jwt.SignedJWT.parse(restored.accessToken()).getJWTClaimsSet();
    assertThat(restored.accountId()).isEqualTo(account.id().toString()); assertThat(claims.getLongClaim("session_epoch")).isEqualTo(7L);
    assertThat(restored.profileId()).isNotBlank(); assertThat(restored.accountType()).isEqualTo("regular");
    assertThat(claims.getStringClaim("user_id")).isEqualTo(restored.accountId()); assertThat(claims.getStringClaim("profile_id")).isEqualTo(restored.profileId()); assertThat(claims.getStringClaim("account_type")).isEqualTo(restored.accountType());
    var validated=h.service.validate(restored.accessToken()); assertThat(validated.userId()).isEqualTo(restored.accountId()); assertThat(validated.profileId()).isEqualTo(restored.profileId()); assertThat(validated.normalizedAccountType()).isEqualTo(restored.accountType()); assertThat(h.tokens.consumeCalls).isEqualTo(1); assertThat(h.floor.calls).isEqualTo(1); assertThat(h.accounts.restoreCalls).isEqualTo(1); assertThat(h.events.restoreCalls).isEqualTo(1); assertThat(h.refresh.createCalls).isEqualTo(1);
    assertThat(h.accounts.findById(account.id().toString()).orElseThrow().status()).isEqualTo("active");
  }
  @Test void consumedRestoreIdMustMatchPeekedId() {
    RecordingAccounts accounts=new RecordingAccounts();
    Account peeked=accounts.create("restore-peek@example.com",null,"hash","regular"); Account consumed=accounts.create("restore-consumed@example.com",null,"hash","regular");
    accounts.markDeleted(peeked.id(),CLOCK.instant()); accounts.markDeleted(consumed.id(),CLOCK.instant());
    RecordingFloor floor=new RecordingFloor(); RecordingRefresh refresh=new RecordingRefresh(); RecordingEvents events=new RecordingEvents(); AuthService service=service(accounts,new MismatchingTokens(peeked.id(),consumed.id()),floor,refresh,events);
    assertThatThrownBy(()->service.restoreAccount("restore-token")).isInstanceOf(AuthException.class).hasMessage("invalid_token");
    assertThat(accounts.findById(peeked.id().toString()).orElseThrow().status()).isEqualTo("deleted");
    assertThat(accounts.findById(consumed.id().toString()).orElseThrow().status()).isEqualTo("deleted");
    assertThat(accounts.restoreCalls).isZero(); assertThat(events.restoreCalls).isZero(); assertThat(refresh.createCalls).isZero();
  }
  @Test void consumedRestoreRechecksDeletedStateBeforeRestoreOrSession() {
    RecordingAccounts accounts=new RecordingAccounts(); Account account=accounts.create("restore-state@example.com",null,"hash","regular"); accounts.markDeleted(account.id(),CLOCK.instant());
    RecordingRefresh refresh=new RecordingRefresh(); RecordingEvents events=new RecordingEvents(); AuthService service=service(accounts,new MutatingTokens(account.id(),()->accounts.setStatus(account.id(),"suspended")),new RecordingFloor(),refresh,events);
    assertThatThrownBy(()->service.restoreAccount("restore-token")).isInstanceOf(AuthException.class).hasMessage("validation_failed");
    assertThat(accounts.findById(account.id().toString()).orElseThrow().status()).isEqualTo("suspended");
    assertThat(accounts.restoreCalls).isZero(); assertThat(events.restoreCalls).isZero(); assertThat(refresh.createCalls).isZero();
  }
  @Test void consumedRestoreRechecksGraceBeforeRestoreOrSession() {
    RecordingAccounts accounts=new RecordingAccounts(); Account account=accounts.create("restore-grace@example.com",null,"hash","regular"); accounts.markDeleted(account.id(),CLOCK.instant());
    RecordingRefresh refresh=new RecordingRefresh(); RecordingEvents events=new RecordingEvents(); AuthService service=service(accounts,new MutatingTokens(account.id(),()->accounts.markDeleted(account.id(),CLOCK.instant().minus(Duration.ofDays(31)))),new RecordingFloor(),refresh,events);
    assertThatThrownBy(()->service.restoreAccount("restore-token")).isInstanceOf(AuthException.class).hasMessage("account_inactive");
    assertThat(accounts.findById(account.id().toString()).orElseThrow().status()).isEqualTo("deleted");
    assertThat(accounts.restoreCalls).isZero(); assertThat(events.restoreCalls).isZero(); assertThat(refresh.createCalls).isZero();
  }
  private static final class Harness { final RecordingAccounts accounts=new RecordingAccounts(); final RecordingTokens tokens=new RecordingTokens(); final RecordingFloor floor=new RecordingFloor(); final RecordingRefresh refresh=new RecordingRefresh(); final RecordingEvents events=new RecordingEvents(); final AuthService service;
    Harness(){service=service(accounts,tokens,floor,refresh,events);}}
  private static AuthService service(RecordingAccounts accounts,AccountRestoreTokenStore tokens,RecordingFloor floor,RecordingRefresh refresh,RecordingEvents events){InMemoryPrimaryProfileProvisioner p=new InMemoryPrimaryProfileProvisioner();return new AuthService(accounts,refresh,new RefreshTokenCodec(),new BCryptPasswordHasher(),JwtService.forTests("voice-auth","voice-client","key",Duration.ofMinutes(15),CLOCK),new InMemoryTokenBlacklist(CLOCK),new TotpService(memory()),new BackupCodeService(new InMemoryBackupCodeRepository()),CLOCK,Duration.ofDays(30),p,new InMemoryPhoneHashResolver(accounts,p),new InMemorySubscriptionTierStore(),new NoOpProfileSwitchValidator(),new InMemoryE2EKeyBackupRepository(),events,new SimpleMeterRegistry(),tokens,new NoopMailSender(),floor);}
  private static AuthProperties memory(){AuthProperties p=new AuthProperties();p.setPersistence(AuthProperties.PersistenceMode.MEMORY);return p;}
  private static final class RecordingTokens extends InMemoryAccountRestoreTokenStore {int consumeCalls; @Override public Optional<UUID> consume(String t){consumeCalls++;return super.consume(t);}}
  private static final class RecordingAccounts extends InMemoryAccountRepository {int restoreCalls; RecordingAccounts(){super(CLOCK);}@Override public synchronized boolean restoreDeleted(UUID id){restoreCalls++;return super.restoreDeleted(id);}}
  private static final class RecordingRefresh extends InMemoryRefreshTokenRepository {int createCalls; @Override public RefreshTokenRecord create(UUID id,String hash,String device,String jti,Instant expires,Instant now){createCalls++;return super.create(id,hash,device,jti,expires,now);}}
  private static final class RecordingEvents implements AuthEventPublisher {int restoreCalls;public void publishGuestConverted(UUID id){}public void publishAccountDeleted(UUID id){}public void publishAccountRestored(UUID id){restoreCalls++;}}
  private static final class MismatchingTokens implements AccountRestoreTokenStore {final UUID peeked,consumed;MismatchingTokens(UUID p,UUID c){peeked=p;consumed=c;}public void store(String t,UUID id,Duration ttl){}public Optional<UUID> peek(String t){return Optional.of(peeked);}public Optional<UUID> consume(String t){return Optional.of(consumed);}}
  private static final class MutatingTokens implements AccountRestoreTokenStore {final UUID id;final Runnable mutation;MutatingTokens(UUID id,Runnable mutation){this.id=id;this.mutation=mutation;}public void store(String t,UUID id,Duration ttl){}public Optional<UUID> peek(String t){return Optional.of(id);}public Optional<UUID> consume(String t){mutation.run();return Optional.of(id);}}
  private static final class RecordingFloor implements SessionEpochFloorStore {RuntimeException failure;long result=1;int calls;public long recordAtLeast(UUID id,long e){calls++;if(failure!=null)throw failure;return result;}public long requireFloor(UUID id){throw new AssertionError();}}
}
