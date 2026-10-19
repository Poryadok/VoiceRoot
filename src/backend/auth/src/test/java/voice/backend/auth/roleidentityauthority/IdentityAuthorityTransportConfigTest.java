package voice.backend.auth.roleidentityauthority;

import static org.junit.jupiter.api.Assertions.*;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.mock.env.MockEnvironment;

class IdentityAuthorityTransportConfigTest {
  @TempDir Path directory;
  MockEnvironment configured() throws Exception {
    var environment = new MockEnvironment();
    environment.setProperty("AUTH_IDENTITY_AUTHORITY_ENABLED", "true");
    for (String suffix : List.of("TLS_CERT_FILE", "TLS_KEY_FILE", "TLS_CLIENT_CA_FILE")) {
      Path file = directory.resolve(suffix); Files.writeString(file, "fixture parsed by TLS builder later");
      environment.setProperty("AUTH_IDENTITY_AUTHORITY_" + suffix, file.toString());
    }
    environment.setProperty("S2S_JWKS_URLS_JSON", "{\"role\":\"https://role.internal/keys\",\"user\":\"https://user.internal/keys\"}");
    return environment;
  }
  @Test void absentFeatureIsDisabledButPartialConfigurationCannotSilentlyDisable() {
    assertNull(IdentityAuthorityTransportConfig.load(new MockEnvironment()));
    for (String suffix : List.of("GRPC_LISTEN", "TLS_CERT_FILE", "JWKS_CA_FILE")) {
      var environment = new MockEnvironment().withProperty("AUTH_IDENTITY_AUTHORITY_" + suffix, "present");
      assertThrows(IllegalArgumentException.class, () -> IdentityAuthorityTransportConfig.load(environment));
    }
  }
  @Test void exactRoleAndUserHttpsTrustDoesNotBorrowUnrelatedIssuer() throws Exception {
    var config = IdentityAuthorityTransportConfig.load(configured());
    assertEquals(9094, config.listen().getPort());
    assertEquals(java.util.Set.of("role", "user"), config.jwks().keySet());
    assertNull(config.jwksCa());
  }
  @Test void incompleteTlsOrIssuerConfigurationFailsBeforeBinding() throws Exception {
    for (String suffix : List.of("TLS_CERT_FILE", "TLS_KEY_FILE", "TLS_CLIENT_CA_FILE")) {
      var environment = configured(); environment.setProperty("AUTH_IDENTITY_AUTHORITY_" + suffix, "");
      assertThrows(IllegalArgumentException.class, () -> IdentityAuthorityTransportConfig.load(environment));
    }
    for (String raw : List.of("{}", "{\"role\":\"https://role/keys\"}",
        "{\"role\":\"http://role/keys\",\"user\":\"https://user/keys\"}",
        "{\"role\":\"https://role/keys\",\"user\":\"https://secret@user/keys\"}",
        "{\"role\":\"https://role/keys\",\"user\":\"https://user/keys#fragment\"}",
        "{\"role\":\"https://role/keys\",\"role\":\"https://other/keys\",\"user\":\"https://user/keys\"}")) {
      var environment = configured(); environment.setProperty("S2S_JWKS_URLS_JSON", raw);
      assertThrows(IllegalArgumentException.class, () -> IdentityAuthorityTransportConfig.load(environment));
    }
  }
  @Test void portAndCachePolicyAreBoundedAndEphemeralPortIsTestOnly() throws Exception {
    for (String address : List.of(":-1", ":65536", ":0", "invalid", "http://auth:9094")) {
      var environment = configured(); environment.setProperty("AUTH_IDENTITY_AUTHORITY_GRPC_LISTEN", address);
      assertThrows(IllegalArgumentException.class, () -> IdentityAuthorityTransportConfig.load(environment));
    }
    var environment = configured(); environment.setActiveProfiles("test");
    environment.setProperty("AUTH_IDENTITY_AUTHORITY_GRPC_LISTEN", "127.0.0.1:0");
    assertEquals(0, IdentityAuthorityTransportConfig.load(environment).listen().getPort());
    environment.setProperty("S2S_JWKS_REFRESH_AFTER", "2m");
    environment.setProperty("S2S_JWKS_HARD_EXPIRY", "1m");
    assertThrows(IllegalArgumentException.class, () -> IdentityAuthorityTransportConfig.load(environment));
  }
}
