package voice.backend.auth.userdb;

import java.util.Map;
import java.util.UUID;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;

public class JdbcUserVerificationSync {
  private final NamedParameterJdbcTemplate jdbc;

  public JdbcUserVerificationSync(NamedParameterJdbcTemplate jdbc) {
    this.jdbc = jdbc;
  }

  public void setPersonalVerification(UUID profileId, String badge) {
    jdbc.update(
        """
        UPDATE profiles SET verification_type = 'personal', verification_badge = :badge, updated_at = now()
        WHERE id = :profileId
        """,
        new MapSqlParameterSource(Map.of("profileId", profileId, "badge", badge)));
  }

  public void clearVerification(UUID profileId) {
    jdbc.update(
        """
        UPDATE profiles SET verification_type = 'none', verification_badge = NULL, updated_at = now()
        WHERE id = :profileId
        """,
        new MapSqlParameterSource("profileId", profileId));
  }
}
