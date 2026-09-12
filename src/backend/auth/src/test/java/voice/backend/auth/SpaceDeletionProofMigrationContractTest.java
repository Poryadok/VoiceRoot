package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import org.junit.jupiter.api.Test;

public class SpaceDeletionProofMigrationContractTest {
  static final String FLYWAY_NAME = "V13__space_deletion_proofs.sql";
  static final String GOLANG_UP_NAME = "000014_space_deletion_proofs.up.sql";
  static final String GOLANG_DOWN_NAME = "000014_space_deletion_proofs.down.sql";

  @Test
  void bothSupportedAuthMigrationCatalogsContainTheSeparateDeletionProofRevision()
      throws IOException {
    Path flyway = authRoot().resolve("src/main/resources/db/migration").resolve(FLYWAY_NAME);
    Path up = repositoryRoot().resolve("src/backend/migrations/auth_db").resolve(GOLANG_UP_NAME);
    Path down = repositoryRoot().resolve("src/backend/migrations/auth_db").resolve(GOLANG_DOWN_NAME);

    assertThat(flyway).as("Flyway V13 deletion-proof migration").isRegularFile();
    assertThat(up).as("golang-migrate 000014 UP mirror").isRegularFile();
    assertThat(down).as("golang-migrate 000014 guarded DOWN").isRegularFile();

    String flywaySql = normalize(Files.readString(flyway));
    String upSql = normalize(Files.readString(up));
    String downSql = normalize(Files.readString(down)).toLowerCase();

    assertThat(upSql).isEqualTo(flywaySql);
    assertThat(flywaySql.toLowerCase())
        .contains("create table space_deletion_proofs")
        .contains("security_revision")
        .contains("expires_at")
        .contains("consumed_at")
        .contains("acknowledged_at")
        .contains("receipt_bytes")
        .contains("receipt_sha256")
        .contains("confirmation_name_sha256")
        .contains("proof_digest_sha256")
        .contains("binding_bytes")
        .contains("receipt_lookup_hmac")
        .contains("receipt_hmac_key_version")
        .doesNotContain("confirmation_name text")
        .doesNotContain("proof text");
    assertThat(downSql)
        .contains("lock table space_deletion_proofs in access exclusive mode")
        .contains("errcode = '55000'")
        .contains("consumed_at")
        .contains("receipt_bytes")
        .contains("drop table space_deletion_proofs");
  }

  private static String normalize(String value) {
    return value.replace("\r\n", "\n").strip() + "\n";
  }

  public static Path authRoot() {
    Path current = Path.of("").toAbsolutePath().normalize();
    if (Files.isRegularFile(current.resolve("pom.xml"))
        && Files.isDirectory(current.resolve("src/main/resources/db/migration"))) {
      return current;
    }
    Path candidate = current.resolve("src/backend/auth");
    if (Files.isRegularFile(candidate.resolve("pom.xml"))) {
      return candidate;
    }
    throw new IllegalStateException("cannot locate Auth project from " + current);
  }

  public static Path repositoryRoot() {
    Path candidate = authRoot();
    while (candidate != null) {
      if (Files.isDirectory(candidate.resolve("src/backend/migrations/auth_db"))) {
        return candidate;
      }
      candidate = candidate.getParent();
    }
    throw new IllegalStateException("cannot locate Voice repository");
  }
}
