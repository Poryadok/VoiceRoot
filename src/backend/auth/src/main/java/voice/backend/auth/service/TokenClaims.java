package voice.backend.auth.service;

import java.time.Instant;
import java.util.List;

public record TokenClaims(
    String userId,
    String profileId,
    List<String> roles,
    String subscriptionTier,
    Instant expiresAt,
    String jti) {}
