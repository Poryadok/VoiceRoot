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
        code.expiresAt().toString(),
        Long.toString(code.originSessionEpoch()));
  }

  public OAuthAuthorizationCode decode(String raw) {
    String[] parts = raw.split(String.valueOf(SEP), -1);
    if (parts.length != 8 && parts.length != 9) {
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
        Instant.parse(parts[7]),
        parts.length == 9 ? parseEpoch(parts[8]) : 0);
  }

  private static long parseEpoch(String value) {
    try {
      long epoch = Long.parseLong(value);
      return epoch > 0 ? epoch : 0;
    } catch (NumberFormatException invalid) {
      return 0;
    }
  }
}
