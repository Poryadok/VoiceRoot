package voice.backend.auth.sdkidentity;

import java.util.Map;

/** Auth-owned signing boundary for short-lived SDK device authority assertions. */
@FunctionalInterface
public interface SdkDeviceStatusIssuer {
  String issueDeviceStatus(Map<String, Object> claims);
}
