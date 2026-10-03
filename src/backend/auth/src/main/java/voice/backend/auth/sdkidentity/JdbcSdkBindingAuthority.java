package voice.backend.auth.sdkidentity;

import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;

/** Resolves one current Auth grant and confirms its binding against GIS at assertion issuance. */
public final class JdbcSdkBindingAuthority implements SdkBindingAuthority {
  private final NamedParameterJdbcTemplate jdbc;
  private final SdkGameIntegrationPolicyClient gis;

  public JdbcSdkBindingAuthority(NamedParameterJdbcTemplate jdbc, SdkGameIntegrationPolicyClient gis) {
    this.jdbc = jdbc;
    this.gis = gis;
  }

  @Override
  public Optional<Binding> currentBinding(UUID applicationId, UUID environmentId, UUID accountId, UUID deviceId) {
    if (jdbc == null || gis == null || applicationId == null || environmentId == null || accountId == null || deviceId == null) {
      return Optional.empty();
    }
    try {
      List<Map<String, Object>> rows = jdbc.queryForList("""
          SELECT g.binding_id, i.actor_id, a.device_id
          FROM sdk_game_message_grants g
          JOIN sdk_authorizations a ON a.request_id=g.authorization_request_id
          JOIN sdk_linked_sessions s ON s.request_id=a.request_id
          JOIN sdk_identities i ON i.account_id=a.source_account_id
            AND i.application_id=a.application_id AND i.environment_id=a.environment_id
          JOIN sdk_devices d ON d.device_id=a.device_id AND d.account_id=a.source_account_id
          WHERE g.application_id=:application AND g.environment_id=:environment
            AND a.source_account_id=:account AND a.device_id=:device
            AND a.source_generation=i.ownership_generation
            AND a.game_binding_status='active' AND g.status='active'
            AND g.binding_id=a.game_binding_id
            AND g.authority_revision=a.game_binding_authority_revision
            AND a.game_binding_consent_revision=s.consent_revision
            AND g.consent_revision=s.consent_revision
            AND i.status='active' AND d.revoked_at IS NULL
          FOR UPDATE OF a,g
          """, Map.of("application", applicationId, "environment", environmentId,
              "account", accountId, "device", deviceId));
      if (rows.size() != 1) return Optional.empty();
      Map<String, Object> row = rows.getFirst();
      UUID bindingId = (UUID) row.get("binding_id");
      UUID actorId = (UUID) row.get("actor_id");
      UUID persistedDeviceId = (UUID) row.get("device_id");
      if (bindingId == null || actorId == null || actorId.equals(new UUID(0, 0)) || !deviceId.equals(persistedDeviceId)) {
        return Optional.empty();
      }
      SdkGameIntegrationPolicyClient.GisBindingAuthority authority =
          gis.resolveBindingAuthority(bindingId, applicationId, environmentId);
      if (!applicationId.equals(authority.applicationId()) || !environmentId.equals(authority.environmentId())
          || !bindingId.equals(authority.bindingId()) || !"active".equals(authority.status())
          || authority.bindingRevision() <= 0) return Optional.empty();
      return Optional.of(new Binding(actorId, bindingId));
    } catch (RuntimeException unavailableOrInvalid) {
      return Optional.empty();
    }
  }
}
