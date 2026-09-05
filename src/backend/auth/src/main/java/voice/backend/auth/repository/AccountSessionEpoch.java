package voice.backend.auth.repository;

import java.util.UUID;

public record AccountSessionEpoch(UUID accountId, long sessionEpoch) {}
