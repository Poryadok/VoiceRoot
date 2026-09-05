package voice.backend.auth.repository;

import java.time.Instant;
import java.util.UUID;

public record Account(
    UUID id,
    String email,
    String phone,
    String passwordHash,
    String type,
    String status,
    byte[] totpSecret,
    boolean totpEnabled,
    long sessionEpoch,
    Instant createdAt,
    Instant deletedAt) {}
