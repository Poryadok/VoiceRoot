package voice.backend.auth.admission;

import java.util.Map;
import java.util.UUID;
import java.util.function.Function;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

/** One non-locking Auth-owned snapshot. This never reads another service's database. */
public final class JdbcAdmissionAccountReader implements Function<UUID, AdmissionObservation.Account> {
  private final NamedParameterJdbcTemplate jdbc;
  private final SessionEpochFloorStore floors;
  public JdbcAdmissionAccountReader(NamedParameterJdbcTemplate jdbc, SessionEpochFloorStore floors) { this.jdbc=jdbc; this.floors=floors; }
  @Override public AdmissionObservation.Account apply(UUID id) {
    var account = jdbc.query("SELECT id,type,status,deleted_at,security_revision,session_epoch FROM accounts WHERE id=:id",
        Map.of("id",id), (rs,row) -> new AdmissionObservation.Account(rs.getObject("id",UUID.class),rs.getString("type"),
            "active".equals(rs.getString("status")) && rs.getTimestamp("deleted_at")==null,
            rs.getLong("security_revision"),rs.getLong("session_epoch"),1)).stream().findFirst().orElse(null);
    if (account == null || !account.active()) return account;
    long retained = floors.requireFloor(id);
    return new AdmissionObservation.Account(account.id(),account.type(),account.active(),account.revision(),account.durableEpoch(),retained);
  }
}
