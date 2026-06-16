package voice.backend.auth.oauth;

import java.time.Instant;

public class OAuthAuthorizationCodeCodec {
  private static final char SEP = '\u001f';

  public String encode(OAuthAuthorizationCode code) {
    return String.join(
        String.valueOf(SEP),
        code.code(),
        code.accountId(),
        code.profileId(),
        code.clientId(),
        code.redirectUri(),
        code.codeChallenge(),
        code.codeChallengeMethod(),
        code.expiresAt().toString());
  }

  public OAuthAuthorizationCode decode(String raw) {
    String[] parts = raw.split(String.valueOf(SEP), -1);
    if (parts.length != 8) {
      throw new IllegalArgumentException("invalid oauth code payload");
    }
    return new OAuthAuthorizationCode(
        parts[0],
        parts[1],
        parts[2],
        parts[3],
        parts[4],
        parts[5],
        parts[6],
        Instant.parse(parts[7]));
  }
}
