package voice.backend.auth.spacedeletionproof;

import java.time.Clock;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.service.BackupCodeService;
import voice.backend.auth.service.TotpService;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

/** Deletion proofs require durable JDBC storage; key-backed erasure fails closed if unconfigured. */
@Configuration(proxyBeanMethods = false)
@ConditionalOnProperty(prefix = "auth", name = "persistence", havingValue = "jdbc")
public class SpaceDeletionProofConfiguration {
  @Bean
  SpaceDeletionProofService spaceDeletionProofService(
      NamedParameterJdbcTemplate jdbc,
      PlatformTransactionManager manager,
      BCryptPasswordHasher passwords,
      TotpService totp,
      BackupCodeService backup,
      SessionEpochFloorStore floors,
      Clock clock,
      ObjectProvider<ReceiptErasureKeyring> keyrings) {
    ReceiptErasureKeyring keyring = keyrings.getIfAvailable(ReceiptErasureKeyring::unavailable);
    return new SpaceDeletionProofService(
        new JdbcSpaceDeletionProofStore(jdbc, manager, keyring),
        passwords, totp, backup, floors, clock);
  }
}
