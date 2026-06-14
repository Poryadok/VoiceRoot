package voice.backend.auth.repository;

public record E2EKeyBackupRecord(String encryptedBlob, String passwordHint) {}
