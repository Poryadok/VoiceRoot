-- Explicit rollback destroys SDK identity fences; keep capability disabled afterward.
DROP TABLE sdk_sessions;
DROP TABLE sdk_challenges;
DROP TABLE sdk_devices;
DROP TABLE sdk_identities;
