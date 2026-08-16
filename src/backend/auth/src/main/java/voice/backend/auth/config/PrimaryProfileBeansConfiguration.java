package voice.backend.auth.config;

import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.boot.autoconfigure.condition.ConditionalOnExpression;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
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
import voice.backend.auth.userdb.JdbcPhoneHashResolver;
import voice.backend.auth.userdb.JdbcPrimaryProfileProvisioner;
import voice.backend.auth.userdb.JdbcProfileSwitchValidator;
import voice.backend.auth.userdb.JdbcUserVerificationSync;
import voice.backend.auth.userdb.NoOpProfileSwitchValidator;
import voice.backend.auth.userdb.NoOpUserVerificationSync;
import voice.backend.auth.userdb.PhoneHashResolver;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;
import voice.backend.auth.userdb.ProfileSwitchValidator;
import voice.backend.auth.userdb.UserVerificationSync;

@Configuration
public class PrimaryProfileBeansConfiguration {

  @Bean
  PrimaryProfileProvisioner primaryProfileProvisioner(
      AuthProperties props,
      @Autowired(required = false) @Qualifier("userJdbc") NamedParameterJdbcTemplate userJdbc) {
    if (props.getPersistence() == AuthProperties.PersistenceMode.MEMORY) {
      return new InMemoryPrimaryProfileProvisioner();
    }
    if (userJdbc == null) {
      throw new IllegalStateException(
          "auth.user-db.jdbc-url is required when auth.persistence=jdbc (see docs/EXEC_PLAN.md)");
    }
    return new JdbcPrimaryProfileProvisioner(userJdbc);
  }

  @Bean
  PhoneHashResolver phoneHashResolver(
      AuthProperties props,
      AccountRepository accounts,
      PrimaryProfileProvisioner primaryProfileProvisioner,
      NamedParameterJdbcTemplate authJdbc,
      @Autowired(required = false) @Qualifier("userJdbc") NamedParameterJdbcTemplate userJdbc) {
    if (props.getPersistence() == AuthProperties.PersistenceMode.MEMORY) {
      return new InMemoryPhoneHashResolver(accounts, primaryProfileProvisioner);
    }
    if (userJdbc == null) {
      throw new IllegalStateException("userJdbc required for phone hash resolution");
    }
    return new JdbcPhoneHashResolver(authJdbc, userJdbc);
  }

  @Bean
  ProfileSwitchValidator profileSwitchValidator(
      AuthProperties props,
      @Autowired(required = false) @Qualifier("userJdbc") NamedParameterJdbcTemplate userJdbc) {
    if (props.getPersistence() == AuthProperties.PersistenceMode.MEMORY) {
      return new NoOpProfileSwitchValidator();
    }
    if (userJdbc == null) {
      throw new IllegalStateException("userJdbc required for profile switch validation");
    }
    return new JdbcProfileSwitchValidator(userJdbc);
  }

  @Bean
  @ConditionalOnExpression("'${auth.user-grpc.addr:}'.blank")
  @ConditionalOnMissingBean(UserVerificationSync.class)
  UserVerificationSync jdbcUserVerificationSync(
      AuthProperties props,
      @Autowired(required = false) @Qualifier("userJdbc") NamedParameterJdbcTemplate userJdbc) {
    if (props.getPersistence() == AuthProperties.PersistenceMode.MEMORY) {
      return new NoOpUserVerificationSync();
    }
    if (userJdbc == null) {
      throw new IllegalStateException("userJdbc required for verification sync");
    }
    return new JdbcUserVerificationSync(userJdbc);
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
