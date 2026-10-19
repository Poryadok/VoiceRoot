package voice.backend.auth.roleidentityauthority;

import static org.junit.jupiter.api.Assertions.*;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Arrays;
import java.util.List;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.mock.env.MockEnvironment;

class AuthIdentitySelectorSettingsTest {
  static final String PREFIX = "AUTH_ROLE_IDENTITY_SELECTOR_";
  @TempDir Path directory;
  MockEnvironment configured() {
    return new MockEnvironment().withProperty(PREFIX + "KEYS_DIR", directory.toString())
        .withProperty(PREFIX + "KEYSET_REVISION", "2").withProperty(PREFIX + "ACTIVE_VERSION", "2");
  }
  void key(int version, byte value) throws Exception {
    byte[] bytes = new byte[32]; Arrays.fill(bytes, value);
    Files.write(directory.resolve("v" + version + ".key"), bytes);
  }
  @Test void absentSettingsAreDisabledAndPartialSettingsFail() {
    assertNull(AuthIdentitySelectorSettings.load(new MockEnvironment()).selectors());
    for (String suffix : List.of("KEYS_DIR", "KEYSET_REVISION", "ACTIVE_VERSION")) {
      assertThrows(IllegalArgumentException.class, () -> AuthIdentitySelectorSettings.load(
          new MockEnvironment().withProperty(PREFIX + suffix, "present")));
    }
  }
  @Test void loadsAllVersionsIntoImmutableLocalSnapshot() throws Exception {
    key(1, (byte) 31); key(2, (byte) 32);
    var selectors = AuthIdentitySelectorSettings.load(configured()).selectors();
    assertEquals(2, selectors.keysetRevision()); assertEquals(2, selectors.activeVersion());
    assertEquals(List.of(1, 2), selectors.retainedVersions());
    UUID account = UUID.randomUUID(); byte[] expected = selectors.activeAccount(account);
    key(2, (byte) 33);
    assertArrayEquals(expected, selectors.activeAccount(account));
  }
  @Test void refusesMissingActiveVersionEmptyOrMalformedKeyMaterial() throws Exception {
    assertThrows(IllegalArgumentException.class, () -> AuthIdentitySelectorSettings.load(configured()));
    key(1, (byte) 1);
    assertThrows(IllegalArgumentException.class, () -> AuthIdentitySelectorSettings.load(configured()));
    for (byte[] bytes : List.of(new byte[32], new byte[31], new byte[33])) {
      Files.write(directory.resolve("v2.key"), bytes);
      assertThrows(IllegalArgumentException.class, () -> AuthIdentitySelectorSettings.load(configured()));
    }
    key(2, (byte) 1);
    assertThrows(IllegalArgumentException.class, () -> AuthIdentitySelectorSettings.load(configured()));
  }
  @Test void refusesAmbiguousFileNamesAndNoncanonicalVersions() throws Exception {
    key(1, (byte) 31); key(2, (byte) 32);
    Files.write(directory.resolve("v02.key"), new byte[32]);
    assertThrows(IllegalArgumentException.class, () -> AuthIdentitySelectorSettings.load(configured()));
    Files.delete(directory.resolve("v02.key"));
    for (String suffix : List.of("KEYSET_REVISION", "ACTIVE_VERSION")) {
      for (String value : List.of("", "0", "-1", "01", "999999999999999999999999")) {
        var environment = configured(); environment.setProperty(PREFIX + suffix, value);
        assertThrows(IllegalArgumentException.class, () -> AuthIdentitySelectorSettings.load(environment));
      }
    }
  }
}
