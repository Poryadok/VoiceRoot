package voice.backend.auth.config;

import org.springframework.context.annotation.Condition;
import org.springframework.context.annotation.ConditionContext;
import org.springframework.core.type.AnnotatedTypeMetadata;

/** Matches when Auth uses JDBC persistence and a second datasource URL for {@code user_db} is set. */
public final class JdbcPersistenceWithUserDbUrlCondition implements Condition {
  @Override
  public boolean matches(ConditionContext context, AnnotatedTypeMetadata metadata) {
    String persistence = context.getEnvironment().getProperty("auth.persistence", "jdbc");
    if (!"jdbc".equalsIgnoreCase(persistence)) {
      return false;
    }
    String url = context.getEnvironment().getProperty("auth.user-db.jdbc-url");
    return url != null && !url.isBlank();
  }
}
