package voice.backend.auth.config;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.charset.StandardCharsets;
import org.springframework.core.io.Resource;
import org.springframework.core.io.ResourceLoader;

public final class JwtPrivateKeyLoader {
  private JwtPrivateKeyLoader() {}

  public static String loadPem(AuthProperties.Jwt jwt, ResourceLoader resourceLoader) {
    String inline = jwt.getPrivateKeyPem();
    if (inline != null && !inline.isBlank()) {
      return inline;
    }
    String location = jwt.getPrivateKeyLocation();
    if (location == null || location.isBlank()) {
      return "";
    }
    Resource resource = resourceLoader.getResource(location);
    try {
      return new String(resource.getInputStream().readAllBytes(), StandardCharsets.UTF_8);
    } catch (IOException ex) {
      throw new UncheckedIOException("load " + location, ex);
    }
  }
}
