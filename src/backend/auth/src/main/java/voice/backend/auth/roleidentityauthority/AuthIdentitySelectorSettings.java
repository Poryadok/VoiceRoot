package voice.backend.auth.roleidentityauthority;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.LinkedHashMap;
import java.util.List;
import org.springframework.core.env.Environment;

/** One immutable local owner-key snapshot, loaded before any account transaction. */
public record AuthIdentitySelectorSettings(RoleIdentitySelectors selectors) {
  private static final String PREFIX = "AUTH_ROLE_IDENTITY_SELECTOR_";
  private static final List<String> SETTINGS = List.of("KEYS_DIR", "KEYSET_REVISION", "ACTIVE_VERSION");

  public static AuthIdentitySelectorSettings load(Environment environment) {
    try {
      if (SETTINGS.stream().noneMatch(name -> environment.containsProperty(PREFIX + name))) {
        return new AuthIdentitySelectorSettings(null);
      }
      for (String name : SETTINGS) {
        if (environment.getProperty(PREFIX + name, "").isBlank()) throw new IllegalArgumentException();
      }
      long revision = positive(environment.getProperty(PREFIX + "KEYSET_REVISION"));
      int active = Math.toIntExact(positive(environment.getProperty(PREFIX + "ACTIVE_VERSION")));
      Path directory = Path.of(environment.getProperty(PREFIX + "KEYS_DIR"));
      if (!Files.isDirectory(directory)) throw new IllegalArgumentException();
      var keys = new LinkedHashMap<Integer, byte[]>();
      try (var files = Files.list(directory)) {
        for (Path file : files.toList()) {
          // Secret-volume metadata directories are not key entries; projected
          // key-file symlinks are read as regular files and validated normally.
          if (Files.isDirectory(file)) continue;
          var name = java.util.regex.Pattern.compile("v([1-9][0-9]*)\\.key").matcher(file.getFileName().toString());
          if (!name.matches() || !Files.isRegularFile(file) || Files.size(file) != 32) {
            throw new IllegalArgumentException();
          }
          int version = Math.toIntExact(positive(name.group(1)));
          if (keys.putIfAbsent(version, Files.readAllBytes(file)) != null) throw new IllegalArgumentException();
        }
      }
      return new AuthIdentitySelectorSettings(new RoleIdentitySelectors(revision, active, keys));
    } catch (Exception ex) {
      throw new IllegalArgumentException("invalid Auth identity selector configuration");
    }
  }

  private static long positive(String value) {
    if (value == null || !value.matches("[1-9][0-9]*")) throw new IllegalArgumentException();
    return Long.parseLong(value);
  }
}
