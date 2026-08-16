package voice.backend.auth.repository;

import java.util.UUID;

/** OAuth-linked external identity for personal verification (docs/features/verification.md). */
public record LinkedIdentity(
    UUID id,
    UUID accountId,
    UUID profileId,
    String platform,
    String externalId,
    String externalLogin,
    byte[] accessTokenEncrypted,
    byte[] refreshTokenEncrypted,
    String status) {}
