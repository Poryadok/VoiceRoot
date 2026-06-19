package voice.backend.auth.userdb;

import java.util.Collection;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;

public final class JdbcPhoneHashResolver implements PhoneHashResolver {
  private final NamedParameterJdbcTemplate authJdbc;
  private final NamedParameterJdbcTemplate userJdbc;

  public JdbcPhoneHashResolver(
      NamedParameterJdbcTemplate authJdbc, NamedParameterJdbcTemplate userJdbc) {
    this.authJdbc = authJdbc;
    this.userJdbc = userJdbc;
  }

  @Override
  public Map<String, String> resolvePrimaryProfileIdsByPhoneHashes(Collection<String> phoneHashes) {
    Map<String, String> out = new HashMap<>();
    if (phoneHashes == null || phoneHashes.isEmpty()) {
      return out;
    }
    List<String> hashes =
        phoneHashes.stream()
            .filter(h -> h != null && !h.isBlank())
            .map(String::trim)
            .distinct()
            .toList();
    if (hashes.isEmpty()) {
      return out;
    }

    List<Map.Entry<String, UUID>> accountRows =
        authJdbc.query(
            """
            SELECT phone, id
            FROM accounts
            WHERE phone IN (:hashes) AND status = 'active'
            """,
            new MapSqlParameterSource("hashes", hashes),
            (rs, rowNum) -> Map.entry(rs.getString("phone"), rs.getObject("id", UUID.class)));

    if (accountRows.isEmpty()) {
      return out;
    }
    List<UUID> accountIds = accountRows.stream().map(Map.Entry::getValue).distinct().toList();
    Map<UUID, String> profileByAccount =
        userJdbc.query(
            """
            SELECT account_id, id::text AS profile_id
            FROM profiles
            WHERE account_id IN (:accountIds) AND is_primary = true
            """,
            new MapSqlParameterSource("accountIds", accountIds),
            rs -> {
              Map<UUID, String> map = new HashMap<>();
              while (rs.next()) {
                map.put(rs.getObject("account_id", UUID.class), rs.getString("profile_id"));
              }
              return map;
            });

    for (Map.Entry<String, UUID> row : accountRows) {
      String profileId = profileByAccount.get(row.getValue());
      if (profileId != null && !profileId.isBlank()) {
        out.put(row.getKey(), profileId);
      }
    }
    return out;
  }
}
