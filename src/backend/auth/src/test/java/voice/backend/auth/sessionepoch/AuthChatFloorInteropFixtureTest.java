package voice.backend.auth.sessionepoch;

import static org.junit.jupiter.api.Assertions.assertThrows;

import java.nio.file.Path;
import java.util.Map;
import java.nio.file.Files;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class AuthChatFloorInteropFixtureTest {
  @TempDir Path output;

  @Test void missingProductionDependenciesMustNeverBeAccepted() {
    assertThrows(IllegalArgumentException.class,
        () -> AuthChatFloorInteropFixture.validateInputs(Map.of(), output, 30));
  }

  @Test void rejectsMalformedInputsBeforeReadiness() throws Exception {
    try (var material = new ChatFloorFixtureTestMaterial(output)) {
      Map<String, String> valid = material.environment();
      for (int lifetime : new int[] {0, -1, 601})
        assertThrows(IllegalArgumentException.class,
            () -> AuthChatFloorInteropFixture.validateInputs(valid, output, lifetime));
      for (String field : new String[] {"RUN_ID", "AUTH_SOURCE_SHA", "LAUNCHER_SOURCE_SHA", "DURABLE_EPOCH", "RETAINED_EPOCH", "REDIS_PORT"}) {
        var changed = new java.util.HashMap<>(valid);
        changed.put(AuthChatFloorInteropFixture.PREFIX + field, "0");
        assertThrows(IllegalArgumentException.class,
            () -> AuthChatFloorInteropFixture.validateInputs(changed, output, 30), field);
      }
      org.junit.jupiter.api.Assertions.assertFalse(Files.exists(output.resolve("ready.json")));
    }
  }

  @Test void rejectsCredentialPathEscapeAndExistingReady() throws Exception {
    try (var material = new ChatFloorFixtureTestMaterial(output)) {
      var valid = material.environment();
      AuthChatFloorInteropFixture.validateInputs(valid, output, 30);
      Path outside = Files.createTempFile("outside-chat-fixture", ".pem");
      try {
        var escaped = new java.util.HashMap<>(valid);
        escaped.put(AuthChatFloorInteropFixture.PREFIX + "CLIENT_KEY_FILE", outside.toString());
        assertThrows(IllegalArgumentException.class,
            () -> AuthChatFloorInteropFixture.validateInputs(escaped, output, 30));
        Path link = output.resolve("escape.pem");
        Files.createSymbolicLink(link, outside);
        escaped.put(AuthChatFloorInteropFixture.PREFIX + "CLIENT_KEY_FILE", link.toString());
        assertThrows(IllegalArgumentException.class,
            () -> AuthChatFloorInteropFixture.validateInputs(escaped, output, 30));
      } finally { Files.deleteIfExists(outside); }
      Files.writeString(output.resolve("ready.json"), "foreign readiness");
      assertThrows(IllegalArgumentException.class,
          () -> AuthChatFloorInteropFixture.validateInputs(valid, output, 30));
    }
  }

  @Test void unavailableDatabaseDoesNotPublishReadiness() throws Exception {
    try (var material = new ChatFloorFixtureTestMaterial(output)) {
      var values = new java.util.HashMap<>(material.environment());
      values.put(AuthChatFloorInteropFixture.PREFIX + "JDBC_URL", "jdbc:postgresql://127.0.0.1:1/unavailable");
      var config = AuthChatFloorInteropFixture.Config.read(values, output, 30);
      assertThrows(IllegalStateException.class, () -> AuthChatFloorInteropFixture.start(config));
      org.junit.jupiter.api.Assertions.assertFalse(Files.exists(output.resolve("ready.json")));
      org.junit.jupiter.api.Assertions.assertTrue(Files.exists(output.resolve("closed.json")));
    }
  }
}
