package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

import java.time.Clock;
import java.time.Duration;
import java.util.Map;
import java.util.UUID;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.transaction.support.TransactionTemplate;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import voice.backend.auth.repository.JdbcAccountRepository;
import voice.backend.auth.repository.RefreshTokenRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.JwtService;
import voice.backend.auth.service.AuthException;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.RegisterCommand;
import voice.backend.auth.service.RegistrationSessionEpochPreparer;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;
import voice.backend.auth.sessionepoch.SessionEpochIssuanceGate;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;

@Testcontainers(disabledWithoutDocker = true)
class SdkErasedRegistrationJdbcIntegrationTest {
  @Container static final PostgreSQLContainer<?> postgres = new PostgreSQLContainer<>("postgres:16-alpine");

  @Test void erasedIntentRollsBackNewAccountBeforeAnySessionSideEffect() {
    var database = new DriverManagerDataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword());
    Flyway.configure().dataSource(database).locations("classpath:db/migration").load().migrate();
    var jdbc = new NamedParameterJdbcTemplate(database);
    // Allows the consumer-only branch to exercise the agreed contract before schema integration.
    jdbc.getJdbcTemplate().execute("""
        CREATE TABLE IF NOT EXISTS auth_terminal_operation_fences (
          family TEXT NOT NULL, operation_id UUID NOT NULL, owner_handle UUID NOT NULL,
          deletion_cycle_id UUID NOT NULL, completed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
          PRIMARY KEY(family,operation_id))
        """);
    UUID intent = UUID.randomUUID();
    jdbc.update("""
        INSERT INTO auth_terminal_operation_fences(family,operation_id,owner_handle,deletion_cycle_id)
        VALUES ('sdk_registration',:intent,:owner,:cycle)
        """, Map.of("intent", intent, "owner", UUID.randomUUID(), "cycle", UUID.randomUUID()));
    var accounts = new JdbcAccountRepository(jdbc);
    var floors = mock(SessionEpochFloorStore.class);
    when(floors.recordAtLeast(any(UUID.class), anyLong())).thenAnswer(call -> call.getArgument(1));
    var refresh = mock(RefreshTokenRepository.class);
    var jwt = mock(JwtService.class);
    var profiles = mock(PrimaryProfileProvisioner.class);
    var hasher = mock(BCryptPasswordHasher.class);
    when(hasher.hash("strong-password")).thenReturn("test-password-hash");
    var auth = new AuthService(accounts, refresh, null, hasher, jwt, null, null, null,
        Clock.systemUTC(), Duration.ofDays(30), profiles, null, null, null, null, null, null, null, null, floors);
    auth.configureRegistrationSessionEpochPreparer(new RegistrationSessionEpochPreparer(
        new TransactionTemplate(new DataSourceTransactionManager(database)), accounts,
        new SessionEpochIssuanceGate(accounts, floors), new JdbcSdkRegistrationIntentBinder(jdbc, Clock.systemUTC())));
    assertThatThrownBy(() -> auth.register(new RegisterCommand(
        "retry@example.test", null, "strong-password", false, "{}", intent)))
        .isInstanceOf(AuthException.class).hasMessage("operation_erased");
    // The epoch preparation proves account creation happened inside the rolled-back transaction.
    verify(floors).recordAtLeast(any(UUID.class), anyLong());
    assertThat(jdbc.queryForObject("SELECT count(*) FROM accounts WHERE email=:email",
        Map.of("email", "retry@example.test"), Long.class)).isZero();
    assertThat(jdbc.getJdbcTemplate().queryForObject("SELECT count(*) FROM refresh_tokens", Long.class)).isZero();
    verifyNoInteractions(refresh, jwt, profiles);
  }
}
