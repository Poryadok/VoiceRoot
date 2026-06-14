package voice.backend.auth.userdb;

import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import voice.backend.auth.service.ProfileSwitchException;

public class JdbcProfileSwitchValidator implements ProfileSwitchValidator {
  private final NamedParameterJdbcTemplate jdbc;

  public JdbcProfileSwitchValidator(NamedParameterJdbcTemplate jdbc) {
    this.jdbc = jdbc;
  }

  public void validateOwnedSwitchable(UUID accountId, UUID profileId) {
    List<ProfileRow> rows =
        jdbc.query(
            """
            SELECT account_id, frozen_at IS NOT NULL AS frozen, deleted_at IS NOT NULL AS deleted
            FROM profiles WHERE id = :profileId
            """,
            new MapSqlParameterSource("profileId", profileId),
            (rs, rowNum) ->
                new ProfileRow(
                    rs.getObject("account_id", UUID.class),
                    rs.getBoolean("frozen"),
                    rs.getBoolean("deleted")));
    if (rows.isEmpty()) {
      throw new ProfileSwitchException("profile_not_found", ProfileSwitchException.Kind.NOT_FOUND);
    }
    ProfileRow row = rows.getFirst();
    if (!accountId.equals(row.accountId())) {
      throw new ProfileSwitchException("profile_forbidden", ProfileSwitchException.Kind.FORBIDDEN);
    }
    if (row.deleted()) {
      throw new ProfileSwitchException("profile_deleted", ProfileSwitchException.Kind.PRECONDITION);
    }
    if (row.frozen()) {
      throw new ProfileSwitchException("profile_frozen", ProfileSwitchException.Kind.PRECONDITION);
    }
  }

  private record ProfileRow(UUID accountId, boolean frozen, boolean deleted) {}
}
