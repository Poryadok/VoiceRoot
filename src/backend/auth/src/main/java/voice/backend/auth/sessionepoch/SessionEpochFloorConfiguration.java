package voice.backend.auth.sessionepoch;

import java.time.Duration;
import org.springframework.beans.factory.SmartInitializingSingleton;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.boot.sql.init.dependency.DependsOnDatabaseInitialization;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.data.redis.core.StringRedisTemplate;
import voice.backend.auth.config.AuthProperties;
import voice.backend.auth.repository.AccountRepository;

/** Wires the shared epoch floor to Redis in production and an equivalent monotonic store in tests. */
@Configuration
public class SessionEpochFloorConfiguration {
  @Bean
  @ConditionalOnProperty(prefix = "auth", name = "persistence", havingValue = "jdbc")
  SessionEpochFloorStore redisSessionEpochFloorStore(StringRedisTemplate redis) {
    return new RedisSessionEpochFloorStore(
        new StringRedisSessionEpochCommands(redis), Duration.ofSeconds(2));
  }

  @Bean
  @ConditionalOnProperty(prefix = "auth", name = "persistence", havingValue = "jdbc")
  DurableAccountEpochSource repositoryDurableAccountEpochSource(AccountRepository accounts) {
    return new RepositoryDurableAccountEpochSource(accounts);
  }

  @Bean
  @ConditionalOnProperty(prefix = "auth", name = "persistence", havingValue = "jdbc")
  AuthSessionEpochFloorReconciler authSessionEpochFloorReconciler(
      DurableAccountEpochSource durableEpochs, SessionEpochFloorStore floors, AuthProperties properties) {
    return new AuthSessionEpochFloorReconciler(
        durableEpochs, floors, properties.getSessionEpoch().getSeed().getPageSize());
  }

  @Bean
  @ConditionalOnProperty(prefix = "auth", name = "persistence", havingValue = "jdbc")
  @DependsOnDatabaseInitialization
  SmartInitializingSingleton sessionEpochFloorSeeder(AuthSessionEpochFloorReconciler reconciler) {
    return reconciler::seedAndReconcile;
  }

  @Bean
  @ConditionalOnProperty(prefix = "auth", name = "persistence", havingValue = "memory")
  SessionEpochFloorStore memorySessionEpochFloorStore() {
    return new InMemorySessionEpochFloorStore();
  }
}
