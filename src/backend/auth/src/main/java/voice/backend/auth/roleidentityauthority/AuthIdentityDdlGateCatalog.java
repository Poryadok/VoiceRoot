package voice.backend.auth.roleidentityauthority;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.HashSet;
import java.util.HexFormat;
import java.util.Set;
import org.springframework.jdbc.core.JdbcTemplate;

/** Metadata is checked only AFTER the caller's direct barrier acquisition. */
final class AuthIdentityDdlGateCatalog {
  private static final ObjectMapper JSON = new ObjectMapper();
  static final JsonNode EXPECTED = json("ddl-barrier-v1.json");
  static final String QUERY = text("gate-catalog.sql");
  private AuthIdentityDdlGateCatalog() {}

  static void validate(JdbcTemplate jdbc, AuthIdentityDatabasePolicy policy) {
    require(policy != null);
    try {
      validateMetadata(JSON.readTree(jdbc.queryForObject(QUERY, String.class)), policy.schemaOwner(), false);
    } catch (Exception failure) { throw unavailable(); }
  }

  static void validateMetadata(JsonNode actual, String owner, boolean maintenance) {
    require(actual != null && actual.isObject() && owner != null && !owner.isBlank());
    for (String name : Set.of("barrier_owner", "function_owner", "event_owner")) {
      require(actual.path(name).isTextual() && owner.equals(actual.path(name).asText()));
    }
    for (String name : Set.of("relation_ok", "acl_ok", "function_ok", "binding_ok", "major_ok")) {
      require(actual.path(name).isBoolean() && actual.path(name).booleanValue());
    }
    if (!maintenance) require(actual.path("runtime_ok").isBoolean() && actual.path("runtime_ok").booleanValue());
    String enabled = actual.path("event_enabled").asText();
    require(enabled.equals("O") || (maintenance && enabled.equals("D")));
    require(actual.path("event_tags").isArray());
    Set<String> tags = new HashSet<>();
    for (var tag : actual.path("event_tags")) require(tag.isTextual() && tags.add(tag.asText()));
    Set<String> expected = new HashSet<>();
    EXPECTED.path("tags").forEach(tag -> expected.add(tag.asText()));
    require(tags.equals(expected));
    try {
      require(actual.path("function_source").isTextual());
      byte[] source = actual.path("function_source").asText().replace("\r\n", "\n").getBytes(StandardCharsets.UTF_8);
      require(EXPECTED.path("sourceSha256").asText().equals(HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(source))));
    } catch (Exception failure) { throw unavailable(); }
  }

  static JsonNode json(String name) {
    try { return JSON.readTree(text(name)); } catch (Exception failure) { throw unavailable(); }
  }
  static String text(String name) {
    try (var input = AuthIdentityDdlGateCatalog.class.getResourceAsStream("/roleidentityauthority/" + name)) {
      if (input == null) throw unavailable();
      return new String(input.readAllBytes(), StandardCharsets.UTF_8);
    } catch (Exception failure) { throw unavailable(); }
  }
  static void require(boolean valid) { if (!valid) throw unavailable(); }
  static IllegalStateException unavailable() { return new IllegalStateException("identity DDL catalog unavailable"); }
}
