package voice.backend.auth.repository;

import java.util.List;
import java.util.Optional;
import java.util.UUID;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;

public class JdbcLinkedIdentityRepository implements LinkedIdentityRepository {
  private static final RowMapper<LinkedIdentity> ROW_MAPPER =
      (rs, rowNum) ->
          new LinkedIdentity(
              rs.getObject("id", UUID.class),
              rs.getObject("account_id", UUID.class),
              rs.getObject("profile_id", UUID.class),
              rs.getString("platform"),
              rs.getString("external_id"),
              rs.getString("external_login"),
              rs.getBytes("access_token_encrypted"),
              rs.getBytes("refresh_token_encrypted"),
              rs.getString("status"));

  private final NamedParameterJdbcTemplate jdbc;

  public JdbcLinkedIdentityRepository(NamedParameterJdbcTemplate jdbc) {
    this.jdbc = jdbc;
  }

  @Override
  public void upsertActive(
      UUID accountId,
      UUID profileId,
      String platform,
      String externalId,
      String externalLogin,
      byte[] accessTokenEncrypted,
      byte[] refreshTokenEncrypted) {
    jdbc.update(
        """
        INSERT INTO linked_identities (
          account_id, profile_id, platform, external_id, external_login,
          access_token_encrypted, refresh_token_encrypted, status, updated_at)
        VALUES (
          :accountId, :profileId, :platform, :externalId, :externalLogin,
          :accessToken, :refreshToken, 'active', now())
        ON CONFLICT (account_id, platform) DO UPDATE SET
          profile_id = EXCLUDED.profile_id,
          external_id = EXCLUDED.external_id,
          external_login = EXCLUDED.external_login,
          access_token_encrypted = EXCLUDED.access_token_encrypted,
          refresh_token_encrypted = EXCLUDED.refresh_token_encrypted,
          status = 'active',
          updated_at = now()
        """,
        new MapSqlParameterSource()
            .addValue("accountId", accountId)
            .addValue("profileId", profileId)
            .addValue("platform", platform)
            .addValue("externalId", externalId)
            .addValue("externalLogin", externalLogin)
            .addValue("accessToken", accessTokenEncrypted)
            .addValue("refreshToken", refreshTokenEncrypted));
  }

  @Override
  public List<LinkedIdentity> listActiveByAccount(UUID accountId) {
    return jdbc.query(
        """
        SELECT id, account_id, profile_id, platform, external_id, external_login,
               access_token_encrypted, refresh_token_encrypted, status
        FROM linked_identities
        WHERE account_id = :accountId AND status = 'active'
        ORDER BY platform
        """,
        new MapSqlParameterSource("accountId", accountId),
        ROW_MAPPER);
  }

  @Override
  public List<LinkedIdentity> listAllActive() {
    return jdbc.query(
        """
        SELECT id, account_id, profile_id, platform, external_id, external_login,
               access_token_encrypted, refresh_token_encrypted, status
        FROM linked_identities
        WHERE status = 'active'
        ORDER BY created_at
        """,
        new MapSqlParameterSource(),
        ROW_MAPPER);
  }

  @Override
  public Optional<LinkedIdentity> findActive(UUID accountId, String platform) {
    return jdbc
        .query(
            """
            SELECT id, account_id, profile_id, platform, external_id, external_login,
                   access_token_encrypted, refresh_token_encrypted, status
            FROM linked_identities
            WHERE account_id = :accountId AND platform = :platform AND status = 'active'
            LIMIT 1
            """,
            new MapSqlParameterSource()
                .addValue("accountId", accountId)
                .addValue("platform", platform),
            ROW_MAPPER)
        .stream()
        .findFirst();
  }

  @Override
  public void revoke(UUID accountId, String platform) {
    jdbc.update(
        """
        UPDATE linked_identities
        SET status = 'revoked', updated_at = now(),
            access_token_encrypted = NULL, refresh_token_encrypted = NULL
        WHERE account_id = :accountId AND platform = :platform
        """,
        new MapSqlParameterSource()
            .addValue("accountId", accountId)
            .addValue("platform", platform));
  }
}
