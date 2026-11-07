package voice.backend.auth.sdkidentity;

import java.util.Map;
import java.util.UUID;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;

/** Reads the shared opaque fence in the caller's transaction, including after lock waits. */
final class SdkTerminalFences {
  private SdkTerminalFences() {}

  static void requireLive(NamedParameterJdbcTemplate jdbc, String family, UUID operation) {
    if (operation != null && Boolean.TRUE.equals(jdbc.queryForObject("""
        SELECT EXISTS(SELECT 1 FROM auth_terminal_operation_fences
          WHERE family=:family AND operation_id=:operation)
        """, Map.of("family", family, "operation", operation), Boolean.class))) {
      throw new SdkOperationErasedException();
    }
  }
}
