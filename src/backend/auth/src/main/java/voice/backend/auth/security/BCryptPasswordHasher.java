package voice.backend.auth.security;

import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;

public class BCryptPasswordHasher {
  private final BCryptPasswordEncoder encoder;

  public BCryptPasswordHasher() {
    this(new BCryptPasswordEncoder(12));
  }

  public BCryptPasswordHasher(BCryptPasswordEncoder encoder) {
    this.encoder = encoder;
  }

  public String hash(String password) {
    return encoder.encode(password);
  }

  public boolean matches(String password, String hash) {
    return encoder.matches(password, hash);
  }
}
