package voice.backend.auth.config;

import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import voice.backend.auth.userdb.InMemoryPrimaryProfileProvisioner;
import voice.backend.auth.userdb.JdbcPrimaryProfileProvisioner;
import voice.backend.auth.userdb.JdbcProfileSwitchValidator;
import voice.backend.auth.userdb.JdbcUserVerificationSync;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;
import voice.backend.auth.userdb.ProfileSwitchValidator;
import voice.backend.auth.service.LinkedAccountsService;

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
  ProfileSwitchValidator profileSwitchValidator(
      @Autowired(required = false) @Qualifier("userJdbc") NamedParameterJdbcTemplate userJdbc) {
    if (userJdbc == null) {
      throw new IllegalStateException("userJdbc required for profile switch validation");
    }
    return new JdbcProfileSwitchValidator(userJdbc);
  }

  @Bean
  JdbcUserVerificationSync userVerificationSync(
      @Autowired(required = false) @Qualifier("userJdbc") NamedParameterJdbcTemplate userJdbc) {
    if (userJdbc == null) {
      throw new IllegalStateException("userJdbc required for verification sync");
    }
    return new JdbcUserVerificationSync(userJdbc);
  }

  @Bean
  LinkedAccountsService linkedAccountsService(
      JdbcUserVerificationSync verificationSync, AuthProperties properties) {
    return new LinkedAccountsService(verificationSync, properties.getOauth().getTwitchApiBaseUrl());
  }
}
