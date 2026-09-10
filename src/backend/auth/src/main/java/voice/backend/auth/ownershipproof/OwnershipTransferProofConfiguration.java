package voice.backend.auth.ownershipproof;

import java.time.Clock;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.service.BackupCodeService;
import voice.backend.auth.service.TotpService;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

/** Proofs require durable atomic storage; memory profiles have no usable proof endpoint. */
@Configuration(proxyBeanMethods = false)
@ConditionalOnProperty(prefix = "auth", name = "persistence", havingValue = "jdbc")
public class OwnershipTransferProofConfiguration {
  @Bean
  OwnershipTransferProofService ownershipTransferProofService(NamedParameterJdbcTemplate jdbc,
      PlatformTransactionManager manager, BCryptPasswordHasher passwords, TotpService totp,
      BackupCodeService backup, SessionEpochFloorStore floors, Clock clock) {
    return new OwnershipTransferProofService(new JdbcOwnershipTransferProofStore(jdbc, manager),
        passwords, totp, backup, floors, clock);
  }
}
