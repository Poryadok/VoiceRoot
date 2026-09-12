package voice.backend.auth.repository;

import java.util.UUID;

/** Durable Auth-owned source state waiting to converge into User Service. */
public record VerificationSourceSyncTarget(
    UUID accountId,
    UUID profileId,
    String platform,
    long revision,
    boolean verified,
    String badge) {}
