package voice.backend.auth.config;

import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.autoconfigure.condition.ConditionalOnExpression;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import voice.backend.auth.lifecycle.VerificationStatusRefresh;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.InMemoryLinkedIdentityRepository;
import voice.backend.auth.repository.JdbcLinkedIdentityRepository;
import voice.backend.auth.repository.LinkedIdentityRepository;
import voice.backend.auth.service.LinkedAccountsService;
import voice.backend.auth.userdb.InMemoryPhoneHashResolver;
import voice.backend.auth.userdb.InMemoryPrimaryProfileProvisioner;
import voice.backend.auth.userdb.NoOpProfileSwitchValidator;
import voice.backend.auth.userdb.NoOpUserVerificationSync;
import voice.backend.auth.userdb.PhoneHashResolver;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;
import voice.backend.auth.userdb.ProfileSwitchValidator;
import voice.backend.auth.userdb.UserVerificationSync;

@Configuration
public class PrimaryProfileBeansConfiguration {

  @Bean
  @ConditionalOnProperty(prefix = "auth", name = "persistence", havingValue = "memory")
  @ConditionalOnMissingBean(PrimaryProfileProvisioner.class)
  PrimaryProfileProvisioner primaryProfileProvisioner() {
    return new InMemoryPrimaryProfileProvisioner();
  }

  @Bean
  @ConditionalOnProperty(prefix = "auth", name = "persistence", havingValue = "memory")
  @ConditionalOnMissingBean(PhoneHashResolver.class)
  PhoneHashResolver phoneHashResolver(
      AccountRepository accounts,
      PrimaryProfileProvisioner primaryProfileProvisioner) {
    return new InMemoryPhoneHashResolver(accounts, primaryProfileProvisioner);
  }

  @Bean
  @ConditionalOnProperty(prefix = "auth", name = "persistence", havingValue = "memory")
  @ConditionalOnMissingBean(ProfileSwitchValidator.class)
  ProfileSwitchValidator profileSwitchValidator() {
    return new NoOpProfileSwitchValidator();
  }

  @Bean
  @ConditionalOnExpression("'${auth.user-grpc.addr:}'.blank")
  @ConditionalOnMissingBean(UserVerificationSync.class)
  @ConditionalOnProperty(prefix = "auth", name = "persistence", havingValue = "memory")
  UserVerificationSync memoryUserVerificationSync() {
    return new NoOpUserVerificationSync();
  }

  @Bean
  LinkedIdentityRepository linkedIdentityRepository(
      AuthProperties props,
      @Autowired(required = false) NamedParameterJdbcTemplate authJdbc) {
    if (props.getPersistence() == AuthProperties.PersistenceMode.MEMORY) {
      return new InMemoryLinkedIdentityRepository();
    }
    if (authJdbc == null) {
      throw new IllegalStateException("authJdbc required for linked identities");
    }
    return new JdbcLinkedIdentityRepository(authJdbc);
  }

  @Bean
  LinkedAccountsService linkedAccountsService(
      UserVerificationSync verificationSync,
      LinkedIdentityRepository linkedIdentityRepository,
      AuthProperties properties) {
    return new LinkedAccountsService(
        verificationSync, linkedIdentityRepository, properties.getOauth());
  }

  @Bean
  VerificationStatusRefresh verificationStatusRefresh(LinkedAccountsService linkedAccountsService) {
    return new VerificationStatusRefresh(linkedAccountsService);
  }
}
