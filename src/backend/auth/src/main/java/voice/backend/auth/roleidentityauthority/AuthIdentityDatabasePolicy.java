package voice.backend.auth.roleidentityauthority;

import org.springframework.core.env.Environment;

/** Explicit trusted installation identity and the already supported mutually exclusive loader. */
public record AuthIdentityDatabasePolicy(String schemaOwner, Loader loader) {
  public enum Loader { FLYWAY, CANONICAL }
  public AuthIdentityDatabasePolicy {
    if (schemaOwner == null || schemaOwner.isBlank() || schemaOwner.indexOf('\0') >= 0
        || schemaOwner.getBytes(java.nio.charset.StandardCharsets.UTF_8).length > 63 || loader == null) {
      throw new IllegalArgumentException("identity database policy unavailable");
    }
  }
  static AuthIdentityDatabasePolicy load(Environment environment) {
    String enabled = environment.getProperty("spring.flyway.enabled", "true");
    if (!enabled.equals("true") && !enabled.equals("false")) throw new IllegalArgumentException("invalid Auth migration loader");
    return new AuthIdentityDatabasePolicy(environment.getProperty("AUTH_IDENTITY_SCHEMA_OWNER"),
        enabled.equals("true") ? Loader.FLYWAY : Loader.CANONICAL);
  }
}
