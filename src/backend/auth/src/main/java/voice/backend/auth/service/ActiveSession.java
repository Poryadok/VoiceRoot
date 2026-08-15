package voice.backend.auth.service;

import java.time.Instant;

/** Active refresh-token session for device management. */
public record ActiveSession(
    String id, String deviceInfoJson, Instant createdAt, Instant expiresAt, boolean current) {}
