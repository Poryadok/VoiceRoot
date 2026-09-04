package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;

import java.nio.file.Files;
import java.nio.file.Path;
import org.junit.jupiter.api.Test;

/**
 * T-049b RED contract for the Auth-owned durable guest-conversion operation.
 *
 * <p>{@code docs/features/auth-and-contacts.md} requires guest conversion to become regular only
 * after email verification and to publish {@code user.guest_converted}. The A1 T-049b plan further
 * requires one persistent operation per account, with a stable operation/event identity and retry
 * metadata, on both supported Auth schema paths. This deliberately checks migrations rather than
 * production implementation so that an incomplete deployment path cannot silently lose pending
 * conversion work.
 */
class GuestConversionDurabilityMigrationContractTest {
  private static final String FLYWAY_MIGRATION = "V8__guest_conversion_operations.sql";
  private static final String GOLANG_MIGRATION = "000009_guest_conversion_operations.up.sql";

  @Test
  void flywayMigrationPersistsOneRetryableGuestConversionOperationPerAccount() throws Exception {
    Path migration =
        authProjectRoot().resolve("src/main/resources/db/migration").resolve(FLYWAY_MIGRATION);

    assertThat(migration)
        .as("Flyway must create the Auth-owned durable guest conversion operation table")
        .exists();

    String sql = Files.readString(migration).toLowerCase();
    assertThat(sql).contains("create table", "guest_conversion_operations");
    assertThat(sql).contains("operation_id", "event_id", "account_id", "state");
    assertThat(sql)
        .as("one account must resume its existing operation, never create a second one")
        .contains("unique", "account_id");
    assertThat(sql)
        .as("a failed User/Auth/NATS step must remain leasable and retryable after restart")
        .containsAnyOf("lease", "retry", "attempt");
  }

  @Test
  void golangMigratePathStaysInLockstepWithFlywayDurabilityMigration() throws Exception {
    Path migration =
        repositoryRoot().resolve("src/backend/migrations/auth_db").resolve(GOLANG_MIGRATION);

    assertThat(migration)
        .as("the optional golang-migrate auth_db path must retain the same pending conversion state")
        .exists();

    String sql = Files.readString(migration).toLowerCase();
    assertThat(sql).contains("create table", "guest_conversion_operations");
    assertThat(sql).contains("operation_id", "event_id", "account_id", "state");
    assertThat(sql).contains("unique", "account_id");
    assertThat(sql).containsAnyOf("lease", "retry", "attempt");
  }

  private static Path authProjectRoot() {
    return repositoryRoot().resolve("src/backend/auth");
  }

  private static Path repositoryRoot() {
    for (Path candidate = Path.of("").toAbsolutePath();
        candidate != null;
        candidate = candidate.getParent()) {
      if (Files.isDirectory(candidate.resolve("src/backend/auth"))
          && Files.isDirectory(candidate.resolve("src/backend/migrations/auth_db"))) {
        return candidate;
      }
    }
    throw new IllegalStateException(
        "Voice repository root was not found from the Maven working directory");
  }
}
