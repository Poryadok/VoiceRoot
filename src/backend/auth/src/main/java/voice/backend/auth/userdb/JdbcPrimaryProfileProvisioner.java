package voice.backend.auth.userdb;

import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ThreadLocalRandom;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.annotation.Transactional;
import voice.backend.auth.service.AuthException;

public class JdbcPrimaryProfileProvisioner implements PrimaryProfileProvisioner {
  private static final int MAX_DISCRIMINATOR_ATTEMPTS = 24;

  private final NamedParameterJdbcTemplate jdbc;

  public JdbcPrimaryProfileProvisioner(NamedParameterJdbcTemplate jdbc) {
    this.jdbc = jdbc;
  }

  @Override
  @Transactional
  public String ensurePrimaryProfile(UUID accountId, String displayHint) {
    List<String> existing =
        jdbc.query(
            "SELECT id::text FROM profiles WHERE account_id = :accountId AND is_primary = true LIMIT 1",
            new MapSqlParameterSource("accountId", accountId),
            (rs, rowNum) -> rs.getString(1));
    if (!existing.isEmpty()) {
      return existing.getFirst();
    }

    UUID profileId = UUID.randomUUID();
    String baseUsername = sanitizeUsername(displayHint);
    String displayName = truncate(displayHint == null || displayHint.isBlank() ? "User" : displayHint.trim(), 32);

    for (int attempt = 0; attempt < MAX_DISCRIMINATOR_ATTEMPTS; attempt++) {
      String discriminator = String.format("%04d", ThreadLocalRandom.current().nextInt(0, 10000));
      try {
        jdbc.update(
            """
            INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
            VALUES (:id, :accountId, :username, :discriminator, :displayName, true)
            """,
            new MapSqlParameterSource(Map.of(
                "id", profileId,
                "accountId", accountId,
                "username", baseUsername,
                "discriminator", discriminator,
                "displayName", displayName)));
        jdbc.update(
            """
            INSERT INTO onboarding_state (profile_id, completed_steps, completed)
            VALUES (:profileId, '[]'::jsonb, false)
            """,
            new MapSqlParameterSource("profileId", profileId));
        return profileId.toString();
      } catch (DataIntegrityViolationException ex) {
        List<String> primaryRow =
            jdbc.query(
                "SELECT id::text FROM profiles WHERE account_id = :accountId AND is_primary = true LIMIT 1",
                new MapSqlParameterSource("accountId", accountId),
                (rs, rowNum) -> rs.getString(1));
        if (!primaryRow.isEmpty()) {
          return primaryRow.getFirst();
        }
        // likely (username, discriminator) collision — retry
      }
    }
    throw new AuthException("auth_unavailable");
  }

  private static String sanitizeUsername(String displayHint) {
    String raw =
        displayHint == null
            ? ""
            : displayHint.contains("@")
                ? displayHint.substring(0, displayHint.indexOf('@'))
                : displayHint;
    StringBuilder sb = new StringBuilder();
    for (int i = 0; i < raw.length() && sb.length() < 32; i++) {
      char c = Character.toLowerCase(raw.charAt(i));
      if ((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
        sb.append(c);
      }
    }
    if (sb.isEmpty()) {
      sb.append("user");
    }
    if (sb.length() > 32) {
      return sb.substring(0, 32);
    }
    return sb.toString();
  }

  private static String truncate(String s, int max) {
    return s.length() <= max ? s : s.substring(0, max);
  }
}
