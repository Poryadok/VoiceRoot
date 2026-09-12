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
              rs.getString("status"),
              rs.getLong("source_revision"));
  private static final RowMapper<VerificationSourceSyncTarget> SYNC_TARGET_MAPPER =
      (rs, rowNum) ->
          new VerificationSourceSyncTarget(
              rs.getObject("account_id", UUID.class),
              rs.getObject("profile_id", UUID.class),
              rs.getString("platform"),
              rs.getLong("source_revision"),
              rs.getBoolean("verified"),
              rs.getString("badge"));

  private final NamedParameterJdbcTemplate jdbc;

  public JdbcLinkedIdentityRepository(NamedParameterJdbcTemplate jdbc) {
    this.jdbc = jdbc;
  }

  @Override
  public LinkedIdentity linkActive(
      UUID accountId,
      UUID profileId,
      String platform,
      String externalId,
      String externalLogin,
      byte[] accessTokenEncrypted,
      byte[] refreshTokenEncrypted) {
    List<LinkedIdentity> linked = jdbc.query(
        """
        WITH revision AS (
          SELECT nextval('verification_source_revision_seq') AS value
        ), linked AS (
          INSERT INTO linked_identities (
            account_id, profile_id, platform, external_id, external_login,
            access_token_encrypted, refresh_token_encrypted, status, source_revision, updated_at)
          SELECT :accountId, :profileId, :platform, :externalId, :externalLogin,
                 :accessToken, :refreshToken, 'active', value, now()
          FROM revision
          ON CONFLICT (account_id, platform) DO UPDATE SET
            profile_id = EXCLUDED.profile_id,
            external_id = EXCLUDED.external_id,
            external_login = EXCLUDED.external_login,
            access_token_encrypted = EXCLUDED.access_token_encrypted,
            refresh_token_encrypted = EXCLUDED.refresh_token_encrypted,
            status = 'active',
            source_revision = EXCLUDED.source_revision,
            updated_at = now()
          WHERE linked_identities.status = 'revoked'
             OR linked_identities.profile_id IS NULL
             OR linked_identities.profile_id = EXCLUDED.profile_id
          RETURNING id, account_id, profile_id, platform, external_id, external_login,
                    access_token_encrypted, refresh_token_encrypted, status, source_revision
        ), target AS (
          INSERT INTO verification_source_sync_targets (
            account_id, profile_id, platform, source_revision, verified, badge, updated_at)
          SELECT account_id, profile_id, platform, source_revision, true, platform, now()
          FROM linked
          ON CONFLICT (account_id, profile_id, platform) DO UPDATE SET
            source_revision = EXCLUDED.source_revision,
            verified = true,
            badge = EXCLUDED.badge,
            updated_at = now()
          WHERE verification_source_sync_targets.source_revision < EXCLUDED.source_revision
          RETURNING account_id, profile_id, platform
        )
        SELECT linked.id, linked.account_id, linked.profile_id, linked.platform,
               linked.external_id, linked.external_login,
               linked.access_token_encrypted, linked.refresh_token_encrypted,
               linked.status, linked.source_revision
        FROM linked
        LEFT JOIN target USING (account_id, profile_id, platform)
        """,
        new MapSqlParameterSource()
            .addValue("accountId", accountId)
            .addValue("profileId", profileId)
            .addValue("platform", platform)
            .addValue("externalId", externalId)
            .addValue("externalLogin", externalLogin)
            .addValue("accessToken", accessTokenEncrypted)
            .addValue("refreshToken", refreshTokenEncrypted),
        ROW_MAPPER);
    if (linked.isEmpty()) {
      throw new LinkedIdentityProfileConflictException();
    }
    return linked.getFirst();
  }

  @Override
  public List<LinkedIdentity> listActiveByAccount(UUID accountId) {
    return jdbc.query(
        """
        SELECT id, account_id, profile_id, platform, external_id, external_login,
               access_token_encrypted, refresh_token_encrypted, status,
               source_revision
        FROM linked_identities
        WHERE account_id = :accountId AND status = 'active' AND profile_id IS NOT NULL
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
               access_token_encrypted, refresh_token_encrypted, status,
               source_revision
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
               access_token_encrypted, refresh_token_encrypted, status,
               source_revision
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
  public Optional<LinkedIdentity> revokeIfUnchanged(LinkedIdentity expected) {
    return jdbc
        .query(
        """
        WITH revision AS (
          SELECT nextval('verification_source_revision_seq') AS value
        ), revoked AS (
          UPDATE linked_identities
          SET status = 'revoked', updated_at = now(),
              access_token_encrypted = NULL, refresh_token_encrypted = NULL,
              source_revision = revision.value
          FROM revision
          WHERE id = :id AND account_id = :accountId AND platform = :platform
            AND status = 'active' AND source_revision = :version
          RETURNING linked_identities.id, linked_identities.account_id,
                    linked_identities.profile_id, linked_identities.platform,
                    linked_identities.external_id, linked_identities.external_login,
                    linked_identities.access_token_encrypted,
                    linked_identities.refresh_token_encrypted,
                    linked_identities.status, linked_identities.source_revision
        ), target AS (
          INSERT INTO verification_source_sync_targets (
            account_id, profile_id, platform, source_revision, verified, badge, updated_at)
          SELECT account_id, profile_id, platform, source_revision, false, platform, now()
          FROM revoked
          ON CONFLICT (account_id, profile_id, platform) DO UPDATE SET
            source_revision = EXCLUDED.source_revision,
            verified = false,
            badge = EXCLUDED.badge,
            updated_at = now()
          WHERE verification_source_sync_targets.source_revision < EXCLUDED.source_revision
          RETURNING account_id, profile_id, platform
        )
        SELECT revoked.id, revoked.account_id, revoked.profile_id, revoked.platform,
               revoked.external_id, revoked.external_login,
               revoked.access_token_encrypted, revoked.refresh_token_encrypted,
               revoked.status, revoked.source_revision
        FROM revoked
        LEFT JOIN target USING (account_id, profile_id, platform)
        """,
        new MapSqlParameterSource()
            .addValue("id", expected.id())
            .addValue("accountId", expected.accountId())
            .addValue("platform", expected.platform())
            .addValue("version", expected.version()),
        ROW_MAPPER)
        .stream()
        .findFirst();
  }

  @Override
  public List<VerificationSourceSyncTarget> listPendingVerificationSyncTargets() {
    return jdbc.query(
        """
        SELECT account_id, profile_id, platform, source_revision, verified, badge
        FROM verification_source_sync_targets
        WHERE source_revision > synced_revision
        ORDER BY source_revision, account_id, profile_id, platform
        """,
        new MapSqlParameterSource(),
        SYNC_TARGET_MAPPER);
  }

  @Override
  public void markVerificationSyncTargetSynced(VerificationSourceSyncTarget target) {
    jdbc.update(
        """
        UPDATE verification_source_sync_targets
        SET synced_revision = :revision, updated_at = now()
        WHERE account_id = :accountId AND profile_id = :profileId AND platform = :platform
          AND source_revision = :revision AND synced_revision < :revision
        """,
        new MapSqlParameterSource()
            .addValue("accountId", target.accountId())
            .addValue("profileId", target.profileId())
            .addValue("platform", target.platform())
            .addValue("revision", target.revision()));
  }
}
